package mongostore

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type entries struct{ db *mongo.Database }

func (s *Store) Entries() store.Entries { return &entries{db: s.db} }

// fullEntryDoc reads the entry columns entryDoc omits (client_id/uid/has_issued), needed by
// the /entry domain where those are part of the wire shape.
type fullEntryDoc struct {
	entryDoc  `bson:",inline"`
	ClientID  *primitive.ObjectID `bson:"client_id"`
	CompanyID *primitive.ObjectID `bson:"company_id"`
	UID       *primitive.ObjectID `bson:"uid"`
	HasIssued bool                `bson:"has_issued"`
}

func (e fullEntryDoc) toStore() store.Entry {
	out := e.entryDoc.toStore()
	out.HasIssued = e.HasIssued
	if e.ClientID != nil {
		out.ClientID = idOf(*e.ClientID)
	}
	if e.UID != nil {
		out.UID = idOf(*e.UID)
	}
	if e.CompanyID != nil {
		out.CompanyID = idOf(*e.CompanyID)
	}
	return out
}

func (en *entries) materialHSN(ctx context.Context, companyOID primitive.ObjectID, name string) string {
	var doc struct {
		Hsn string `bson:"hsn"`
	}
	_ = en.db.Collection(colMaterials).FindOne(ctx,
		bson.M{"material_name": name, "company_id": companyOID}).Decode(&doc)
	return doc.Hsn
}

// linkSheet pushes entryOID onto the day-sheet for date, creating the sheet if none matches -
// the same linkage jobs.linkEntryToSheet performs for conversions.
func (en *entries) linkSheet(ctx context.Context, companyOID, uidOID primitive.ObjectID, date string, entryOID primitive.ObjectID) error {
	if sid, ok, err := en.sheetForDate(ctx, companyOID, date); err != nil {
		return err
	} else if ok {
		_, err := en.db.Collection(colSheets).UpdateByID(ctx, sid, bson.M{"$push": bson.M{"entries": entryOID}})
		return err
	}
	_, err := en.db.Collection(colSheets).InsertOne(ctx, bson.M{
		"date": date, "uid": uidOID, "company_id": companyOID, "entries": bson.A{entryOID},
		"createdAt": time.Now().UTC(), "__v": 0,
	})
	return err
}

func (en *entries) sheetForDate(ctx context.Context, companyOID primitive.ObjectID, date string) (primitive.ObjectID, bool, error) {
	target := store.NormalizeDate(date)
	cur, err := en.db.Collection(colSheets).Find(ctx, bson.M{"company_id": companyOID})
	if err != nil {
		return primitive.NilObjectID, false, err
	}
	var sheets []struct {
		ID   primitive.ObjectID `bson:"_id"`
		Date string             `bson:"date"`
	}
	if err := cur.All(ctx, &sheets); err != nil {
		return primitive.NilObjectID, false, err
	}
	for _, s := range sheets {
		if store.NormalizeDate(s.Date) == target {
			return s.ID, true, nil
		}
	}
	return primitive.NilObjectID, false, nil
}

func (en *entries) load(ctx context.Context, id primitive.ObjectID) (store.Entry, error) {
	var doc fullEntryDoc
	if err := en.db.Collection(colEntries).FindOne(ctx, bson.M{"_id": id}).Decode(&doc); err != nil {
		return store.Entry{}, err
	}
	return doc.toStore(), nil
}

func (en *entries) Add(ctx context.Context, uid, companyID store.ID, in store.EntryWrite) (store.Entry, error) {
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.Entry{}, err
	}
	uidOID, err := objectID(uid)
	if err != nil {
		return store.Entry{}, err
	}
	clientOID, err := objectID(in.ClientID)
	if err != nil {
		return store.Entry{}, err
	}
	hsn := en.materialHSN(ctx, companyOID, in.Material)
	now := time.Now().UTC()
	doc := bson.M{
		"client_id": clientOID, "uid": uidOID, "company_id": companyOID,
		"material": in.Material, "hsn": hsn, "description": in.Description,
		"length": in.Length, "width": in.Width, "date": in.Date, "qty": in.Qty, "rate": in.Rate,
		"cgst": in.Cgst, "sgst": in.Sgst, "igst": in.Igst, "total": in.Total, "amount": in.Amount,
		"advance": in.Advance, "discount": in.Discount, "charges": in.Charges,
		"has_issued": false, "createdAt": now, "updatedAt": now, "__v": 0,
	}
	res, err := en.db.Collection(colEntries).InsertOne(ctx, doc)
	if err != nil {
		return store.Entry{}, err
	}
	entryOID := res.InsertedID.(primitive.ObjectID)
	if err := en.linkSheet(ctx, companyOID, uidOID, in.Date, entryOID); err != nil {
		return store.Entry{}, err
	}
	if _, err := en.db.Collection(colClients).UpdateByID(ctx, clientOID,
		bson.M{"$push": bson.M{"entries": entryOID}}); err != nil {
		return store.Entry{}, err
	}
	e, err := en.load(ctx, entryOID)
	if err != nil {
		return store.Entry{}, err
	}
	e.Client = en.loadClient(ctx, clientOID)
	return e, nil
}

func (en *entries) loadClient(ctx context.Context, id primitive.ObjectID) *store.EntryClient {
	var doc struct {
		ID            primitive.ObjectID  `bson:"_id"`
		CompanyID     *primitive.ObjectID `bson:"company_id"`
		UID           *primitive.ObjectID `bson:"uid"`
		LegacyID      *int64              `bson:"client_id"`
		ClientName    string              `bson:"clientName"`
		ClientFirm    string              `bson:"clientFirm"`
		ClientPhone   string              `bson:"clientPhone"`
		ClientGST     string              `bson:"clientGST"`
		ClientAddress string              `bson:"clientAddress"`
		CreatedAt     time.Time           `bson:"createdAt"`
		UpdatedAt     time.Time           `bson:"updatedAt"`
	}
	if err := en.db.Collection(colClients).FindOne(ctx, bson.M{"_id": id}).Decode(&doc); err != nil {
		return nil
	}
	c := &store.EntryClient{
		ID: idOf(doc.ID), LegacyID: doc.LegacyID, ClientName: doc.ClientName, ClientFirm: doc.ClientFirm,
		ClientPhone: doc.ClientPhone, ClientGST: doc.ClientGST, ClientAddress: doc.ClientAddress,
		CreatedAt: doc.CreatedAt, UpdatedAt: doc.UpdatedAt,
	}
	if doc.CompanyID != nil {
		c.CompanyID = idOf(*doc.CompanyID)
	}
	if doc.UID != nil {
		c.UID = idOf(*doc.UID)
	}
	return c
}

func (en *entries) Update(ctx context.Context, uid, companyID store.ID, in store.EntryUpdate) (store.Entry, bool, error) {
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.Entry{}, false, err
	}
	uidOID, err := objectID(uid)
	if err != nil {
		return store.Entry{}, false, err
	}
	entryOID, err := objectID(in.EntryID)
	if err != nil {
		return store.Entry{}, false, err
	}
	clientOID, err := objectID(in.ClientID)
	if err != nil {
		return store.Entry{}, false, err
	}

	var existing struct {
		Date string `bson:"date"`
	}
	err = en.db.Collection(colEntries).FindOne(ctx,
		bson.M{"_id": entryOID, "company_id": companyOID}).Decode(&existing)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.Entry{}, false, nil
	}
	if err != nil {
		return store.Entry{}, false, err
	}

	// Move between day-sheets on a date change: pull from the old day's sheet, push to (or
	// create) the new day's sheet. The old sheet document is left in place, as Node leaves it.
	if store.NormalizeDate(existing.Date) != store.NormalizeDate(in.Date) {
		if oldSID, ok, err := en.sheetForDate(ctx, companyOID, existing.Date); err != nil {
			return store.Entry{}, false, err
		} else if ok {
			if _, err := en.db.Collection(colSheets).UpdateByID(ctx, oldSID,
				bson.M{"$pull": bson.M{"entries": entryOID}}); err != nil {
				return store.Entry{}, false, err
			}
		}
		if err := en.linkSheet(ctx, companyOID, uidOID, in.Date, entryOID); err != nil {
			return store.Entry{}, false, err
		}
	}

	hsn := en.materialHSN(ctx, companyOID, in.Material)
	_, err = en.db.Collection(colEntries).UpdateOne(ctx,
		bson.M{"_id": entryOID, "company_id": companyOID},
		bson.M{"$set": bson.M{
			"client_id": clientOID, "material": in.Material, "hsn": hsn, "description": in.Description,
			"length": in.Length, "width": in.Width, "date": in.Date, "qty": in.Qty, "rate": in.Rate,
			"cgst": in.Cgst, "sgst": in.Sgst, "igst": in.Igst, "total": in.Total, "amount": in.Amount,
			"advance": in.Advance, "discount": in.Discount, "charges": in.Charges, "updatedAt": time.Now().UTC(),
		}})
	if err != nil {
		return store.Entry{}, false, err
	}
	e, err := en.load(ctx, entryOID)
	if err != nil {
		return store.Entry{}, false, err
	}
	return e, true, nil
}

func (en *entries) Get(ctx context.Context, uid, companyID, entryID store.ID) (store.Entry, bool, error) {
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.Entry{}, false, err
	}
	entryOID, err := objectID(entryID)
	if err != nil {
		return store.Entry{}, false, err
	}
	var doc fullEntryDoc
	err = en.db.Collection(colEntries).FindOne(ctx,
		bson.M{"_id": entryOID, "company_id": companyOID}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.Entry{}, false, nil
	}
	if err != nil {
		return store.Entry{}, false, err
	}
	return doc.toStore(), true, nil
}

func (en *entries) List(ctx context.Context, uid, companyID store.ID) ([]store.Entry, error) {
	companyOID, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	cur, err := en.db.Collection(colEntries).Find(ctx, bson.M{"company_id": companyOID})
	if err != nil {
		return nil, err
	}
	var docs []fullEntryDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]store.Entry, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.toStore())
	}
	return out, nil
}

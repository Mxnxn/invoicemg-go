package mongostore

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/mxnxn/invoicemg-go/internal/entrymath"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

func (i *invoices) Numbers(ctx context.Context, companyID store.ID) ([]string, error) {
	companyOID, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	cur, err := i.db.Collection(colInvoices).Find(ctx, bson.M{"company_id": companyOID},
		options.Find().SetProjection(bson.M{"invoiceId": 1}))
	if err != nil {
		return nil, fmt.Errorf("invoice numbers: %w", err)
	}
	var docs []struct {
		N string `bson:"invoiceId"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.N)
	}
	return out, nil
}

func (i *invoices) EntryJobLabels(ctx context.Context, companyID store.ID, entryIDs []store.ID) (map[string]string, error) {
	out := map[string]string{}
	if len(entryIDs) == 0 {
		return out, nil
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	oids := make([]primitive.ObjectID, 0, len(entryIDs))
	want := map[primitive.ObjectID]bool{}
	for _, id := range entryIDs {
		if oid, err := objectID(id); err == nil {
			oids = append(oids, oid)
			want[oid] = true
		}
	}
	cur, err := i.db.Collection(colJobs).Find(ctx, bson.M{"company_id": companyOID, "rows.entry_id": bson.M{"$in": oids}},
		options.Find().SetProjection(bson.M{"challanNumber": 1, "rows.entry_id": 1}))
	if err != nil {
		return nil, fmt.Errorf("entry job labels: %w", err)
	}
	var jobs []struct {
		Challan string `bson:"challanNumber"`
		Rows    []struct {
			EntryID *primitive.ObjectID `bson:"entry_id"`
		} `bson:"rows"`
	}
	if err := cur.All(ctx, &jobs); err != nil {
		return nil, err
	}
	for _, j := range jobs {
		for _, r := range j.Rows {
			if r.EntryID != nil && want[*r.EntryID] {
				out[r.EntryID.Hex()] = j.Challan
			}
		}
	}
	return out, nil
}

func (i *invoices) Received(ctx context.Context, companyID, invoiceID store.ID) ([]store.InvoiceReceivedRow, error) {
	companyOID, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	invOID, err := objectID(invoiceID)
	if err != nil {
		return nil, store.ErrBadID
	}
	cur, err := i.db.Collection(colInvoiceReceived).Find(ctx, bson.M{"company_id": companyOID, "invoice_id": invOID},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("invoice received: %w", err)
	}
	var docs []struct {
		ID        primitive.ObjectID  `bson:"_id"`
		Date      string              `bson:"date"`
		Amount    float64             `bson:"amount"`
		Note      string              `bson:"note"`
		InvoiceID *primitive.ObjectID `bson:"invoice_id"`
		BankID    *primitive.ObjectID `bson:"bank_id"`
		CreatedAt time.Time           `bson:"createdAt"`
		Version   int                 `bson:"__v"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	bankIDs := map[primitive.ObjectID]struct{}{}
	for _, d := range docs {
		if d.BankID != nil {
			bankIDs[*d.BankID] = struct{}{}
		}
	}
	banks, err := i.bankNames(ctx, bankIDs)
	if err != nil {
		return nil, err
	}
	out := make([]store.InvoiceReceivedRow, 0, len(docs))
	for _, d := range docs {
		r := store.InvoiceReceivedRow{ID: idOf(d.ID), Date: d.Date, Amount: d.Amount, Note: d.Note, CreatedAt: d.CreatedAt, Version: d.Version}
		if d.InvoiceID != nil {
			r.InvoiceID = idOf(*d.InvoiceID)
		}
		if d.BankID != nil {
			r.BankID, r.BankName = idOf(*d.BankID), banks[*d.BankID]
		}
		out = append(out, r)
	}
	return out, nil
}

func (i *invoices) bankNames(ctx context.Context, ids map[primitive.ObjectID]struct{}) (map[primitive.ObjectID]string, error) {
	out := map[primitive.ObjectID]string{}
	if len(ids) == 0 {
		return out, nil
	}
	list := make([]primitive.ObjectID, 0, len(ids))
	for id := range ids {
		list = append(list, id)
	}
	cur, err := i.db.Collection(colBanks).Find(ctx, bson.M{"_id": bson.M{"$in": list}}, options.Find().SetProjection(bson.M{"name": 1}))
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ID   primitive.ObjectID `bson:"_id"`
		Name string             `bson:"name"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ID] = r.Name
	}
	return out, nil
}

func (i *invoices) Remove(ctx context.Context, companyID, invoiceID store.ID) (bool, error) {
	companyOID, err := objectID(companyID)
	if err != nil {
		return false, err
	}
	invOID, err := objectID(invoiceID)
	if err != nil {
		return false, nil
	}
	var doc struct {
		Entries []primitive.ObjectID `bson:"entries"`
	}
	err = i.db.Collection(colInvoices).FindOne(ctx, bson.M{"_id": invOID, "company_id": companyOID}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("looking up invoice: %w", err)
	}
	if len(doc.Entries) > 0 {
		if _, err := i.db.Collection(colEntries).UpdateMany(ctx, bson.M{"_id": bson.M{"$in": doc.Entries}},
			bson.M{"$set": bson.M{"issued": nil, "has_issued": false}}); err != nil {
			return false, fmt.Errorf("un-issue entries: %w", err)
		}
	}
	if _, err := i.db.Collection(colInvoices).DeleteOne(ctx, bson.M{"_id": invOID, "company_id": companyOID}); err != nil {
		return false, fmt.Errorf("delete invoice: %w", err)
	}
	return true, nil
}

func (i *invoices) Save(ctx context.Context, uid, companyID store.ID, in store.InvoiceSaveInput) (store.ID, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return "", err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return "", err
	}
	clientOID, err := objectID(in.ClientID)
	if err != nil {
		return "", store.ErrBadID
	}
	entryOIDs := make([]primitive.ObjectID, 0, len(in.EntryIDs))
	for _, id := range in.EntryIDs {
		if oid, err := objectID(id); err == nil {
			entryOIDs = append(entryOIDs, oid)
		}
	}

	var existing struct {
		ID      primitive.ObjectID   `bson:"_id"`
		Entries []primitive.ObjectID `bson:"entries"`
	}
	err = i.db.Collection(colInvoices).FindOne(ctx, bson.M{"invoiceId": in.InvNo, "company_id": companyOID}).Decode(&existing)
	reissue := err == nil
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return "", fmt.Errorf("looking up invoice number: %w", err)
	}

	var invoiceOID primitive.ObjectID
	if reissue {
		invoiceOID = existing.ID
		if len(existing.Entries) > 0 {
			if _, err := i.db.Collection(colEntries).UpdateMany(ctx, bson.M{"_id": bson.M{"$in": existing.Entries}},
				bson.M{"$set": bson.M{"issued": nil, "has_issued": false}}); err != nil {
				return "", fmt.Errorf("release entries: %w", err)
			}
		}
	} else {
		invoiceOID = primitive.NewObjectID()
		now := time.Now().UTC()
		if _, err := i.db.Collection(colInvoices).InsertOne(ctx, bson.M{
			"_id": invoiceOID, "uid": uidOID, "company_id": companyOID, "client": clientOID,
			"invoiceId": in.InvNo, "date": in.Date, "entries": bson.A{}, "amount": 0, "totalAmount": 0,
			"createdAt": now, "updatedAt": now, "__v": 0,
		}); err != nil {
			return "", fmt.Errorf("insert invoice: %w", err)
		}
	}

	var paid, total float64
	for _, entryOID := range entryOIDs {
		var e struct {
			Amount  float64 `bson:"amount"`
			Advance float64 `bson:"advance"`
		}
		err := i.db.Collection(colEntries).FindOneAndUpdate(ctx, bson.M{"_id": entryOID},
			bson.M{"$set": bson.M{"issued": invoiceOID, "has_issued": true}},
			options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&e)
		if errors.Is(err, mongo.ErrNoDocuments) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("issue entry: %w", err)
		}
		paid = entrymath.RoundOffWithAmount(paid + e.Advance)
		total = entrymath.RoundOffWithAmount(total + entrymath.RoundOffWithAmount(e.Amount*1.18))
	}

	entriesArr := bson.A{}
	for _, oid := range entryOIDs {
		entriesArr = append(entriesArr, oid)
	}
	if _, err := i.db.Collection(colInvoices).UpdateByID(ctx, invoiceOID, bson.M{"$set": bson.M{
		"client": clientOID, "date": in.Date, "entries": entriesArr, "amount": paid, "totalAmount": total, "updatedAt": time.Now().UTC(),
	}}); err != nil {
		return "", fmt.Errorf("update invoice totals: %w", err)
	}
	return idOf(invoiceOID), nil
}

func (i *invoices) Paid(ctx context.Context, companyID store.ID, in store.InvoicePaidInput) (bool, error) {
	companyOID, err := objectID(companyID)
	if err != nil {
		return false, err
	}
	invOID, err := objectID(in.InvoiceID)
	if err != nil {
		return false, nil
	}
	var inv struct {
		Amount      float64              `bson:"amount"`
		TotalAmount float64              `bson:"totalAmount"`
		UID         primitive.ObjectID   `bson:"uid"`
		Client      *primitive.ObjectID  `bson:"client"`
		Entries     []primitive.ObjectID `bson:"entries"`
	}
	err = i.db.Collection(colInvoices).FindOne(ctx, bson.M{"_id": invOID, "company_id": companyOID}).Decode(&inv)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("looking up invoice: %w", err)
	}

	type ent struct {
		id                                 primitive.ObjectID
		amount, advance, total, cgst, sgst float64
	}
	loadEntry := func(id primitive.ObjectID) (ent, bool, error) {
		var raw struct {
			Amount  float64 `bson:"amount"`
			Advance float64 `bson:"advance"`
			Total   float64 `bson:"total"`
			Cgst    float64 `bson:"cgst"`
			Sgst    float64 `bson:"sgst"`
		}
		err := i.db.Collection(colEntries).FindOne(ctx, bson.M{"_id": id}).Decode(&raw)
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ent{}, false, nil
		}
		if err != nil {
			return ent{}, false, err
		}
		return ent{id: id, amount: raw.Amount, advance: raw.Advance, total: raw.Total, cgst: raw.Cgst, sgst: raw.Sgst}, true, nil
	}
	setEntry := func(id primitive.ObjectID, advance, total float64) error {
		_, err := i.db.Collection(colEntries).UpdateByID(ctx, id, bson.M{"$set": bson.M{"advance": advance, "total": total}})
		return err
	}
	bumpJob := func(entryID primitive.ObjectID, applied float64) error {
		if !(applied > 0) {
			return nil
		}
		var job struct {
			ID      primitive.ObjectID `bson:"_id"`
			Total   float64            `bson:"total"`
			Advance float64            `bson:"advance"`
		}
		err := i.db.Collection(colJobs).FindOne(ctx, bson.M{"rows.entry_id": entryID, "company_id": companyOID}).Decode(&job)
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil
		}
		if err != nil {
			return err
		}
		next := brRound2(math.Min(job.Total, job.Advance+applied))
		_, err = i.db.Collection(colJobs).UpdateByID(ctx, job.ID, bson.M{"$set": bson.M{"advance": next}})
		return err
	}

	received := in.ReceivedAmount

	if in.Mode == "manual" {
		want := map[primitive.ObjectID]bool{}
		for _, e := range inv.Entries {
			want[e] = true
		}
		var appliedTotal float64
		for _, a := range in.Allocations {
			eid, err := objectID(a.EntryID)
			if err != nil || !want[eid] {
				continue
			}
			e, ok, err := loadEntry(eid)
			if err != nil {
				return false, err
			}
			if !ok {
				continue
			}
			applied := math.Max(0, math.Min(e.total, a.Amount))
			if applied <= 0 {
				continue
			}
			if err := setEntry(e.id, brRound2(e.advance+applied), brRound2(e.total-applied)); err != nil {
				return false, err
			}
			if err := bumpJob(e.id, applied); err != nil {
				return false, err
			}
			appliedTotal += applied
		}
		if _, err := i.db.Collection(colInvoices).UpdateByID(ctx, invOID, bson.M{"$set": bson.M{"amount": brRound2(inv.Amount + appliedTotal)}}); err != nil {
			return false, err
		}
	} else if received+inv.Amount >= inv.TotalAmount {
		for _, eid := range inv.Entries {
			e, ok, err := loadEntry(eid)
			if err != nil {
				return false, err
			}
			if !ok {
				continue
			}
			taxed := e.amount * (1 + e.cgst/100 + e.sgst/100)
			applied := math.Max(0, taxed-e.advance)
			if err := setEntry(e.id, taxed, 0); err != nil {
				return false, err
			}
			if err := bumpJob(e.id, applied); err != nil {
				return false, err
			}
		}
		if _, err := i.db.Collection(colInvoices).UpdateByID(ctx, invOID, bson.M{"$set": bson.M{"amount": inv.TotalAmount}}); err != nil {
			return false, err
		}
	} else {
		rem := received
		for _, eid := range inv.Entries {
			if rem <= 0 {
				break
			}
			e, ok, err := loadEntry(eid)
			if err != nil {
				return false, err
			}
			if !ok {
				continue
			}
			if rem > e.total {
				applied := e.total
				if err := setEntry(e.id, e.advance+applied, 0); err != nil {
					return false, err
				}
				if err := bumpJob(e.id, applied); err != nil {
					return false, err
				}
				rem -= e.total
			} else {
				applied := rem
				if err := setEntry(e.id, e.advance+applied, e.total-applied); err != nil {
					return false, err
				}
				if err := bumpJob(e.id, applied); err != nil {
					return false, err
				}
				rem = 0
			}
		}
		if _, err := i.db.Collection(colInvoices).UpdateByID(ctx, invOID, bson.M{"$set": bson.M{"amount": inv.Amount + received}}); err != nil {
			return false, err
		}
	}

	doc := bson.M{
		"client": inv.Client, "date": in.Date, "uid": inv.UID, "company_id": companyOID,
		"amount": in.ReceivedAmount, "invoice_id": invOID, "note": in.Note, "createdAt": time.Now().UTC(), "__v": 0,
	}
	if in.BankID != "" {
		if bankOID, err := objectID(in.BankID); err == nil {
			doc["bank_id"] = bankOID
		}
	}
	if _, err := i.db.Collection(colInvoiceReceived).InsertOne(ctx, doc); err != nil {
		return false, fmt.Errorf("log invoice received: %w", err)
	}
	return true, nil
}

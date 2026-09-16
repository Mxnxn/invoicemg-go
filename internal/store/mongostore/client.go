package mongostore

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type clients struct{ db *mongo.Database }

func (s *Store) Clients() store.Clients { return &clients{db: s.db} }

// clientDoc is the subset of Model/Client.js the customer list reads. Sharing is a pointer so an
// absent field decodes to nil (Node omits the key) rather than to an empty struct.
type clientDoc struct {
	ID             primitive.ObjectID  `bson:"_id"`
	UID            primitive.ObjectID  `bson:"uid"`
	CompanyID      *primitive.ObjectID `bson:"company_id"`
	ClientName     string              `bson:"clientName"`
	ClientFirm     string              `bson:"clientFirm"`
	ClientPhone    string              `bson:"clientPhone"`
	ClientGST      string              `bson:"clientGST"`
	ClientAddress  string              `bson:"clientAddress"`
	OpeningBalance float64             `bson:"openingBalance"`
	Sharing        *struct {
		Companies []primitive.ObjectID `bson:"companies"`
	} `bson:"sharing"`
}

func (d clientDoc) toStore() store.Client {
	c := store.Client{
		ID:             idOf(d.ID),
		UID:            idOf(d.UID),
		ClientName:     d.ClientName,
		ClientFirm:     d.ClientFirm,
		ClientPhone:    d.ClientPhone,
		ClientGST:      d.ClientGST,
		ClientAddress:  d.ClientAddress,
		OpeningBalance: d.OpeningBalance,
	}
	if d.CompanyID != nil {
		c.CompanyID = idOf(*d.CompanyID)
	}
	if d.Sharing != nil {
		// present, even if empty - make(0) so a stamped-but-unshared record sends {companies:[]}.
		ids := make([]store.ID, 0, len(d.Sharing.Companies))
		for _, id := range d.Sharing.Companies {
			ids = append(ids, idOf(id))
		}
		c.Sharing = &ids
	}
	return c
}

func (c *clients) Visible(ctx context.Context, uid, companyID store.ID) ([]store.Client, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return nil, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	// visibleScope, bounded by uid: own company, legacy null-company rows, or shared with this
	// company (Helpers/SharedRecords.js). uid keeps it inside one admin, so sharing can never
	// cross accounts. No sort, matching routes/Client.js (natural order); the sqlstore adds the
	// id tiebreak (#19).
	filter := bson.M{
		"uid": uidOID,
		"$or": bson.A{
			bson.M{"company_id": companyOID},
			bson.M{"company_id": nil},
			bson.M{"sharing.companies": companyOID},
		},
	}
	cur, err := c.db.Collection(colClients).Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("listing clients: %w", err)
	}
	defer cur.Close(ctx)

	var docs []clientDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("reading clients: %w", err)
	}
	out := make([]store.Client, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.toStore())
	}
	return out, nil
}

func (c *clients) Create(ctx context.Context, companyID, uid store.ID, legacyID int64, in store.ClientWrite) (store.Client, store.Dup, error) {
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.Client{}, store.DupNone, err
	}
	uidOID, err := objectID(uid)
	if err != nil {
		return store.Client{}, store.DupNone, err
	}
	if dup, err := c.dupCheck(ctx, companyOID, nil, in.ClientGST, in.ClientPhone); err != nil || dup != store.DupNone {
		return store.Client{}, dup, err
	}
	doc := bson.M{
		"uid": uidOID, "company_id": companyOID, "client_id": legacyID,
		"clientName": in.ClientName, "clientFirm": in.ClientFirm, "clientPhone": in.ClientPhone,
		"clientGST": in.ClientGST, "clientAddress": in.ClientAddress,
	}
	res, err := c.db.Collection(colClients).InsertOne(ctx, doc)
	if err != nil {
		return store.Client{}, store.DupNone, fmt.Errorf("insert client: %w", err)
	}
	lid := legacyID
	out := store.Client{
		UID: uid, CompanyID: companyID, LegacyID: &lid,
		ClientName: in.ClientName, ClientFirm: in.ClientFirm, ClientPhone: in.ClientPhone,
		ClientGST: in.ClientGST, ClientAddress: in.ClientAddress,
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		out.ID = idOf(oid)
	}
	return out, store.DupNone, nil
}

func (c *clients) Update(ctx context.Context, companyID, clientID store.ID, in store.ClientWrite) (store.Client, store.Dup, bool, error) {
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.Client{}, store.DupNone, false, err
	}
	clientOID, err := objectID(clientID)
	if err != nil {
		return store.Client{}, store.DupNone, false, nil // a malformed id is just "not found"
	}
	if dup, err := c.dupCheck(ctx, companyOID, &clientOID, in.ClientGST, in.ClientPhone); err != nil || dup != store.DupNone {
		return store.Client{}, dup, false, err
	}
	set := bson.M{
		"clientName": in.ClientName, "clientFirm": in.ClientFirm, "clientPhone": in.ClientPhone,
		"clientGST": in.ClientGST, "clientAddress": in.ClientAddress,
	}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var doc struct {
		clientDoc `bson:",inline"`
		LegacyID  any `bson:"client_id"`
	}
	err = c.db.Collection(colClients).FindOneAndUpdate(ctx,
		bson.M{"_id": clientOID, "company_id": companyOID}, bson.M{"$set": set}, opts).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return store.Client{}, store.DupNone, false, nil
	}
	if err != nil {
		return store.Client{}, store.DupNone, false, fmt.Errorf("update client: %w", err)
	}
	out := doc.clientDoc.toStore()
	out.LegacyID = doc.LegacyID
	return out, store.DupNone, true, nil
}

func (c *clients) Delete(ctx context.Context, companyID, clientID store.ID) (bool, error) {
	companyOID, err := objectID(companyID)
	if err != nil {
		return false, err
	}
	clientOID, err := objectID(clientID)
	if err != nil {
		return false, nil
	}
	res, err := c.db.Collection(colClients).DeleteOne(ctx, bson.M{"_id": clientOID, "company_id": companyOID})
	if err != nil {
		return false, fmt.Errorf("delete client: %w", err)
	}
	return res.DeletedCount > 0, nil
}

func (c *clients) dupCheck(ctx context.Context, companyOID primitive.ObjectID, excludeOID *primitive.ObjectID, gst, phone string) (store.Dup, error) {
	build := func(field, value string) bson.M {
		f := bson.M{field: value, "company_id": companyOID}
		if excludeOID != nil {
			f["_id"] = bson.M{"$ne": *excludeOID}
		}
		return f
	}
	n, err := c.db.Collection(colClients).CountDocuments(ctx, build("clientGST", gst))
	if err != nil {
		return store.DupNone, fmt.Errorf("gst dup check: %w", err)
	}
	if n > 0 {
		return store.DupGST, nil
	}
	n, err = c.db.Collection(colClients).CountDocuments(ctx, build("clientPhone", phone))
	if err != nil {
		return store.DupNone, fmt.Errorf("phone dup check: %w", err)
	}
	if n > 0 {
		return store.DupPhone, nil
	}
	return store.DupNone, nil
}

func (c *clients) EnsureSupplier(ctx context.Context, uid store.ID, in store.ClientWrite) (bool, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return false, err
	}
	match := bson.M{"uid": uidOID, "type": "Supplier"}
	if in.ClientGST != "" {
		match["gst"] = in.ClientGST
	} else {
		match["phone"] = in.ClientPhone
	}
	n, err := c.db.Collection(colPersons).CountDocuments(ctx, match)
	if err != nil {
		return false, fmt.Errorf("supplier match: %w", err)
	}
	if n > 0 {
		return false, nil
	}
	if _, err := c.db.Collection(colPersons).InsertOne(ctx, bson.M{
		"uid": uidOID, "type": "Supplier", "name": in.ClientName, "firm": in.ClientFirm,
		"phone": in.ClientPhone, "gst": in.ClientGST, "address": in.ClientAddress,
	}); err != nil {
		return false, fmt.Errorf("insert supplier: %w", err)
	}
	return true, nil
}

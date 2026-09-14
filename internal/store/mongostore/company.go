package mongostore

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type companies struct{ db *mongo.Database }

func (s *Store) Companies() store.Companies { return &companies{db: s.db} }

// companyDoc is the PUBLIC_FIELDS subset of Model/Company.js the shell needs.
type companyDoc struct {
	ID        primitive.ObjectID `bson:"_id"`
	Name      string             `bson:"name"`
	Firm      string             `bson:"firm"`
	Address   string             `bson:"address"`
	Phone     string             `bson:"phone"`
	Gst       string             `bson:"gst"`
	URL       string             `bson:"url"`
	UpiQr     string             `bson:"upiQr"`
	AccountNo string             `bson:"account_no"`
	Ifsc      string             `bson:"ifsc"`
	BankName  string             `bson:"bank_name"`
	IsDefault bool               `bson:"is_default"`
	IsActive  bool               `bson:"is_active"`
}

func (d companyDoc) toStore() store.Company {
	return store.Company{
		ID: idOf(d.ID), Name: d.Name, Firm: d.Firm, Address: d.Address, Phone: d.Phone,
		Gst: d.Gst, URL: d.URL, UpiQr: d.UpiQr, AccountNo: d.AccountNo, Ifsc: d.Ifsc,
		BankName: d.BankName, IsDefault: d.IsDefault, IsActive: d.IsActive,
	}
}

func (c *companies) List(ctx context.Context, uid store.ID) ([]store.Company, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return nil, err
	}
	// default first, then oldest, matching routes/Company.js's {is_default:-1, createdAt:1}.
	cur, err := c.db.Collection(colCompanies).Find(ctx,
		bson.M{"uid": uidOID, "is_active": true},
		options.Find().SetSort(bson.D{{Key: "is_default", Value: -1}, {Key: "createdAt", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("listing companies: %w", err)
	}
	defer cur.Close(ctx)

	var docs []companyDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("reading companies: %w", err)
	}
	out := make([]store.Company, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.toStore())
	}
	return out, nil
}

func (c *companies) Active(ctx context.Context, companyID, uid store.ID) (store.Company, error) {
	cOID, err := objectID(companyID)
	if err != nil {
		return store.Company{}, store.ErrNotFound
	}
	uidOID, err := objectID(uid)
	if err != nil {
		return store.Company{}, store.ErrNotFound
	}
	var doc companyDoc
	err = c.db.Collection(colCompanies).FindOne(ctx, bson.M{"_id": cOID, "uid": uidOID}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.Company{}, store.ErrNotFound
	}
	if err != nil {
		return store.Company{}, fmt.Errorf("looking up company: %w", err)
	}
	return doc.toStore(), nil
}

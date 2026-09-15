package mongostore

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

const colSupplierPayments = "supplierpayments"

type supplierPayments struct{ db *mongo.Database }

func (s *Store) SupplierPayments() store.SupplierPayments { return &supplierPayments{db: s.db} }

type spAlloc struct {
	PurchaseInvoiceID *primitive.ObjectID `bson:"purchase_invoice_id"`
	Amount            float64             `bson:"amount"`
}
type spDoc struct {
	ID          primitive.ObjectID  `bson:"_id"`
	UID         primitive.ObjectID  `bson:"uid"`
	CompanyID   *primitive.ObjectID `bson:"company_id"`
	SupplierID  *primitive.ObjectID `bson:"supplier_id"`
	BankID      *primitive.ObjectID `bson:"bank_id"`
	Date        string              `bson:"date"`
	Amount      float64             `bson:"amount"`
	Note        string              `bson:"note"`
	Mode        string              `bson:"mode"`
	Allocations []spAlloc           `bson:"allocations"`
	CreatedAt   time.Time           `bson:"createdAt"`
	UpdatedAt   time.Time           `bson:"updatedAt"`
	Version     int                 `bson:"__v"`
}

func (s *supplierPayments) List(ctx context.Context, uid, companyID store.ID) ([]store.SupplierPayment, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return nil, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	cur, err := s.db.Collection(colSupplierPayments).Find(ctx, bson.M{"uid": uidOID, "company_id": companyOID},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("listing supplier payments: %w", err)
	}
	defer cur.Close(ctx)
	var docs []spDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}

	supIDs, bankIDs, purIDs := map[primitive.ObjectID]struct{}{}, map[primitive.ObjectID]struct{}{}, map[primitive.ObjectID]struct{}{}
	for _, d := range docs {
		addID(supIDs, d.SupplierID)
		addID(bankIDs, d.BankID)
		for _, a := range d.Allocations {
			addID(purIDs, a.PurchaseInvoiceID)
		}
	}
	sups, err := s.personTriples(ctx, keys(supIDs))
	if err != nil {
		return nil, err
	}
	banks, err := s.names(ctx, colBanks, keys(bankIDs), "name")
	if err != nil {
		return nil, err
	}
	purNums, err := s.names(ctx, colPurchaseInvoices, keys(purIDs), "invoiceNumber")
	if err != nil {
		return nil, err
	}

	out := make([]store.SupplierPayment, 0, len(docs))
	for _, d := range docs {
		p := store.SupplierPayment{
			ID: idOf(d.ID), UID: idOf(d.UID), Date: d.Date, Amount: d.Amount, Note: d.Note, Mode: d.Mode,
			CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt, Version: d.Version,
		}
		if d.CompanyID != nil {
			p.CompanyID = idOf(*d.CompanyID)
		}
		if d.BankID != nil {
			p.BankID, p.BankName = idOf(*d.BankID), banks[*d.BankID]
		}
		if d.SupplierID != nil {
			t := sups[*d.SupplierID]
			p.SupplierID, p.SupplierName, p.SupplierFirm, p.SupplierPhone = idOf(*d.SupplierID), t.name, t.firm, t.phone
		}
		var dests []store.ReceiptDestination
		for _, a := range d.Allocations {
			if a.PurchaseInvoiceID != nil {
				dests = append(dests, store.ReceiptDestination{Kind: "purchase-invoice", ID: idOf(*a.PurchaseInvoiceID).String(), Label: purNums[*a.PurchaseInvoiceID], Amount: a.Amount})
			}
		}
		p.Destinations = dests
		out = append(out, p)
	}
	return out, nil
}

type personTriple struct{ name, firm, phone string }

func (s *supplierPayments) personTriples(ctx context.Context, ids []primitive.ObjectID) (map[primitive.ObjectID]personTriple, error) {
	out := map[primitive.ObjectID]personTriple{}
	if len(ids) == 0 {
		return out, nil
	}
	cur, err := s.db.Collection(colPersons).Find(ctx, bson.M{"_id": bson.M{"$in": ids}},
		options.Find().SetProjection(bson.M{"name": 1, "firm": 1, "phone": 1}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var rows []struct {
		ID    primitive.ObjectID `bson:"_id"`
		Name  string             `bson:"name"`
		Firm  string             `bson:"firm"`
		Phone string             `bson:"phone"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ID] = personTriple{r.Name, r.Firm, r.Phone}
	}
	return out, nil
}

func (s *supplierPayments) names(ctx context.Context, coll string, ids []primitive.ObjectID, field string) (map[primitive.ObjectID]string, error) {
	out := map[primitive.ObjectID]string{}
	if len(ids) == 0 {
		return out, nil
	}
	cur, err := s.db.Collection(coll).Find(ctx, bson.M{"_id": bson.M{"$in": ids}}, options.Find().SetProjection(bson.M{field: 1}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var rows []bson.M
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	for _, r := range rows {
		id, _ := r["_id"].(primitive.ObjectID)
		v, _ := r[field].(string)
		out[id] = v
	}
	return out, nil
}

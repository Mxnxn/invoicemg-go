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

const colPurchaseInvoices = "purchaseinvoices"

type purchaseInvoices struct{ db *mongo.Database }

func (s *Store) PurchaseInvoices() store.PurchaseInvoices { return &purchaseInvoices{db: s.db} }

type purchaseRowDoc struct {
	ID            primitive.ObjectID `bson:"_id"`
	Description   string             `bson:"description"`
	Material      string             `bson:"material"`
	Hsn           string             `bson:"hsn"`
	Gst           float64            `bson:"gst"`
	HasDimensions *bool              `bson:"hasDimensions"`
	Length        string             `bson:"length"`
	Width         string             `bson:"width"`
	Rate          float64            `bson:"rate"`
	Qty           float64            `bson:"qty"`
	Unit          string             `bson:"unit"`
	Discount      float64            `bson:"discount"`
	Charges       float64            `bson:"charges"`
	CreatedAt     time.Time          `bson:"createdAt"`
	UpdatedAt     time.Time          `bson:"updatedAt"`
}

type purchaseInvoiceDoc struct {
	ID            primitive.ObjectID  `bson:"_id"`
	UID           primitive.ObjectID  `bson:"uid"`
	CompanyID     *primitive.ObjectID `bson:"company_id"`
	SupplierID    *primitive.ObjectID `bson:"supplier_id"`
	Date          string              `bson:"date"`
	InvoiceNumber string              `bson:"invoiceNumber"`
	Rows          []purchaseRowDoc    `bson:"rows"`
	Total         float64             `bson:"total"`
	Amount        float64             `bson:"amount"`
	CreatedAt     time.Time           `bson:"createdAt"`
	UpdatedAt     time.Time           `bson:"updatedAt"`
	Version       int                 `bson:"__v"`
}

func (p *purchaseInvoices) List(ctx context.Context, uid, companyID store.ID) ([]store.PurchaseInvoice, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return nil, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	cur, err := p.db.Collection(colPurchaseInvoices).Find(ctx,
		bson.M{"uid": uidOID, "company_id": companyOID},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("listing purchase invoices: %w", err)
	}
	defer cur.Close(ctx)

	var docs []purchaseInvoiceDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("reading purchase invoices: %w", err)
	}

	// #6: batch-populate supplier_id from the people collection in one query.
	suppliers, err := p.suppliersByID(ctx, docs)
	if err != nil {
		return nil, err
	}

	out := make([]store.PurchaseInvoice, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.toStore(suppliers))
	}
	return out, nil
}

func (p *purchaseInvoices) suppliersByID(ctx context.Context, docs []purchaseInvoiceDoc) (map[primitive.ObjectID]store.PurchaseSupplier, error) {
	idSet := map[primitive.ObjectID]struct{}{}
	for _, d := range docs {
		if d.SupplierID != nil {
			idSet[*d.SupplierID] = struct{}{}
		}
	}
	out := map[primitive.ObjectID]store.PurchaseSupplier{}
	if len(idSet) == 0 {
		return out, nil
	}
	ids := make([]primitive.ObjectID, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	cur, err := p.db.Collection(colPersons).Find(ctx, bson.M{"_id": bson.M{"$in": ids}},
		options.Find().SetProjection(bson.M{"name": 1, "firm": 1, "phone": 1}))
	if err != nil {
		return nil, fmt.Errorf("populating suppliers: %w", err)
	}
	defer cur.Close(ctx)
	var rows []struct {
		ID    primitive.ObjectID `bson:"_id"`
		Name  string             `bson:"name"`
		Firm  string             `bson:"firm"`
		Phone string             `bson:"phone"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("reading suppliers: %w", err)
	}
	for _, r := range rows {
		out[r.ID] = store.PurchaseSupplier{ID: idOf(r.ID), Name: r.Name, Firm: r.Firm, Phone: r.Phone}
	}
	return out, nil
}

func (d purchaseInvoiceDoc) toStore(suppliers map[primitive.ObjectID]store.PurchaseSupplier) store.PurchaseInvoice {
	inv := store.PurchaseInvoice{
		ID: idOf(d.ID), UID: idOf(d.UID), Date: d.Date, InvoiceNumber: d.InvoiceNumber,
		Total: d.Total, Amount: d.Amount, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt, Version: d.Version,
	}
	if d.CompanyID != nil {
		inv.CompanyID = idOf(*d.CompanyID)
	}
	if d.SupplierID != nil {
		if s, ok := suppliers[*d.SupplierID]; ok {
			inv.Supplier = &s
		}
	}
	inv.Rows = make([]store.PurchaseInvoiceRow, 0, len(d.Rows))
	for _, r := range d.Rows {
		inv.Rows = append(inv.Rows, store.PurchaseInvoiceRow{
			ID: idOf(r.ID), Description: r.Description, Material: r.Material, Hsn: r.Hsn, Gst: r.Gst,
			HasDimensions: r.HasDimensions, Length: r.Length, Width: r.Width, Rate: r.Rate, Qty: r.Qty,
			Unit: r.Unit, Discount: r.Discount, Charges: r.Charges, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		})
	}
	return inv
}

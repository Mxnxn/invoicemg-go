package mongostore

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

const (
	colInvoices = "invoices"
	colEntries  = "entries"
)

type invoices struct{ db *mongo.Database }

func (s *Store) Invoices() store.Invoices { return &invoices{db: s.db} }

type invoiceDoc struct {
	ID          primitive.ObjectID   `bson:"_id"`
	Client      *primitive.ObjectID  `bson:"client"`
	Entries     []primitive.ObjectID `bson:"entries"`
	InvoiceID   string               `bson:"invoiceId"`
	Date        string               `bson:"date"`
	Amount      float64              `bson:"amount"`
	TotalAmount float64              `bson:"totalAmount"`
	CreatedAt   time.Time            `bson:"createdAt"`
}

type entryDoc struct {
	ID            primitive.ObjectID `bson:"_id"`
	Description   string             `bson:"description"`
	Material      string             `bson:"material"`
	Hsn           string             `bson:"hsn"`
	Unit          string             `bson:"unit"`
	Rate          float64            `bson:"rate"`
	Qty           float64            `bson:"qty"`
	HasDimensions *bool              `bson:"hasDimensions"`
	Length        string             `bson:"length"`
	Width         string             `bson:"width"`
	Date          string             `bson:"date"`
	Amount        float64            `bson:"amount"`
	Cgst          float64            `bson:"cgst"`
	Sgst          float64            `bson:"sgst"`
	Igst          float64            `bson:"igst"`
	Discount      float64            `bson:"discount"`
	Charges       float64            `bson:"charges"`
	Advance       float64            `bson:"advance"`
	Total         float64            `bson:"total"`
	CreatedAt     time.Time          `bson:"createdAt"`
	UpdatedAt     time.Time          `bson:"updatedAt"`
	Version       int                `bson:"__v"`
}

func (e entryDoc) toStore() store.Entry {
	return store.Entry{
		ID: idOf(e.ID), Description: e.Description, Material: e.Material, Hsn: e.Hsn, Unit: e.Unit,
		Rate: e.Rate, Qty: e.Qty, HasDimensions: e.HasDimensions, Length: e.Length, Width: e.Width,
		Date: e.Date, Amount: e.Amount, Cgst: e.Cgst, Sgst: e.Sgst, Igst: e.Igst,
		Discount: e.Discount, Charges: e.Charges, Advance: e.Advance, Total: e.Total,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt, Version: e.Version,
	}
}

func (i *invoices) List(ctx context.Context, companyID store.ID) ([]store.Invoice, error) {
	companyOID, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	cur, err := i.db.Collection(colInvoices).Find(ctx, bson.M{"company_id": companyOID})
	if err != nil {
		return nil, fmt.Errorf("listing invoices: %w", err)
	}
	defer cur.Close(ctx)
	var docs []invoiceDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("reading invoices: %w", err)
	}

	// #6: batch both populates. Collect every client id and every entry id across all invoices,
	// resolve each set in ONE query, then stitch - never a query per invoice.
	clientIDs := map[primitive.ObjectID]struct{}{}
	entryIDs := map[primitive.ObjectID]struct{}{}
	for _, d := range docs {
		if d.Client != nil {
			clientIDs[*d.Client] = struct{}{}
		}
		for _, e := range d.Entries {
			entryIDs[e] = struct{}{}
		}
	}
	clients, err := i.clientsByID(ctx, keys(clientIDs))
	if err != nil {
		return nil, err
	}
	entries, err := i.entriesByID(ctx, keys(entryIDs))
	if err != nil {
		return nil, err
	}

	out := make([]store.Invoice, 0, len(docs))
	for _, d := range docs {
		inv := store.Invoice{
			ID: idOf(d.ID), InvoiceID: d.InvoiceID, Date: d.Date,
			Amount: d.Amount, TotalAmount: d.TotalAmount, CreatedAt: d.CreatedAt,
		}
		if d.Client != nil {
			if c, ok := clients[*d.Client]; ok {
				inv.Client = &c
			}
		}
		inv.Entries = make([]store.Entry, 0, len(d.Entries))
		for _, eid := range d.Entries { // preserve the invoice's own entry order
			if e, ok := entries[eid]; ok {
				inv.Entries = append(inv.Entries, e)
			}
		}
		out = append(out, inv)
	}
	return out, nil
}

func keys(m map[primitive.ObjectID]struct{}) []primitive.ObjectID {
	out := make([]primitive.ObjectID, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func (i *invoices) clientsByID(ctx context.Context, ids []primitive.ObjectID) (map[primitive.ObjectID]store.InvoiceClient, error) {
	out := map[primitive.ObjectID]store.InvoiceClient{}
	if len(ids) == 0 {
		return out, nil
	}
	cur, err := i.db.Collection(colClients).Find(ctx, bson.M{"_id": bson.M{"$in": ids}})
	if err != nil {
		return nil, fmt.Errorf("populating invoice clients: %w", err)
	}
	defer cur.Close(ctx)
	var rows []struct {
		ID            primitive.ObjectID `bson:"_id"`
		UID           primitive.ObjectID `bson:"uid"`
		ClientName    string             `bson:"clientName"`
		ClientFirm    string             `bson:"clientFirm"`
		ClientPhone   string             `bson:"clientPhone"`
		ClientAddress string             `bson:"clientAddress"`
		ClientGST     string             `bson:"clientGST"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("reading invoice clients: %w", err)
	}
	for _, r := range rows {
		out[r.ID] = store.InvoiceClient{
			ID: idOf(r.ID), UID: idOf(r.UID), ClientName: r.ClientName, ClientFirm: r.ClientFirm,
			ClientPhone: r.ClientPhone, ClientAddress: r.ClientAddress, ClientGST: r.ClientGST,
		}
	}
	return out, nil
}

func (i *invoices) entriesByID(ctx context.Context, ids []primitive.ObjectID) (map[primitive.ObjectID]store.Entry, error) {
	out := map[primitive.ObjectID]store.Entry{}
	if len(ids) == 0 {
		return out, nil
	}
	cur, err := i.db.Collection(colEntries).Find(ctx, bson.M{"_id": bson.M{"$in": ids}})
	if err != nil {
		return nil, fmt.Errorf("populating entries: %w", err)
	}
	defer cur.Close(ctx)
	var docs []entryDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("reading entries: %w", err)
	}
	for _, d := range docs {
		out[d.ID] = d.toStore()
	}
	return out, nil
}

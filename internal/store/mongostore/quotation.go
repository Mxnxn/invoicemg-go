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

const colQuotations = "quotations"

type quotations struct{ db *mongo.Database }

func (s *Store) Quotations() store.Quotations { return &quotations{db: s.db} }

type quotationRowDoc struct {
	ID            primitive.ObjectID  `bson:"_id"`
	Material      string              `bson:"material"`
	Description   string              `bson:"description"`
	HasDimensions *bool               `bson:"hasDimensions"`
	Length        string              `bson:"length"`
	Width         string              `bson:"width"`
	Qty           float64             `bson:"qty"`
	Rate          float64             `bson:"rate"`
	Cgst          float64             `bson:"cgst"`
	Sgst          float64             `bson:"sgst"`
	Igst          float64             `bson:"igst"`
	Discount      float64             `bson:"discount"`
	Charges       float64             `bson:"charges"`
	JobID         *primitive.ObjectID `bson:"job_id"`
	CreatedAt     time.Time           `bson:"createdAt"`
	UpdatedAt     time.Time           `bson:"updatedAt"`
}

type quotationDoc struct {
	ID              primitive.ObjectID  `bson:"_id"`
	UID             primitive.ObjectID  `bson:"uid"`
	CompanyID       *primitive.ObjectID `bson:"company_id"`
	ClientID        *primitive.ObjectID `bson:"client_id"`
	QuotationNumber string              `bson:"quotationNumber"`
	Date            string              `bson:"date"`
	Rows            []quotationRowDoc   `bson:"rows"`
	CreatedAt       time.Time           `bson:"createdAt"`
	UpdatedAt       time.Time           `bson:"updatedAt"`
	Version         int                 `bson:"__v"`
}

func (q *quotations) List(ctx context.Context, uid, companyID, clientID store.ID) ([]store.Quotation, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return nil, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	filter := bson.M{"uid": uidOID, "company_id": companyOID}
	if clientID != "" {
		clientOID, err := objectID(clientID)
		if err != nil {
			return nil, store.ErrBadID
		}
		filter["client_id"] = clientOID
	}
	cur, err := q.db.Collection(colQuotations).Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("listing quotations: %w", err)
	}
	defer cur.Close(ctx)

	var docs []quotationDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("reading quotations: %w", err)
	}

	// #6: batch-populate client_id. Collect every distinct client id and resolve them in ONE
	// query, rather than a lookup per quotation (which would be the naive N+1 port of populate).
	clients, err := q.clientsByID(ctx, docs)
	if err != nil {
		return nil, err
	}

	out := make([]store.Quotation, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.toStore(clients))
	}
	return out, nil
}

func (q *quotations) clientsByID(ctx context.Context, docs []quotationDoc) (map[primitive.ObjectID]store.QuotationClient, error) {
	idSet := map[primitive.ObjectID]struct{}{}
	for _, d := range docs {
		if d.ClientID != nil {
			idSet[*d.ClientID] = struct{}{}
		}
	}
	out := map[primitive.ObjectID]store.QuotationClient{}
	if len(idSet) == 0 {
		return out, nil
	}
	ids := make([]primitive.ObjectID, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	cur, err := q.db.Collection(colClients).Find(ctx, bson.M{"_id": bson.M{"$in": ids}},
		options.Find().SetProjection(bson.M{
			"clientName": 1, "clientFirm": 1, "clientPhone": 1, "clientAddress": 1, "clientGST": 1,
		}))
	if err != nil {
		return nil, fmt.Errorf("populating clients: %w", err)
	}
	defer cur.Close(ctx)
	var rows []struct {
		ID            primitive.ObjectID `bson:"_id"`
		ClientName    string             `bson:"clientName"`
		ClientFirm    string             `bson:"clientFirm"`
		ClientPhone   string             `bson:"clientPhone"`
		ClientAddress string             `bson:"clientAddress"`
		ClientGST     string             `bson:"clientGST"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("reading clients: %w", err)
	}
	for _, r := range rows {
		out[r.ID] = store.QuotationClient{
			ID: idOf(r.ID), ClientName: r.ClientName, ClientFirm: r.ClientFirm,
			ClientPhone: r.ClientPhone, ClientAddress: r.ClientAddress, ClientGST: r.ClientGST,
		}
	}
	return out, nil
}

func (d quotationDoc) toStore(clients map[primitive.ObjectID]store.QuotationClient) store.Quotation {
	quo := store.Quotation{
		ID: idOf(d.ID), UID: idOf(d.UID), QuotationNumber: d.QuotationNumber, Date: d.Date,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt, Version: d.Version,
	}
	if d.CompanyID != nil {
		quo.CompanyID = idOf(*d.CompanyID)
	}
	// populate: client_id becomes the client object, or nil (null) when the ref is dangling.
	if d.ClientID != nil {
		if c, ok := clients[*d.ClientID]; ok {
			quo.Client = &c
		}
	}
	quo.Rows = make([]store.QuotationRow, 0, len(d.Rows))
	for _, r := range d.Rows {
		row := store.QuotationRow{
			ID: idOf(r.ID), Material: r.Material, Description: r.Description,
			HasDimensions: r.HasDimensions, Length: r.Length, Width: r.Width,
			Qty: r.Qty, Rate: r.Rate, Cgst: r.Cgst, Sgst: r.Sgst, Igst: r.Igst,
			Discount: r.Discount, Charges: r.Charges, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		}
		if r.JobID != nil {
			row.JobID = idOf(*r.JobID)
		}
		quo.Rows = append(quo.Rows, row)
	}
	return quo
}

package mongostore

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type statistics struct{ db *mongo.Database }

func (s *Store) Statistics() store.Statistics { return &statistics{db: s.db} }

func (s *statistics) Clients(ctx context.Context, companyID store.ID) ([]store.LookupClient, error) {
	oid, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	cur, err := s.db.Collection(colClients).Find(ctx, bson.M{"company_id": oid},
		options.Find().SetProjection(bson.M{"clientName": 1, "clientFirm": 1}))
	if err != nil {
		return nil, fmt.Errorf("stats clients: %w", err)
	}
	defer cur.Close(ctx)
	var docs []clientDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]store.LookupClient, 0, len(docs))
	for _, d := range docs {
		out = append(out, store.LookupClient{ID: idOf(d.ID), ClientName: d.ClientName, ClientFirm: d.ClientFirm})
	}
	return out, nil
}

func (s *statistics) Client(ctx context.Context, companyID, clientID store.ID) (store.LookupClient, error) {
	cOID, err := objectID(clientID)
	if err != nil {
		return store.LookupClient{}, store.ErrNotFound
	}
	coOID, err := objectID(companyID)
	if err != nil {
		return store.LookupClient{}, store.ErrNotFound
	}
	var d clientDoc
	err = s.db.Collection(colClients).FindOne(ctx, bson.M{"_id": cOID, "company_id": coOID},
		options.FindOne().SetProjection(bson.M{"clientName": 1, "clientFirm": 1})).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.LookupClient{}, store.ErrNotFound
	}
	if err != nil {
		return store.LookupClient{}, err
	}
	return store.LookupClient{ID: idOf(d.ID), ClientName: d.ClientName, ClientFirm: d.ClientFirm}, nil
}

func (s *statistics) StatEntries(ctx context.Context, companyID store.ID, monthName string, paid bool, clientID store.ID) ([]store.StatEntry, error) {
	oid, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	filter := bson.M{"company_id": oid, "date": bson.M{"$regex": monthName, "$options": "i"}}
	if paid {
		filter["total"] = 0
	} else {
		filter["total"] = bson.M{"$gt": 0}
	}
	if clientID != "" {
		cOID, err := objectID(clientID)
		if err != nil {
			return nil, store.ErrBadID
		}
		filter["client_id"] = cOID
	}
	cur, err := s.db.Collection(colEntries).Find(ctx, filter,
		options.Find().SetProjection(bson.M{"material": 1, "length": 1, "width": 1, "qty": 1, "date": 1, "hasDimensions": 1}))
	if err != nil {
		return nil, fmt.Errorf("stat entries: %w", err)
	}
	defer cur.Close(ctx)
	var docs []struct {
		Material      string  `bson:"material"`
		Length        string  `bson:"length"`
		Width         string  `bson:"width"`
		Qty           float64 `bson:"qty"`
		Date          string  `bson:"date"`
		HasDimensions *bool   `bson:"hasDimensions"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]store.StatEntry, 0, len(docs))
	for _, d := range docs {
		out = append(out, store.StatEntry{Material: d.Material, Length: d.Length, Width: d.Width, Qty: d.Qty, Date: d.Date, HasDimensions: d.HasDimensions})
	}
	return out, nil
}

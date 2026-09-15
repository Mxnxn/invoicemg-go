package mongostore

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type lookups struct{ db *mongo.Database }

func (s *Store) Lookups() store.Lookups { return &lookups{db: s.db} }

func (l *lookups) Clients(ctx context.Context, uid, companyID store.ID) ([]store.LookupClient, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return nil, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	cur, err := l.db.Collection(colClients).Find(ctx, bson.M{"uid": uidOID, "company_id": companyOID},
		options.Find().SetProjection(bson.M{"clientName": 1, "clientFirm": 1, "clientPhone": 1}))
	if err != nil {
		return nil, fmt.Errorf("lookup clients: %w", err)
	}
	defer cur.Close(ctx)
	var docs []clientDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("reading client lookups: %w", err)
	}
	out := make([]store.LookupClient, 0, len(docs))
	for _, d := range docs {
		out = append(out, store.LookupClient{
			ID: idOf(d.ID), ClientName: d.ClientName, ClientFirm: d.ClientFirm, ClientPhone: d.ClientPhone,
		})
	}
	return out, nil
}

func (l *lookups) Materials(ctx context.Context, companyID store.ID) ([]store.LookupMaterial, error) {
	companyOID, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	cur, err := l.db.Collection(colMaterials).Find(ctx, bson.M{"company_id": companyOID},
		options.Find().SetProjection(bson.M{"material_name": 1, "material_rate": 1, "hsn": 1, "tax": 1}))
	if err != nil {
		return nil, fmt.Errorf("lookup materials: %w", err)
	}
	defer cur.Close(ctx)
	var docs []materialDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("reading material lookups: %w", err)
	}
	out := make([]store.LookupMaterial, 0, len(docs))
	for _, d := range docs {
		out = append(out, store.LookupMaterial{
			ID: idOf(d.ID), MaterialName: d.MaterialName, MaterialRate: d.MaterialRate, Hsn: d.Hsn, Tax: d.Tax,
		})
	}
	return out, nil
}

func (l *lookups) People(ctx context.Context, uid store.ID) ([]store.LookupPerson, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return nil, err
	}
	cur, err := l.db.Collection(colPersons).Find(ctx, bson.M{"uid": uidOID, "is_active": true},
		options.Find().SetProjection(bson.M{"name": 1, "type": 1}))
	if err != nil {
		return nil, fmt.Errorf("lookup people: %w", err)
	}
	defer cur.Close(ctx)
	var docs []personDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("reading people lookups: %w", err)
	}
	out := make([]store.LookupPerson, 0, len(docs))
	for _, d := range docs {
		out = append(out, store.LookupPerson{ID: idOf(d.ID), Name: d.Name, Type: d.Type})
	}
	return out, nil
}

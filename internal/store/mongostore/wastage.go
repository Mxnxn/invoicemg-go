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

const colWastages = "wastages"

type wastages struct{ db *mongo.Database }

func (s *Store) Wastages() store.Wastages { return &wastages{db: s.db} }

type wastageDoc struct {
	ID           primitive.ObjectID  `bson:"_id"`
	UID          primitive.ObjectID  `bson:"uid"`
	CompanyID    *primitive.ObjectID `bson:"company_id"`
	MaterialName string              `bson:"material_name"`
	Rate         float64             `bson:"rate"`
	PurchaseRate float64             `bson:"purchase_rate"`
	CostTotal    float64             `bson:"cost_total"`
	Length       float64             `bson:"length"`
	Height       float64             `bson:"height"`
	Total        float64             `bson:"total"`
	Date         string              `bson:"date"`
	CreatedAt    time.Time           `bson:"createdAt"`
	UpdatedAt    time.Time           `bson:"updatedAt"`
	Version      int                 `bson:"__v"`
}

func (w *wastages) List(ctx context.Context, companyID store.ID) ([]store.Wastage, error) {
	oid, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	cur, err := w.db.Collection(colWastages).Find(ctx, bson.M{"company_id": oid},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("listing wastages: %w", err)
	}
	defer cur.Close(ctx)
	var docs []wastageDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("reading wastages: %w", err)
	}
	out := make([]store.Wastage, 0, len(docs))
	for _, d := range docs {
		wa := store.Wastage{
			ID: idOf(d.ID), UID: idOf(d.UID), MaterialName: d.MaterialName, Rate: d.Rate,
			PurchaseRate: d.PurchaseRate, CostTotal: d.CostTotal, Length: d.Length, Height: d.Height,
			Total: d.Total, Date: d.Date, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt, Version: d.Version,
		}
		if d.CompanyID != nil {
			wa.CompanyID = idOf(*d.CompanyID)
		}
		out = append(out, wa)
	}
	return out, nil
}

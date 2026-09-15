package mongostore

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

const colChallans = "challans"

type challans struct{ db *mongo.Database }

func (s *Store) Challans() store.Challans { return &challans{db: s.db} }

type challanDoc struct {
	ID          primitive.ObjectID  `bson:"_id"`
	UID         primitive.ObjectID  `bson:"uid"`
	CompanyID   *primitive.ObjectID `bson:"company_id"`
	CompanyName string              `bson:"companyName"`
	Description string              `bson:"description"`
	Date        string              `bson:"date"`
	Type        string              `bson:"type"`
	Quantity    float64             `bson:"quantity"`
	Amount      float64             `bson:"amount"`
	Version     int                 `bson:"__v"`
}

func (c *challans) List(ctx context.Context, companyID store.ID) ([]store.Challan, error) {
	oid, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	cur, err := c.db.Collection(colChallans).Find(ctx, bson.M{"company_id": oid})
	if err != nil {
		return nil, fmt.Errorf("listing challans: %w", err)
	}
	defer cur.Close(ctx)
	var docs []challanDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("reading challans: %w", err)
	}
	out := make([]store.Challan, 0, len(docs))
	for _, d := range docs {
		ch := store.Challan{
			ID: idOf(d.ID), UID: idOf(d.UID), CompanyName: d.CompanyName, Description: d.Description,
			Date: d.Date, Type: d.Type, Quantity: d.Quantity, Amount: d.Amount, Version: d.Version,
		}
		if d.CompanyID != nil {
			ch.CompanyID = idOf(*d.CompanyID)
		}
		out = append(out, ch)
	}
	return out, nil
}

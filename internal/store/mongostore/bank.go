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

const colBanks = "banks"

type banks struct{ db *mongo.Database }

func (s *Store) Banks() store.Banks { return &banks{db: s.db} }

// bankDoc mirrors Model/Bank.js. Version carries __v because /bank/list returns whole documents
// and Node's response includes it.
type bankDoc struct {
	ID             primitive.ObjectID  `bson:"_id"`
	UID            primitive.ObjectID  `bson:"uid"`
	CompanyID      *primitive.ObjectID `bson:"company_id"`
	Name           string              `bson:"name"`
	OpeningBalance float64             `bson:"openingBalance"`
	CreatedAt      time.Time           `bson:"createdAt"`
	UpdatedAt      time.Time           `bson:"updatedAt"`
	Version        int                 `bson:"__v"`
}

func (d bankDoc) toStore() store.Bank {
	b := store.Bank{
		ID:             idOf(d.ID),
		UID:            idOf(d.UID),
		Name:           d.Name,
		OpeningBalance: d.OpeningBalance,
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
		Version:        d.Version,
	}
	if d.CompanyID != nil {
		b.CompanyID = idOf(*d.CompanyID)
	}
	return b
}

func (b *banks) List(ctx context.Context, companyID store.ID) ([]store.Bank, error) {
	oid, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	// sort by name only, matching routes/Bank.js exactly - no _id tiebreak here, because relying
	// on Mongo's natural order for equal names is what keeps this identical to the live service
	// while the two run side by side. The sqlstore adds the tiebreak (#19).
	cur, err := b.db.Collection(colBanks).Find(ctx, bson.M{"company_id": oid},
		options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("listing banks: %w", err)
	}
	defer cur.Close(ctx)

	var docs []bankDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("reading banks: %w", err)
	}
	out := make([]store.Bank, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.toStore())
	}
	return out, nil
}

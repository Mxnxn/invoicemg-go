package mongostore

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

const colInvoiceReceived = "invoicereceiveds"

type analytics struct{ db *mongo.Database }

func (s *Store) Analytics() store.Analytics { return &analytics{db: s.db} }

// revRow carries both date sources and every amount field the revenue series reads; the caller
// picks which amount by collection.
type revRow struct {
	CreatedAt   *time.Time `bson:"createdAt"`
	Date        string     `bson:"date"`
	Total       float64    `bson:"total"`
	Amount      float64    `bson:"amount"`
	TotalAmount float64    `bson:"totalAmount"`
}

// effective resolves createdAt || date (getEffectiveDate).
func (r revRow) effective() (time.Time, bool) {
	if r.CreatedAt != nil && !r.CreatedAt.IsZero() {
		return r.CreatedAt.UTC(), true
	}
	if r.Date != "" {
		if t, err := time.Parse("2006-01-02", r.Date); err == nil {
			return t.UTC(), true
		}
		if t, err := time.Parse(time.RFC3339, r.Date); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

func (a *analytics) series(ctx context.Context, coll string, filter bson.M, amount func(revRow) float64) ([]store.DatedAmount, error) {
	cur, err := a.db.Collection(coll).Find(ctx, filter,
		options.Find().SetProjection(bson.M{"createdAt": 1, "date": 1, "total": 1, "amount": 1, "totalAmount": 1}))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", coll, err)
	}
	defer cur.Close(ctx)
	var rows []revRow
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	out := make([]store.DatedAmount, 0, len(rows))
	for _, r := range rows {
		if t, ok := r.effective(); ok {
			out = append(out, store.DatedAmount{Date: t, Amount: amount(r)})
		}
	}
	return out, nil
}

func (a *analytics) RevenueSeries(ctx context.Context, companyID store.ID, source string) ([]store.DatedAmount, []store.DatedAmount, error) {
	oid, err := objectID(companyID)
	if err != nil {
		return nil, nil, err
	}
	filter := bson.M{"company_id": oid}
	if source == "all" {
		billed, err := a.series(ctx, colEntries, filter, func(r revRow) float64 { return r.Total })
		if err != nil {
			return nil, nil, err
		}
		collected, err := a.series(ctx, colEntries, filter, func(r revRow) float64 { return r.Amount })
		if err != nil {
			return nil, nil, err
		}
		return billed, collected, nil
	}
	billed, err := a.series(ctx, colInvoices, filter, func(r revRow) float64 { return r.TotalAmount })
	if err != nil {
		return nil, nil, err
	}
	collected, err := a.series(ctx, colInvoiceReceived, filter, func(r revRow) float64 { return r.Amount })
	if err != nil {
		return nil, nil, err
	}
	return billed, collected, nil
}

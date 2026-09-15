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

const colExpenses = "expenses"

type expenses struct{ db *mongo.Database }

func (s *Store) Expenses() store.Expenses { return &expenses{db: s.db} }

type expenseDoc struct {
	ID        primitive.ObjectID  `bson:"_id"`
	UID       primitive.ObjectID  `bson:"uid"`
	CompanyID *primitive.ObjectID `bson:"company_id"`
	BankID    *primitive.ObjectID `bson:"bank_id"`
	Date      string              `bson:"date"`
	Amount    float64             `bson:"amount"`
	Notes     string              `bson:"notes"`
	CreatedAt time.Time           `bson:"createdAt"`
	UpdatedAt time.Time           `bson:"updatedAt"`
	Version   int                 `bson:"__v"`
}

func (e *expenses) List(ctx context.Context, companyID store.ID, f store.ExpenseFilter) ([]store.Expense, error) {
	oid, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	filter := bson.M{"company_id": oid}
	if f.BankID != "" {
		bOID, err := objectID(f.BankID)
		if err != nil {
			return nil, store.ErrBadID
		}
		filter["bank_id"] = bOID
	}
	if f.From != "" || f.To != "" {
		dr := bson.M{}
		if f.From != "" {
			dr["$gte"] = f.From
		}
		if f.To != "" {
			dr["$lte"] = f.To
		}
		filter["date"] = dr
	}
	cur, err := e.db.Collection(colExpenses).Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "date", Value: -1}, {Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("listing expenses: %w", err)
	}
	defer cur.Close(ctx)
	var docs []expenseDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("reading expenses: %w", err)
	}

	// #6: batch-populate bank_id.
	bankIDs := map[primitive.ObjectID]struct{}{}
	for _, d := range docs {
		addID(bankIDs, d.BankID)
	}
	banks, err := e.bankNames(ctx, keys(bankIDs))
	if err != nil {
		return nil, err
	}

	out := make([]store.Expense, 0, len(docs))
	for _, d := range docs {
		ex := store.Expense{
			ID: idOf(d.ID), UID: idOf(d.UID), Date: d.Date, Amount: d.Amount, Notes: d.Notes,
			CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt, Version: d.Version,
		}
		if d.CompanyID != nil {
			ex.CompanyID = idOf(*d.CompanyID)
		}
		if d.BankID != nil {
			if name, ok := banks[*d.BankID]; ok {
				b := store.ExpenseBank{ID: idOf(*d.BankID), Name: name}
				ex.Bank = &b
			}
		}
		out = append(out, ex)
	}
	return out, nil
}

func (e *expenses) bankNames(ctx context.Context, ids []primitive.ObjectID) (map[primitive.ObjectID]string, error) {
	out := map[primitive.ObjectID]string{}
	if len(ids) == 0 {
		return out, nil
	}
	cur, err := e.db.Collection(colBanks).Find(ctx, bson.M{"_id": bson.M{"$in": ids}},
		options.Find().SetProjection(bson.M{"name": 1}))
	if err != nil {
		return nil, fmt.Errorf("populating expense banks: %w", err)
	}
	defer cur.Close(ctx)
	var rows []struct {
		ID   primitive.ObjectID `bson:"_id"`
		Name string             `bson:"name"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ID] = r.Name
	}
	return out, nil
}

package mongostore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

const (
	colTrash      = "trashes"
	colTrashUsers = "trashusers"
)

type trash struct{ db *mongo.Database }

func (s *Store) Trash() store.Trash { return &trash{db: s.db} }

func (t *trash) PasswordHash(ctx context.Context, uid store.ID) (string, error) {
	oid, err := objectID(uid)
	if err != nil {
		return "", err
	}
	var doc struct {
		Password string `bson:"password"`
	}
	err = t.db.Collection(colTrashUsers).FindOne(ctx, bson.M{"uid": oid}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("trash password: %w", err)
	}
	return doc.Password, nil
}

func (t *trash) SetPassword(ctx context.Context, uid store.ID, hash string) error {
	oid, err := objectID(uid)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	_, err = t.db.Collection(colTrashUsers).UpdateOne(ctx, bson.M{"uid": oid},
		bson.M{"$set": bson.M{"password": hash, "updatedAt": now}, "$setOnInsert": bson.M{"uid": oid, "createdAt": now}},
		options.Update().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("set trash password: %w", err)
	}
	return nil
}

func (t *trash) List(ctx context.Context, companyID store.ID) ([]store.TrashRecord, error) {
	oid, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	cur, err := t.db.Collection(colTrash).Find(ctx, bson.M{"company_id": oid})
	if err != nil {
		return nil, fmt.Errorf("listing trash: %w", err)
	}
	defer cur.Close(ctx)
	var docs []struct {
		ID            primitive.ObjectID  `bson:"_id"`
		ClientID      *primitive.ObjectID `bson:"client_id"`
		Description   string              `bson:"description"`
		Material      string              `bson:"material"`
		Rate          float64             `bson:"rate"`
		Qty           float64             `bson:"qty"`
		HasDimensions *bool               `bson:"hasDimensions"`
		Length        string              `bson:"length"`
		Width         string              `bson:"width"`
		Date          string              `bson:"date"`
		Amount        float64             `bson:"amount"`
		Cgst          float64             `bson:"cgst"`
		Sgst          float64             `bson:"sgst"`
		Igst          float64             `bson:"igst"`
		CreatedAt     time.Time           `bson:"createdAt"`
		UpdatedAt     time.Time           `bson:"updatedAt"`
		Version       int                 `bson:"__v"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	cids := map[primitive.ObjectID]struct{}{}
	for _, d := range docs {
		if d.ClientID != nil {
			cids[*d.ClientID] = struct{}{}
		}
	}
	names := map[primitive.ObjectID]clientNameFirm{}
	if len(cids) > 0 {
		ids := make([]primitive.ObjectID, 0, len(cids))
		for id := range cids {
			ids = append(ids, id)
		}
		c, err := (&analytics{db: t.db}).clientNames(ctx, ids)
		if err != nil {
			return nil, err
		}
		names = c
	}
	out := make([]store.TrashRecord, 0, len(docs))
	for _, d := range docs {
		rec := store.TrashRecord{
			ID: idOf(d.ID), Description: d.Description, Material: d.Material, Rate: d.Rate, Qty: d.Qty,
			HasDimensions: d.HasDimensions, Length: d.Length, Width: d.Width, Date: d.Date, Amount: d.Amount,
			Cgst: d.Cgst, Sgst: d.Sgst, Igst: d.Igst, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt, Version: d.Version,
		}
		if d.ClientID != nil {
			rec.ClientID = idOf(*d.ClientID)
			if c, ok := names[*d.ClientID]; ok {
				rec.ClientName, rec.ClientFirm = c.name, c.firm
			}
		}
		out = append(out, rec)
	}
	return out, nil
}

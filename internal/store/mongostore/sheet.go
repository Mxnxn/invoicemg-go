package mongostore

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type sheets struct{ db *mongo.Database }

func (s *Store) Sheets() store.Sheets { return &sheets{db: s.db} }

// Get returns the day-sheet's date and its entries in sheet order, each with its client
// populated - the raw material routes/Sheet.js groups by client.
func (sh *sheets) Get(ctx context.Context, companyID, sheetID store.ID) (string, []store.Entry, bool, error) {
	companyOID, err := objectID(companyID)
	if err != nil {
		return "", nil, false, err
	}
	sheetOID, err := objectID(sheetID)
	if err != nil {
		return "", nil, false, err
	}
	var doc struct {
		Date    string               `bson:"date"`
		Entries []primitive.ObjectID `bson:"entries"`
	}
	err = sh.db.Collection(colSheets).FindOne(ctx,
		bson.M{"_id": sheetOID, "company_id": companyOID}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return "", nil, false, nil
	}
	if err != nil {
		return "", nil, false, err
	}

	en := &entries{db: sh.db}
	out := make([]store.Entry, 0, len(doc.Entries))
	for _, id := range doc.Entries {
		var ed fullEntryDoc
		if err := sh.db.Collection(colEntries).FindOne(ctx, bson.M{"_id": id}).Decode(&ed); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				continue // an entry id that no longer resolves is skipped, as populate drops it
			}
			return "", nil, false, err
		}
		e := ed.toStore()
		if ed.ClientID != nil {
			e.Client = en.loadClient(ctx, *ed.ClientID)
		}
		out = append(out, e)
	}
	return doc.Date, out, true, nil
}

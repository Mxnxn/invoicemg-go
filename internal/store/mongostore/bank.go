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

func (b *banks) ReportData(ctx context.Context, companyID store.ID) (store.BankReportData, error) {
	var out store.BankReportData
	companyOID, err := objectID(companyID)
	if err != nil {
		return out, err
	}
	if out.BatchReceives, err = b.reportTxns(ctx, colBatchReceives, "note", companyOID); err != nil {
		return out, err
	}
	if out.SupplierPayments, err = b.reportTxns(ctx, colSupplierPayments, "note", companyOID); err != nil {
		return out, err
	}
	if out.Expenses, err = b.reportTxns(ctx, colExpenses, "notes", companyOID); err != nil {
		return out, err
	}
	cur, err := b.db.Collection(colBanks).Find(ctx, bson.M{"company_id": companyOID},
		options.Find().SetProjection(bson.M{"name": 1, "openingBalance": 1}))
	if err != nil {
		return out, fmt.Errorf("bank report banks: %w", err)
	}
	var bankDocs []struct {
		ID             primitive.ObjectID `bson:"_id"`
		Name           string             `bson:"name"`
		OpeningBalance float64            `bson:"openingBalance"`
	}
	if err := cur.All(ctx, &bankDocs); err != nil {
		return out, err
	}
	for _, d := range bankDocs {
		out.Banks = append(out.Banks, store.BankOpening{ID: idOf(d.ID), Name: d.Name, OpeningBalance: d.OpeningBalance})
	}
	return out, nil
}

// reportTxns reads one cash-moving collection's rows. A null bank_id becomes an empty id, which
// bankledger buckets as Unassigned.
func (b *banks) reportTxns(ctx context.Context, collection, noteField string, companyOID primitive.ObjectID) ([]store.BankTxn, error) {
	cur, err := b.db.Collection(collection).Find(ctx, bson.M{"company_id": companyOID},
		options.Find().SetProjection(bson.M{"bank_id": 1, "date": 1, "amount": 1, noteField: 1}))
	if err != nil {
		return nil, fmt.Errorf("bank report %s: %w", collection, err)
	}
	var docs []struct {
		BankID *primitive.ObjectID `bson:"bank_id"`
		Date   string              `bson:"date"`
		Amount float64             `bson:"amount"`
		Note   string              `bson:"note"`
		Notes  string              `bson:"notes"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("reading bank report %s: %w", collection, err)
	}
	out := make([]store.BankTxn, 0, len(docs))
	for _, d := range docs {
		t := store.BankTxn{Date: d.Date, Amount: d.Amount, Note: d.Note}
		if noteField == "notes" {
			t.Note = d.Notes
		}
		if d.BankID != nil {
			t.BankID = idOf(*d.BankID)
		}
		out = append(out, t)
	}
	return out, nil
}

func (b *banks) Create(ctx context.Context, companyID, uid store.ID, name string) (store.Bank, error) {
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.Bank{}, err
	}
	uidOID, err := objectID(uid)
	if err != nil {
		return store.Bank{}, err
	}
	now := time.Now().UTC()
	doc := bson.M{"uid": uidOID, "company_id": companyOID, "name": name, "openingBalance": 0, "createdAt": now, "updatedAt": now, "__v": 0}
	res, err := b.db.Collection(colBanks).InsertOne(ctx, doc)
	if err != nil {
		return store.Bank{}, fmt.Errorf("insert bank: %w", err)
	}
	out := store.Bank{UID: uid, CompanyID: companyID, Name: name, CreatedAt: now, UpdatedAt: now}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		out.ID = idOf(oid)
	}
	return out, nil
}

func (b *banks) Update(ctx context.Context, companyID, bankID store.ID, name string, openingBalance *float64) (store.Bank, bool, error) {
	bankOID, err := objectID(bankID)
	if err != nil {
		return store.Bank{}, false, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.Bank{}, false, err
	}
	set := bson.M{"name": name, "updatedAt": time.Now().UTC()}
	if openingBalance != nil {
		set["openingBalance"] = *openingBalance
	}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var doc bankDoc
	err = b.db.Collection(colBanks).
		FindOneAndUpdate(ctx, bson.M{"_id": bankOID, "company_id": companyOID}, bson.M{"$set": set}, opts).
		Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.Bank{}, false, nil
	}
	if err != nil {
		return store.Bank{}, false, fmt.Errorf("updating bank: %w", err)
	}
	return doc.toStore(), true, nil
}

func (b *banks) Remove(ctx context.Context, companyID, bankID store.ID) (int, bool, error) {
	bankOID, err := objectID(bankID)
	if err != nil {
		return 0, false, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return 0, false, err
	}
	// A bank referenced by a receipt, supplier payment or expense cannot be removed - deleting
	// it would leave those rows pointing at nothing and break the Bank Report arithmetic. The
	// scope is company-bound, so a bank_id belonging to another company counts zero here and
	// falls through to the delete, which then finds nothing (404), exactly as Node behaves.
	scope := bson.M{"bank_id": bankOID, "company_id": companyOID}
	var inUse int64
	for _, col := range []string{colBatchReceives, colSupplierPayments, colExpenses} {
		n, err := b.db.Collection(col).CountDocuments(ctx, scope)
		if err != nil {
			return 0, false, fmt.Errorf("counting bank references: %w", err)
		}
		inUse += n
	}
	if inUse > 0 {
		return int(inUse), false, nil
	}
	err = b.db.Collection(colBanks).
		FindOneAndDelete(ctx, bson.M{"_id": bankOID, "company_id": companyOID}).
		Err()
	if errors.Is(err, mongo.ErrNoDocuments) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("deleting bank: %w", err)
	}
	return 0, true, nil
}

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

func (w *wastages) Create(ctx context.Context, companyID, uid store.ID, in store.WastageWrite) (store.Wastage, error) {
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.Wastage{}, err
	}
	uidOID, err := objectID(uid)
	if err != nil {
		return store.Wastage{}, err
	}
	now := time.Now().UTC()
	doc := bson.M{
		"uid": uidOID, "company_id": companyOID, "material_name": in.MaterialName,
		"rate": in.Rate, "purchase_rate": in.PurchaseRate, "cost_total": in.CostTotal,
		"length": in.Length, "height": in.Height, "total": in.Total, "date": in.Date,
		"createdAt": now, "updatedAt": now, "__v": 0,
	}
	res, err := w.db.Collection(colWastages).InsertOne(ctx, doc)
	if err != nil {
		return store.Wastage{}, fmt.Errorf("insert wastage: %w", err)
	}
	out := store.Wastage{
		UID: uid, CompanyID: companyID, MaterialName: in.MaterialName, Rate: in.Rate,
		PurchaseRate: in.PurchaseRate, CostTotal: in.CostTotal, Length: in.Length, Height: in.Height,
		Total: in.Total, Date: in.Date, CreatedAt: now, UpdatedAt: now,
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		out.ID = idOf(oid)
	}
	return out, nil
}

func (w *wastages) MaterialsSummary(ctx context.Context, companyID store.ID) ([]store.MaterialAvg, error) {
	companyOID, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"company_id": companyOID}}},
		{{Key: "$group", Value: bson.M{
			"_id":           bson.M{"$toLower": "$material_name"},
			"material_name": bson.M{"$first": "$material_name"},
			"material_rate": bson.M{"$avg": "$material_rate"},
			"purchase_rate": bson.M{"$avg": "$purchase_rate"},
		}}},
		{{Key: "$project", Value: bson.M{"_id": 0, "material_name": 1, "material_rate": 1, "purchase_rate": 1}}},
		{{Key: "$sort", Value: bson.M{"material_name": 1}}},
	}
	cur, err := w.db.Collection(colMaterials).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("wastage materials summary: %w", err)
	}
	var docs []struct {
		MaterialName string  `bson:"material_name"`
		MaterialRate float64 `bson:"material_rate"`
		PurchaseRate float64 `bson:"purchase_rate"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]store.MaterialAvg, 0, len(docs))
	for _, d := range docs {
		out = append(out, store.MaterialAvg{MaterialName: d.MaterialName, MaterialRate: d.MaterialRate, PurchaseRate: d.PurchaseRate})
	}
	return out, nil
}

func (w *wastages) Delete(ctx context.Context, companyID, wastageID store.ID) (bool, error) {
	companyOID, err := objectID(companyID)
	if err != nil {
		return false, err
	}
	wastageOID, err := objectID(wastageID)
	if err != nil {
		return false, err // malformed id: Node's CastError -> 500, unlike /material/remove
	}
	err = w.db.Collection(colWastages).
		FindOneAndDelete(ctx, bson.M{"_id": wastageOID, "company_id": companyOID}).
		Err()
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("delete wastage: %w", err)
	}
	return true, nil
}

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

type inventory struct{ db *mongo.Database }

func (s *Store) Inventory() store.Inventory { return &inventory{db: s.db} }

func (i *inventory) Data(ctx context.Context, companyID store.ID, from, to string) (store.InventoryData, error) {
	var out store.InventoryData
	oid, err := objectID(companyID)
	if err != nil {
		return out, err
	}
	scope := bson.M{"company_id": oid}
	dated := bson.M{"company_id": oid}
	if dr := createdRange(from, to); dr != nil {
		dated["createdAt"] = dr
	}

	// materials (not date-filtered)
	mcur, err := i.db.Collection(colMaterials).Find(ctx, scope,
		options.Find().SetProjection(bson.M{"material_name": 1, "unit": 1, "purchase_rate": 1}))
	if err != nil {
		return out, fmt.Errorf("inventory materials: %w", err)
	}
	var mats []struct {
		ID           primitive.ObjectID `bson:"_id"`
		MaterialName string             `bson:"material_name"`
		Unit         string             `bson:"unit"`
		PurchaseRate float64            `bson:"purchase_rate"`
	}
	if err := mcur.All(ctx, &mats); err != nil {
		return out, err
	}
	for _, m := range mats {
		out.Materials = append(out.Materials, store.InvMaterial{ID: m.ID.Hex(), MaterialName: m.MaterialName, Unit: m.Unit, PurchaseRate: m.PurchaseRate})
	}

	// purchase invoice rows (date-filtered on parent)
	pcur, err := i.db.Collection(colPurchaseInvoices).Find(ctx, dated, options.Find().SetProjection(bson.M{"rows": 1, "company_id": 1}))
	if err != nil {
		return out, fmt.Errorf("inventory purchases: %w", err)
	}
	var purs []struct {
		CompanyID *primitive.ObjectID `bson:"company_id"`
		Rows      []struct {
			ID       primitive.ObjectID `bson:"_id"`
			Material string             `bson:"material"`
			Qty      float64            `bson:"qty"`
			Rate     float64            `bson:"rate"`
		} `bson:"rows"`
	}
	if err := pcur.All(ctx, &purs); err != nil {
		return out, err
	}
	for _, p := range purs {
		cid := hexOrEmpty(p.CompanyID)
		for _, r := range p.Rows {
			out.PurchaseRows = append(out.PurchaseRows, store.InvPurchaseRow{ID: r.ID.Hex(), Material: r.Material, Qty: r.Qty, Rate: r.Rate, CompanyID: cid})
		}
	}

	// job rows (date-filtered on parent)
	jcur, err := i.db.Collection(colJobs).Find(ctx, dated, options.Find().SetProjection(bson.M{"rows": 1, "company_id": 1}))
	if err != nil {
		return out, fmt.Errorf("inventory jobs: %w", err)
	}
	var jbs []struct {
		CompanyID *primitive.ObjectID `bson:"company_id"`
		Rows      []struct {
			ID            primitive.ObjectID `bson:"_id"`
			Material      string             `bson:"material"`
			Length        string             `bson:"length"`
			Width         string             `bson:"width"`
			Qty           float64            `bson:"qty"`
			HasDimensions *bool              `bson:"hasDimensions"`
		} `bson:"rows"`
	}
	if err := jcur.All(ctx, &jbs); err != nil {
		return out, err
	}
	for _, j := range jbs {
		cid := hexOrEmpty(j.CompanyID)
		for _, r := range j.Rows {
			out.JobRows = append(out.JobRows, store.InvJobRow{ID: r.ID.Hex(), Material: r.Material, Length: r.Length, Width: r.Width, Qty: r.Qty, HasDimensions: r.HasDimensions, CompanyID: cid})
		}
	}

	// wastage (not date-filtered)
	wcur, err := i.db.Collection(colWastages).Find(ctx, scope, options.Find().SetProjection(bson.M{"material_name": 1, "length": 1, "height": 1, "company_id": 1}))
	if err != nil {
		return out, fmt.Errorf("inventory wastage: %w", err)
	}
	var was []struct {
		ID           primitive.ObjectID  `bson:"_id"`
		MaterialName string              `bson:"material_name"`
		Length       float64             `bson:"length"`
		Height       float64             `bson:"height"`
		CompanyID    *primitive.ObjectID `bson:"company_id"`
	}
	if err := wcur.All(ctx, &was); err != nil {
		return out, err
	}
	for _, w := range was {
		out.WastageRows = append(out.WastageRows, store.InvWastageRow{ID: w.ID.Hex(), MaterialName: w.MaterialName, Length: w.Length, Height: w.Height, CompanyID: hexOrEmpty(w.CompanyID)})
	}
	return out, nil
}

func hexOrEmpty(id *primitive.ObjectID) string {
	if id == nil {
		return ""
	}
	return id.Hex()
}

// createdRange builds a Mongo range on createdAt from YYYY-MM-DD from/to (to is end-of-day).
func createdRange(from, to string) bson.M {
	dr := bson.M{}
	if from != "" {
		if t, err := time.Parse("2006-01-02", from); err == nil {
			dr["$gte"] = t.UTC()
		}
	}
	if to != "" {
		if t, err := time.Parse("2006-01-02", to); err == nil {
			dr["$lte"] = t.UTC().Add(24*time.Hour - time.Nanosecond)
		}
	}
	if len(dr) == 0 {
		return nil
	}
	return dr
}

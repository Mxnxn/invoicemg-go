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

const colMaterials = "materials"

type materials struct{ db *mongo.Database }

func (s *Store) Materials() store.Materials { return &materials{db: s.db} }

type priceHistoryDoc struct {
	MaterialRate float64   `bson:"material_rate"`
	PurchaseRate float64   `bson:"purchase_rate"`
	ChangedAt    time.Time `bson:"changed_at"`
}

// materialDoc mirrors Model/Material.js. getall returns the whole lean doc, so this decodes all
// of it, __v included. Sharing is a pointer to preserve nil (legacy) vs {companies:[]}.
type materialDoc struct {
	ID           primitive.ObjectID  `bson:"_id"`
	UID          primitive.ObjectID  `bson:"uid"`
	CompanyID    *primitive.ObjectID `bson:"company_id"`
	MaterialName string              `bson:"material_name"`
	MaterialRate float64             `bson:"material_rate"`
	PurchaseRate float64             `bson:"purchase_rate"`
	Unit         string              `bson:"unit"`
	Hsn          string              `bson:"hsn"`
	Tax          float64             `bson:"tax"`
	PriceHistory []priceHistoryDoc   `bson:"priceHistory"`
	Sharing      *struct {
		Companies []primitive.ObjectID `bson:"companies"`
	} `bson:"sharing"`
	CreatedAt time.Time `bson:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt"`
	Version   int       `bson:"__v"`
}

func (d materialDoc) toStore() store.Material {
	m := store.Material{
		ID:           idOf(d.ID),
		UID:          idOf(d.UID),
		MaterialName: d.MaterialName,
		MaterialRate: d.MaterialRate,
		PurchaseRate: d.PurchaseRate,
		Unit:         d.Unit,
		Hsn:          d.Hsn,
		Tax:          d.Tax,
		CreatedAt:    d.CreatedAt,
		UpdatedAt:    d.UpdatedAt,
		Version:      d.Version,
	}
	if d.CompanyID != nil {
		m.CompanyID = idOf(*d.CompanyID)
	}
	m.PriceHistory = make([]store.PriceHistoryEntry, 0, len(d.PriceHistory))
	for _, h := range d.PriceHistory {
		m.PriceHistory = append(m.PriceHistory, store.PriceHistoryEntry{
			MaterialRate: h.MaterialRate, PurchaseRate: h.PurchaseRate, ChangedAt: h.ChangedAt,
		})
	}
	if d.Sharing != nil {
		ids := make([]store.ID, 0, len(d.Sharing.Companies))
		for _, id := range d.Sharing.Companies {
			ids = append(ids, idOf(id))
		}
		m.Sharing = &ids
	}
	return m
}

func (m *materials) Visible(ctx context.Context, companyID store.ID) ([]store.Material, error) {
	oid, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	// visibleScope: own company OR shared with it. No uid clause here - matching routes/Material.js
	// exactly (unlike Client, which adds uid). No sort (natural order); sqlstore adds the id tiebreak.
	filter := bson.M{"$or": bson.A{
		bson.M{"company_id": oid},
		bson.M{"sharing.companies": oid},
	}}
	cur, err := m.db.Collection(colMaterials).Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("listing materials: %w", err)
	}
	defer cur.Close(ctx)

	var docs []materialDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("reading materials: %w", err)
	}
	out := make([]store.Material, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.toStore())
	}
	return out, nil
}

func (m *materials) Create(ctx context.Context, companyID, uid store.ID, in store.MaterialWrite) (store.Material, error) {
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.Material{}, err
	}
	uidOID, err := objectID(uid)
	if err != nil {
		return store.Material{}, err
	}
	now := time.Now().UTC()
	doc := bson.M{
		"uid": uidOID, "company_id": companyOID, "material_name": in.MaterialName,
		"material_rate": in.MaterialRate, "purchase_rate": in.PurchaseRate, "hsn": in.Hsn, "tax": in.Tax,
		"priceHistory": bson.A{}, "createdAt": now, "updatedAt": now, "__v": 0,
	}
	res, err := m.db.Collection(colMaterials).InsertOne(ctx, doc)
	if err != nil {
		return store.Material{}, fmt.Errorf("insert material: %w", err)
	}
	out := store.Material{
		UID: uid, CompanyID: companyID, MaterialName: in.MaterialName, MaterialRate: in.MaterialRate,
		PurchaseRate: in.PurchaseRate, Hsn: in.Hsn, Tax: in.Tax, CreatedAt: now, UpdatedAt: now,
		PriceHistory: make([]store.PriceHistoryEntry, 0),
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		out.ID = idOf(oid)
	}
	return out, nil
}

func (m *materials) Update(ctx context.Context, companyID, materialID store.ID, in store.MaterialWrite) (store.Material, bool, error) {
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.Material{}, false, err
	}
	materialOID, err := objectID(materialID)
	if err != nil {
		return store.Material{}, false, nil // malformed id is just "not found"
	}
	var existing materialDoc
	err = m.db.Collection(colMaterials).FindOne(ctx, bson.M{"_id": materialOID, "company_id": companyOID}).Decode(&existing)
	if err == mongo.ErrNoDocuments {
		return store.Material{}, false, nil
	}
	if err != nil {
		return store.Material{}, false, fmt.Errorf("read material: %w", err)
	}

	update := bson.M{"$set": bson.M{
		"material_name": in.MaterialName, "material_rate": in.MaterialRate,
		"purchase_rate": in.PurchaseRate, "hsn": in.Hsn, "tax": in.Tax,
	}}
	if existing.MaterialRate != in.MaterialRate || existing.PurchaseRate != in.PurchaseRate {
		update["$push"] = bson.M{"priceHistory": bson.M{
			"material_rate": existing.MaterialRate, "purchase_rate": existing.PurchaseRate, "changed_at": time.Now().UTC(),
		}}
	}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var updated materialDoc
	if err := m.db.Collection(colMaterials).FindOneAndUpdate(ctx,
		bson.M{"_id": materialOID, "company_id": companyOID}, update, opts).Decode(&updated); err != nil {
		return store.Material{}, false, fmt.Errorf("update material: %w", err)
	}
	return updated.toStore(), true, nil
}

func (m *materials) Delete(ctx context.Context, companyID, materialID store.ID) error {
	companyOID, err := objectID(companyID)
	if err != nil {
		return err
	}
	materialOID, err := objectID(materialID)
	if err != nil {
		return nil // malformed id: nothing to delete, and /remove answers 200 regardless
	}
	if _, err := m.db.Collection(colMaterials).DeleteOne(ctx, bson.M{"_id": materialOID, "company_id": companyOID}); err != nil {
		return fmt.Errorf("delete material: %w", err)
	}
	return nil
}

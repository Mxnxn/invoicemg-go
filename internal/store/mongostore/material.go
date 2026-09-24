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

func (m *materials) Get(ctx context.Context, companyID, materialID store.ID) (store.Material, bool, error) {
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.Material{}, false, err
	}
	materialOID, err := objectID(materialID)
	if err != nil {
		return store.Material{}, false, err
	}
	var doc materialDoc
	err = m.db.Collection(colMaterials).FindOne(ctx, bson.M{"_id": materialOID, "company_id": companyOID}).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return store.Material{}, false, nil
	}
	if err != nil {
		return store.Material{}, false, err
	}
	return doc.toStore(), true, nil
}

func (m *materials) SetUnit(ctx context.Context, companyID, materialID store.ID, unit string) (string, string, bool, bool, error) {
	companyOID, err := objectID(companyID)
	if err != nil {
		return "", "", false, false, err
	}
	matOID, err := objectID(materialID)
	if err != nil {
		return "", "", false, false, err
	}
	var updated struct {
		Name string `bson:"material_name"`
		Unit string `bson:"unit"`
	}
	opts := options.FindOneAndUpdate().
		SetReturnDocument(options.After).
		SetProjection(bson.M{"material_name": 1, "unit": 1})
	// ownedScope is company_id only - a borrowed product cannot have its unit set from here.
	err = m.db.Collection(colMaterials).
		FindOneAndUpdate(ctx, bson.M{"_id": matOID, "company_id": companyOID}, bson.M{"$set": bson.M{"unit": unit}}, opts).
		Decode(&updated)
	if err == nil {
		return updated.Name, updated.Unit, true, false, nil
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return "", "", false, false, fmt.Errorf("set unit: %w", err)
	}
	// Not owned. If the id exists at all it is shared in - the borrowed case worth naming.
	err = m.db.Collection(colMaterials).
		FindOne(ctx, bson.M{"_id": matOID}, options.FindOne().SetProjection(bson.M{"_id": 1})).
		Err()
	if err == nil {
		return "", "", false, true, nil
	}
	if errors.Is(err, mongo.ErrNoDocuments) {
		return "", "", false, false, nil
	}
	return "", "", false, false, fmt.Errorf("set unit borrow check: %w", err)
}

// sharingDoc is the projection of a record's sharing.companies list, shared by the sharing
// get/set reads. A nil pointer is a legacy record with no sharing block, read as empty.
type sharingDoc struct {
	Sharing *struct {
		Companies []primitive.ObjectID `bson:"companies"`
	} `bson:"sharing"`
}

// storeIDs turns a decoded sharingDoc's companies into store IDs (empty, never nil, on legacy).
func storeIDs(d sharingDoc) []store.ID {
	if d.Sharing == nil {
		return []store.ID{}
	}
	out := make([]store.ID, 0, len(d.Sharing.Companies))
	for _, oid := range d.Sharing.Companies {
		out = append(out, idOf(oid))
	}
	return out
}

func (m *materials) OwnedSharing(ctx context.Context, companyID, materialID store.ID) ([]store.ID, bool, error) {
	companyOID, err := objectID(companyID)
	if err != nil {
		return nil, false, err
	}
	matOID, err := objectID(materialID)
	if err != nil {
		return nil, false, err
	}
	var doc sharingDoc
	err = m.db.Collection(colMaterials).FindOne(ctx,
		bson.M{"_id": matOID, "company_id": companyOID},
		options.FindOne().SetProjection(bson.M{"sharing.companies": 1})).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("material owned sharing: %w", err)
	}
	return storeIDs(doc), true, nil
}

func (m *materials) SetSharing(ctx context.Context, companyID, materialID store.ID, companies []store.ID) ([]store.ID, bool, error) {
	companyOID, err := objectID(companyID)
	if err != nil {
		return nil, false, err
	}
	matOID, err := objectID(materialID)
	if err != nil {
		return nil, false, err
	}
	compOIDs, err := objectIDs(companies)
	if err != nil {
		return nil, false, err
	}
	if compOIDs == nil {
		compOIDs = []primitive.ObjectID{}
	}
	var updated sharingDoc
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After).SetProjection(bson.M{"sharing.companies": 1})
	err = m.db.Collection(colMaterials).FindOneAndUpdate(ctx,
		bson.M{"_id": matOID, "company_id": companyOID},
		bson.M{"$set": bson.M{"sharing.companies": compOIDs}}, opts).Decode(&updated)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("material set sharing: %w", err)
	}
	return storeIDs(updated), true, nil
}

func (m *materials) DuplicatesSource(ctx context.Context, companyIDs []store.ID) ([]store.MaterialDuplicate, error) {
	oids, err := objectIDs(companyIDs)
	if err != nil {
		return nil, err
	}
	proj := bson.M{"material_name": 1, "material_rate": 1, "purchase_rate": 1, "hsn": 1, "unit": 1, "company_id": 1, "sharing.companies": 1}
	cur, err := m.db.Collection(colMaterials).Find(ctx, bson.M{"company_id": bson.M{"$in": oids}},
		options.Find().SetProjection(proj).SetSort(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("duplicate materials source: %w", err)
	}
	defer cur.Close(ctx)
	var docs []struct {
		ID           primitive.ObjectID  `bson:"_id"`
		MaterialName string              `bson:"material_name"`
		MaterialRate float64             `bson:"material_rate"`
		PurchaseRate float64             `bson:"purchase_rate"`
		Hsn          string              `bson:"hsn"`
		Unit         string              `bson:"unit"`
		CompanyID    *primitive.ObjectID `bson:"company_id"`
		Sharing      *struct {
			Companies []primitive.ObjectID `bson:"companies"`
		} `bson:"sharing"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]store.MaterialDuplicate, 0, len(docs))
	for _, d := range docs {
		md := store.MaterialDuplicate{
			ID: idOf(d.ID), MaterialName: d.MaterialName, MaterialRate: d.MaterialRate,
			PurchaseRate: d.PurchaseRate, Hsn: d.Hsn, Unit: d.Unit,
		}
		if d.CompanyID != nil {
			md.CompanyID = idOf(*d.CompanyID)
		}
		if d.Sharing != nil {
			md.SharedWith = len(d.Sharing.Companies)
		}
		out = append(out, md)
	}
	return out, nil
}

func (m *materials) SharedList(ctx context.Context, companyIDs []store.ID) ([]store.SharedMaterial, error) {
	oids, err := objectIDs(companyIDs)
	if err != nil {
		return nil, err
	}
	proj := bson.M{"material_name": 1, "material_rate": 1, "purchase_rate": 1, "hsn": 1, "unit": 1, "company_id": 1}
	cur, err := m.db.Collection(colMaterials).Find(ctx, bson.M{"company_id": bson.M{"$in": oids}},
		options.Find().SetProjection(proj).SetSort(bson.D{{Key: "material_name", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("shared materials: %w", err)
	}
	defer cur.Close(ctx)
	var docs []struct {
		ID           primitive.ObjectID  `bson:"_id"`
		MaterialName string              `bson:"material_name"`
		MaterialRate float64             `bson:"material_rate"`
		PurchaseRate float64             `bson:"purchase_rate"`
		Hsn          string              `bson:"hsn"`
		Unit         string              `bson:"unit"`
		CompanyID    *primitive.ObjectID `bson:"company_id"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]store.SharedMaterial, 0, len(docs))
	for _, d := range docs {
		sm := store.SharedMaterial{ID: idOf(d.ID), MaterialName: d.MaterialName, MaterialRate: d.MaterialRate, PurchaseRate: d.PurchaseRate, Hsn: d.Hsn, Unit: d.Unit}
		if d.CompanyID != nil {
			sm.CompanyID = idOf(*d.CompanyID)
		}
		out = append(out, sm)
	}
	return out, nil
}

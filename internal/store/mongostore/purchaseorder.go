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

const (
	colPurchaseOrders   = "purchaseorders"
	colPOHistories      = "purchaseorderhistories"
	colPONotes          = "purchaseordernotes"
	colPODismissals     = "poapprovaldismissals"
)

type purchaseOrders struct{ db *mongo.Database }

func (s *Store) PurchaseOrders() store.PurchaseOrders { return &purchaseOrders{db: s.db} }

type poRowDoc struct {
	ID            primitive.ObjectID `bson:"_id"`
	Description   string             `bson:"description"`
	Material      string             `bson:"material"`
	Hsn           string             `bson:"hsn"`
	Gst           float64            `bson:"gst"`
	HasDimensions bool               `bson:"hasDimensions"`
	Length        string             `bson:"length"`
	Width         string             `bson:"width"`
	Rate          float64            `bson:"rate"`
	Qty           float64            `bson:"qty"`
	Unit          string             `bson:"unit"`
	Discount      float64            `bson:"discount"`
	Charges       float64            `bson:"charges"`
}

type poDoc struct {
	ID         primitive.ObjectID  `bson:"_id"`
	UID        primitive.ObjectID  `bson:"uid"`
	CompanyID  *primitive.ObjectID `bson:"company_id"`
	SupplierID *primitive.ObjectID `bson:"supplier_id"`
	PoNumber   string              `bson:"poNumber"`
	Date       string              `bson:"date"`
	Total      float64             `bson:"total"`
	Rows       []poRowDoc          `bson:"rows"`
	Approval   struct {
		State          string              `bson:"state"`
		ApprovedBy     *primitive.ObjectID `bson:"approvedBy"`
		ApprovedByName string              `bson:"approvedByName"`
		ApprovedAt     *time.Time          `bson:"approvedAt"`
		Fingerprint    string              `bson:"fingerprint"`
	} `bson:"approval"`
	PurchaseInvoiceID *primitive.ObjectID `bson:"purchaseInvoice_id"`
	ConvertedAt       *time.Time          `bson:"convertedAt"`
	Alerts            struct {
		Sent struct {
			Count       int        `bson:"count"`
			Fingerprint string     `bson:"fingerprint"`
			SentAt      *time.Time `bson:"sentAt"`
		} `bson:"sent"`
		Confirm struct {
			SentAt *time.Time `bson:"sentAt"`
		} `bson:"confirm"`
	} `bson:"alerts"`
	CreatedAt time.Time `bson:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt"`
}

func (p *purchaseOrders) actorName(ctx context.Context, actor store.NoteActor) string {
	if actor.Role == "admin" || actor.Role == "superadmin" {
		if oid, err := objectID(actor.UID); err == nil {
			var u struct {
				Name string `bson:"name"`
			}
			if p.db.Collection(colUsers).FindOne(ctx, bson.M{"_id": oid}).Decode(&u) == nil && u.Name != "" {
				return u.Name
			}
		}
		return "Admin"
	}
	if oid, err := objectID(actor.PersonID); err == nil {
		var per struct {
			Name string `bson:"name"`
		}
		if p.db.Collection(colPersons).FindOne(ctx, bson.M{"_id": oid}).Decode(&per) == nil && per.Name != "" {
			return per.Name
		}
	}
	return "Unknown"
}

// poActor returns the (type, id) for a history row, superadmin mapped to admin.
func poActor(actor store.NoteActor) (string, primitive.ObjectID) {
	role := actor.Role
	if role == "superadmin" {
		role = "admin"
	}
	var id primitive.ObjectID
	if role == "admin" {
		id, _ = objectID(actor.UID)
	} else {
		id, _ = objectID(actor.PersonID)
	}
	return role, id
}

func (p *purchaseOrders) logHistory(ctx context.Context, poID, uid, companyID primitive.ObjectID, actor store.NoteActor, name, action string, changes []store.Change, detail string) error {
	role, aid := poActor(actor)
	if changes == nil {
		changes = []store.Change{}
	}
	_, err := p.db.Collection(colPOHistories).InsertOne(ctx, bson.M{
		"po_id": poID, "uid": uid, "company_id": companyID, "actorType": role, "actorId": aid,
		"actorName": name, "action": action, "changes": changes, "detail": detail,
		"createdAt": time.Now().UTC(), "__v": 0,
	})
	return err
}

func poRowsToDocs(rows []store.PORow) []bson.M {
	out := make([]bson.M, 0, len(rows))
	now := time.Now().UTC()
	for _, r := range rows {
		length := r.Length
		if length == "" {
			length = "1"
		}
		width := r.Width
		if width == "" {
			width = "1"
		}
		qty := r.Qty
		if qty == 0 {
			qty = 1
		}
		out = append(out, bson.M{
			"_id": primitive.NewObjectID(), "description": r.Description, "material": r.Material,
			"hsn": r.Hsn, "gst": r.Gst, "hasDimensions": r.HasDimensions, "length": length, "width": width,
			"rate": r.Rate, "qty": qty, "unit": r.Unit, "discount": r.Discount, "charges": r.Charges,
			"createdAt": now, "updatedAt": now,
		})
	}
	return out
}

func (p *purchaseOrders) Numbers(ctx context.Context, uid, companyID store.ID) ([]string, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return nil, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	cur, err := p.db.Collection(colPurchaseOrders).Find(ctx, bson.M{"uid": uidOID, "company_id": companyOID},
		options.Find().SetProjection(bson.M{"poNumber": 1}))
	if err != nil {
		return nil, fmt.Errorf("po numbers: %w", err)
	}
	var docs []struct {
		N string `bson:"poNumber"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.N)
	}
	return out, nil
}

func (p *purchaseOrders) Create(ctx context.Context, uid, companyID store.ID, poNumber string, actor store.NoteActor, in store.POWrite) (store.PurchaseOrder, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return store.PurchaseOrder{}, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.PurchaseOrder{}, err
	}
	now := time.Now().UTC()
	doc := bson.M{
		"uid": uidOID, "company_id": companyOID, "supplier_id": optionalOID(in.SupplierID),
		"poNumber": poNumber, "date": in.Date, "rows": poRowsToDocs(in.Rows), "total": in.Total,
		"approval":            bson.M{"state": "draft", "approvedBy": nil, "approvedByName": "", "approvedAt": nil, "fingerprint": ""},
		"purchaseInvoice_id":  nil,
		"convertedAt":         nil,
		"alerts":              bson.M{"sent": bson.M{"count": 0, "fingerprint": "", "sentAt": nil}, "confirm": bson.M{"sentAt": nil}},
		"createdAt":           now,
		"updatedAt":           now,
		"__v":                 0,
	}
	res, err := p.db.Collection(colPurchaseOrders).InsertOne(ctx, doc)
	if err != nil {
		return store.PurchaseOrder{}, fmt.Errorf("insert po: %w", err)
	}
	poOID := res.InsertedID.(primitive.ObjectID)
	if err := p.logHistory(ctx, poOID, uidOID, companyOID, actor, p.actorName(ctx, actor), "Created", nil, poNumber); err != nil {
		return store.PurchaseOrder{}, fmt.Errorf("po create history: %w", err)
	}
	po, _, err := p.loadOne(ctx, uid, companyID, store.ID(poOID.Hex()))
	return po, err
}

func optionalOID(id store.ID) any {
	if id == "" {
		return nil
	}
	if oid, err := objectID(id); err == nil {
		return oid
	}
	return nil
}

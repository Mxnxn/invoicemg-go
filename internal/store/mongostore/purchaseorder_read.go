package mongostore

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

func (d poDoc) toStore(supplier *store.POSupplier) store.PurchaseOrder {
	po := store.PurchaseOrder{
		ID: idOf(d.ID), UID: idOf(d.UID), PoNumber: d.PoNumber, Date: d.Date, Total: d.Total,
		Supplier: supplier, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
	if d.CompanyID != nil {
		po.CompanyID = idOf(*d.CompanyID)
	}
	if d.SupplierID != nil {
		po.SupplierID = idOf(*d.SupplierID)
	}
	po.Approval = store.POApproval{State: d.Approval.State, ApprovedByName: d.Approval.ApprovedByName, ApprovedAt: d.Approval.ApprovedAt, Fingerprint: d.Approval.Fingerprint}
	if d.Approval.State == "" {
		po.Approval.State = "draft"
	}
	if d.Approval.ApprovedBy != nil {
		po.Approval.ApprovedBy = idOf(*d.Approval.ApprovedBy)
	}
	if d.PurchaseInvoiceID != nil {
		po.PurchaseInvoiceID = idOf(*d.PurchaseInvoiceID)
	}
	po.ConvertedAt = d.ConvertedAt
	po.Send = store.POSend{Count: d.Alerts.Sent.Count, Fingerprint: d.Alerts.Sent.Fingerprint, SentAt: d.Alerts.Sent.SentAt, ConfirmSentAt: d.Alerts.Confirm.SentAt}
	po.Rows = make([]store.PORow, 0, len(d.Rows))
	for _, r := range d.Rows {
		po.Rows = append(po.Rows, store.PORow{
			ID: idOf(r.ID), Description: r.Description, Material: r.Material, Hsn: r.Hsn, Gst: r.Gst,
			HasDimensions: r.HasDimensions, Length: r.Length, Width: r.Width, Rate: r.Rate, Qty: r.Qty,
			Unit: r.Unit, Discount: r.Discount, Charges: r.Charges,
		})
	}
	return po
}

func (p *purchaseOrders) suppliersFor(ctx context.Context, docs []poDoc) (map[primitive.ObjectID]*store.POSupplier, error) {
	ids := []primitive.ObjectID{}
	for _, d := range docs {
		if d.SupplierID != nil {
			ids = append(ids, *d.SupplierID)
		}
	}
	out := map[primitive.ObjectID]*store.POSupplier{}
	if len(ids) == 0 {
		return out, nil
	}
	cur, err := p.db.Collection(colPersons).Find(ctx, bson.M{"_id": bson.M{"$in": ids}},
		options.Find().SetProjection(bson.M{"name": 1, "firm": 1, "phone": 1, "address": 1, "gst": 1}))
	if err != nil {
		return nil, err
	}
	var people []struct {
		ID      primitive.ObjectID `bson:"_id"`
		Name    string             `bson:"name"`
		Firm    string             `bson:"firm"`
		Phone   string             `bson:"phone"`
		Address string             `bson:"address"`
		Gst     string             `bson:"gst"`
	}
	if err := cur.All(ctx, &people); err != nil {
		return nil, err
	}
	for _, pr := range people {
		out[pr.ID] = &store.POSupplier{ID: idOf(pr.ID), Name: pr.Name, Firm: pr.Firm, Phone: pr.Phone, Address: pr.Address, Gst: pr.Gst}
	}
	return out, nil
}

func (p *purchaseOrders) List(ctx context.Context, uid, companyID, supplierID store.ID) ([]store.PurchaseOrder, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return nil, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	filter := bson.M{"uid": uidOID, "company_id": companyOID}
	if supplierID != "" {
		if sOID, err := objectID(supplierID); err == nil {
			filter["supplier_id"] = sOID
		}
	}
	cur, err := p.db.Collection(colPurchaseOrders).Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, err
	}
	var docs []poDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	suppliers, err := p.suppliersFor(ctx, docs)
	if err != nil {
		return nil, err
	}
	out := make([]store.PurchaseOrder, 0, len(docs))
	for _, d := range docs {
		var sup *store.POSupplier
		if d.SupplierID != nil {
			sup = suppliers[*d.SupplierID]
		}
		out = append(out, d.toStore(sup))
	}
	return out, nil
}

func (p *purchaseOrders) loadOne(ctx context.Context, uid, companyID, poID store.ID) (store.PurchaseOrder, bool, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return store.PurchaseOrder{}, false, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.PurchaseOrder{}, false, err
	}
	poOID, err := objectID(poID)
	if err != nil {
		return store.PurchaseOrder{}, false, nil
	}
	var d poDoc
	err = p.db.Collection(colPurchaseOrders).FindOne(ctx, bson.M{"_id": poOID, "uid": uidOID, "company_id": companyOID}).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.PurchaseOrder{}, false, nil
	}
	if err != nil {
		return store.PurchaseOrder{}, false, err
	}
	suppliers, err := p.suppliersFor(ctx, []poDoc{d})
	if err != nil {
		return store.PurchaseOrder{}, false, err
	}
	var sup *store.POSupplier
	if d.SupplierID != nil {
		sup = suppliers[*d.SupplierID]
	}
	return d.toStore(sup), true, nil
}

func (p *purchaseOrders) Detail(ctx context.Context, uid, companyID, poID store.ID) (store.PurchaseOrder, []store.POHistoryRow, []store.PONote, bool, error) {
	po, found, err := p.loadOne(ctx, uid, companyID, poID)
	if err != nil || !found {
		return store.PurchaseOrder{}, nil, nil, found, err
	}
	poOID, _ := objectID(poID)
	companyOID, _ := objectID(companyID)

	history, err := p.historyFor(ctx, companyOID, poOID)
	if err != nil {
		return po, nil, nil, false, err
	}
	notes, err := p.notesFor(ctx, companyOID, poOID)
	if err != nil {
		return po, nil, nil, false, err
	}
	return po, history, notes, true, nil
}

func (p *purchaseOrders) historyFor(ctx context.Context, companyOID, poOID primitive.ObjectID) ([]store.POHistoryRow, error) {
	cur, err := p.db.Collection(colPOHistories).Find(ctx, bson.M{"po_id": poOID, "company_id": companyOID},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, err
	}
	var docs []struct {
		ID        primitive.ObjectID  `bson:"_id"`
		POID      primitive.ObjectID  `bson:"po_id"`
		ActorType string              `bson:"actorType"`
		ActorID   *primitive.ObjectID `bson:"actorId"`
		ActorName string              `bson:"actorName"`
		Action    string              `bson:"action"`
		Changes   []store.Change      `bson:"changes"`
		Detail    string              `bson:"detail"`
		CreatedAt time.Time           `bson:"createdAt"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := []store.POHistoryRow{}
	for _, d := range docs {
		h := store.POHistoryRow{ID: idOf(d.ID), POID: idOf(d.POID), ActorType: d.ActorType, ActorName: d.ActorName, Action: d.Action, Detail: d.Detail, CreatedAt: d.CreatedAt}
		if d.ActorID != nil {
			h.ActorID = idOf(*d.ActorID)
		}
		h.Changes = d.Changes
		if h.Changes == nil {
			h.Changes = []store.Change{}
		}
		out = append(out, h)
	}
	return out, nil
}

func (p *purchaseOrders) notesFor(ctx context.Context, companyOID, poOID primitive.ObjectID) ([]store.PONote, error) {
	cur, err := p.db.Collection(colPONotes).Find(ctx, bson.M{"po_id": poOID, "company_id": companyOID},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, err
	}
	var docs []struct {
		ID         primitive.ObjectID  `bson:"_id"`
		POID       primitive.ObjectID  `bson:"po_id"`
		AuthorType string              `bson:"authorType"`
		AuthorID   *primitive.ObjectID `bson:"authorId"`
		AuthorName string              `bson:"authorName"`
		Text       string              `bson:"text"`
		CreatedAt  time.Time           `bson:"createdAt"`
		UpdatedAt  time.Time           `bson:"updatedAt"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := []store.PONote{}
	for _, d := range docs {
		n := store.PONote{ID: idOf(d.ID), POID: idOf(d.POID), AuthorType: d.AuthorType, AuthorName: d.AuthorName, Text: d.Text, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt}
		if d.AuthorID != nil {
			n.AuthorID = idOf(*d.AuthorID)
		}
		out = append(out, n)
	}
	return out, nil
}

func (p *purchaseOrders) Update(ctx context.Context, uid, companyID, poID store.ID, actor store.NoteActor, in store.POUpdate) (store.PurchaseOrder, store.POUpdateResult, error) {
	var res store.POUpdateResult
	uidOID, err := objectID(uid)
	if err != nil {
		return store.PurchaseOrder{}, res, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.PurchaseOrder{}, res, err
	}
	poOID, err := objectID(poID)
	if err != nil {
		return store.PurchaseOrder{}, res, nil
	}

	var d poDoc
	err = p.db.Collection(colPurchaseOrders).FindOne(ctx, bson.M{"_id": poOID, "uid": uidOID, "company_id": companyOID}).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.PurchaseOrder{}, res, nil
	}
	if err != nil {
		return store.PurchaseOrder{}, res, err
	}
	res.Found = true
	if d.PurchaseInvoiceID != nil {
		res.Converted = true
		return store.PurchaseOrder{}, res, nil
	}

	set := bson.M{"total": in.Total, "updatedAt": time.Now().UTC()}
	if in.SetSupplier {
		set["supplier_id"] = optionalOID(in.SupplierID)
	}
	if in.SetDate {
		set["date"] = in.Date
	}
	if in.SetRows {
		set["rows"] = poRowsToDocs(in.Rows)
	}
	if d.Approval.State == "approved" && in.NewFingerprint != d.Approval.Fingerprint {
		set["approval.state"] = "draft"
		res.RevokedApproval = true
	}
	if _, err := p.db.Collection(colPurchaseOrders).UpdateOne(ctx, bson.M{"_id": poOID}, bson.M{"$set": set}); err != nil {
		return store.PurchaseOrder{}, res, err
	}

	name := p.actorName(ctx, actor)
	if len(in.Changes) > 0 {
		if err := p.logHistory(ctx, poOID, uidOID, companyOID, actor, name, "Updated", in.Changes, ""); err != nil {
			return store.PurchaseOrder{}, res, err
		}
	}
	if res.RevokedApproval {
		if err := p.logHistory(ctx, poOID, uidOID, companyOID, actor, name, "Approval revoked", in.Changes, "Price-bearing details changed after approval."); err != nil {
			return store.PurchaseOrder{}, res, err
		}
	}
	po, _, err := p.loadOne(ctx, uid, companyID, poID)
	return po, res, err
}

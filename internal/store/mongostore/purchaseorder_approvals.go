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

func (p *purchaseOrders) PendingApprovals(ctx context.Context, uid, companyID, actorID store.ID) ([]store.PurchaseOrder, []store.PurchaseOrder, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return nil, nil, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return nil, nil, err
	}
	cur, err := p.db.Collection(colPurchaseOrders).Find(ctx,
		bson.M{"uid": uidOID, "company_id": companyOID, "approval.state": "draft", "purchaseInvoice_id": nil},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, nil, fmt.Errorf("pending approvals: %w", err)
	}
	var docs []poDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, nil, err
	}
	suppliers, err := p.suppliersFor(ctx, docs)
	if err != nil {
		return nil, nil, err
	}
	pending := []store.PurchaseOrder{}
	pendingOIDs := []primitive.ObjectID{}
	for _, d := range docs {
		if len(d.Rows) == 0 { // isAwaitingApproval's rows clause
			continue
		}
		var sup *store.POSupplier
		if d.SupplierID != nil {
			sup = suppliers[*d.SupplierID]
		}
		pending = append(pending, d.toStore(sup))
		pendingOIDs = append(pendingOIDs, d.ID)
	}

	dismissed := map[store.ID]time.Time{}
	if len(pendingOIDs) > 0 {
		actorOID, err := objectID(actorID)
		if err == nil {
			dc, err := p.db.Collection(colPODismissals).Find(ctx, bson.M{"actorId": actorOID, "purchaseOrder_id": bson.M{"$in": pendingOIDs}})
			if err != nil {
				return nil, nil, fmt.Errorf("pending dismissals: %w", err)
			}
			var drows []struct {
				POID        primitive.ObjectID `bson:"purchaseOrder_id"`
				PoUpdatedAt time.Time          `bson:"poUpdatedAt"`
			}
			if err := dc.All(ctx, &drows); err != nil {
				return nil, nil, err
			}
			for _, d := range drows {
				dismissed[idOf(d.POID)] = d.PoUpdatedAt
			}
		}
	}

	visible := []store.PurchaseOrder{}
	for _, po := range pending {
		at, ok := dismissed[po.ID]
		if !ok || po.UpdatedAt.After(at) {
			visible = append(visible, po)
		}
	}
	return pending, visible, nil
}

func (p *purchaseOrders) DismissApprovals(ctx context.Context, uid, companyID, actorID store.ID, actorType string, all bool, poID store.ID) (int, int, int, error) {
	pending, visible, err := p.PendingApprovals(ctx, uid, companyID, actorID)
	if err != nil {
		return 0, 0, 0, err
	}
	actorOID, err := objectID(actorID)
	if err != nil {
		return 0, 0, 0, err
	}
	uidOID, _ := objectID(uid)
	companyOID, _ := objectID(companyID)

	pendingOIDs := make([]primitive.ObjectID, 0, len(pending))
	for _, po := range pending {
		if oid, err := objectID(po.ID); err == nil {
			pendingOIDs = append(pendingOIDs, oid)
		}
	}
	if _, err := p.db.Collection(colPODismissals).DeleteMany(ctx, bson.M{"actorId": actorOID, "purchaseOrder_id": bson.M{"$nin": pendingOIDs}}); err != nil {
		return 0, 0, 0, fmt.Errorf("prune dismissals: %w", err)
	}

	targets := []store.PurchaseOrder{}
	for _, po := range visible {
		if all || po.ID == poID {
			targets = append(targets, po)
		}
	}
	if len(targets) == 0 {
		return 0, len(visible), len(pending), nil
	}

	models := make([]mongo.WriteModel, 0, len(targets))
	for _, po := range targets {
		poOID, err := objectID(po.ID)
		if err != nil {
			continue
		}
		models = append(models, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"purchaseOrder_id": poOID, "actorId": actorOID}).
			SetUpdate(bson.M{"$set": bson.M{"uid": uidOID, "company_id": companyOID, "actorType": actorType, "poUpdatedAt": po.UpdatedAt}}).
			SetUpsert(true))
	}
	if len(models) > 0 {
		if _, err := p.db.Collection(colPODismissals).BulkWrite(ctx, models); err != nil {
			return 0, 0, 0, fmt.Errorf("upsert dismissals: %w", err)
		}
	}

	freshPending, freshVisible, err := p.PendingApprovals(ctx, uid, companyID, actorID)
	if err != nil {
		return 0, 0, 0, err
	}
	return len(targets), len(freshVisible), len(freshPending), nil
}

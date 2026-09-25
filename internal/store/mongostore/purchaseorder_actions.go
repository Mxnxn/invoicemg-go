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

func (p *purchaseOrders) Approve(ctx context.Context, uid, companyID, poID store.ID, actor store.NoteActor) (store.PurchaseOrder, store.POActionStatus, error) {
	po, found, err := p.loadOne(ctx, uid, companyID, poID)
	if err != nil {
		return store.PurchaseOrder{}, store.POActionNotFound, err
	}
	if !found {
		return store.PurchaseOrder{}, store.POActionNotFound, nil
	}
	if po.PurchaseInvoiceID != "" {
		return store.PurchaseOrder{}, store.POActionConverted, nil
	}
	if len(po.Rows) == 0 {
		return store.PurchaseOrder{}, store.POActionNoRows, nil
	}
	if po.Approval.State == "approved" {
		return store.PurchaseOrder{}, store.POActionAlreadyApproved, nil
	}
	poOID, _ := objectID(poID)
	uidOID, _ := objectID(uid)
	companyOID, _ := objectID(companyID)
	fp := store.POFingerprint(po)
	name := p.actorName(ctx, actor)
	_, aid := poActor(actor)

	set := bson.M{"approval.state": "approved", "approval.approvedBy": aid, "approval.approvedByName": name,
		"approval.approvedAt": time.Now().UTC(), "approval.fingerprint": fp, "updatedAt": time.Now().UTC()}
	if _, err := p.db.Collection(colPurchaseOrders).UpdateOne(ctx, bson.M{"_id": poOID}, bson.M{"$set": set}); err != nil {
		return store.PurchaseOrder{}, store.POActionNotFound, fmt.Errorf("po approve: %w", err)
	}
	if err := p.logHistory(ctx, poOID, uidOID, companyOID, actor, name, "Approved", nil, "Approved by "+name); err != nil {
		return store.PurchaseOrder{}, store.POActionNotFound, err
	}
	out, _, err := p.loadOne(ctx, uid, companyID, poID)
	return out, store.POActionOK, err
}

func (p *purchaseOrders) Revoke(ctx context.Context, uid, companyID, poID store.ID, actor store.NoteActor) (store.PurchaseOrder, store.POActionStatus, error) {
	po, found, err := p.loadOne(ctx, uid, companyID, poID)
	if err != nil {
		return store.PurchaseOrder{}, store.POActionNotFound, err
	}
	if !found {
		return store.PurchaseOrder{}, store.POActionNotFound, nil
	}
	if po.Approval.State != "approved" {
		return store.PurchaseOrder{}, store.POActionNotApproved, nil
	}
	poOID, _ := objectID(poID)
	uidOID, _ := objectID(uid)
	companyOID, _ := objectID(companyID)
	name := p.actorName(ctx, actor)
	if _, err := p.db.Collection(colPurchaseOrders).UpdateOne(ctx, bson.M{"_id": poOID},
		bson.M{"$set": bson.M{"approval.state": "draft", "updatedAt": time.Now().UTC()}}); err != nil {
		return store.PurchaseOrder{}, store.POActionNotFound, fmt.Errorf("po revoke: %w", err)
	}
	if err := p.logHistory(ctx, poOID, uidOID, companyOID, actor, name, "Approval revoked", nil, "Withdrawn by hand."); err != nil {
		return store.PurchaseOrder{}, store.POActionNotFound, err
	}
	out, _, err := p.loadOne(ctx, uid, companyID, poID)
	return out, store.POActionOK, err
}

func (p *purchaseOrders) AddNote(ctx context.Context, uid, companyID, poID store.ID, actor store.NoteActor, text string) (store.PONote, bool, error) {
	poOID, err := objectID(poID)
	if err != nil {
		return store.PONote{}, false, nil
	}
	uidOID, _ := objectID(uid)
	companyOID, _ := objectID(companyID)
	if err := p.db.Collection(colPurchaseOrders).FindOne(ctx, bson.M{"_id": poOID, "uid": uidOID, "company_id": companyOID},
		options.FindOne().SetProjection(bson.M{"_id": 1})).Err(); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return store.PONote{}, false, nil
		}
		return store.PONote{}, false, err
	}

	role, aid := poActor(actor)
	name := p.actorName(ctx, actor)
	now := time.Now().UTC()
	doc := bson.M{"po_id": poOID, "uid": uidOID, "company_id": companyOID, "authorType": role,
		"authorId": aid, "authorName": name, "text": text, "createdAt": now, "updatedAt": now, "__v": 0}
	res, err := p.db.Collection(colPONotes).InsertOne(ctx, doc)
	if err != nil {
		return store.PONote{}, false, fmt.Errorf("insert po note: %w", err)
	}
	if err := p.logHistory(ctx, poOID, uidOID, companyOID, actor, name, "Note added", nil, name); err != nil {
		return store.PONote{}, false, err
	}
	n := store.PONote{ID: idOf(res.InsertedID.(primitive.ObjectID)), POID: poID, AuthorType: role,
		AuthorID: idOf(aid), AuthorName: name, Text: text, CreatedAt: now, UpdatedAt: now}
	return n, true, nil
}

func (p *purchaseOrders) EditNote(ctx context.Context, companyID, noteID store.ID, actor store.NoteActor, text string) (store.PONote, bool, bool, error) {
	noteOID, err := objectID(noteID)
	if err != nil {
		return store.PONote{}, false, false, nil
	}
	companyOID, _ := objectID(companyID)
	var d struct {
		POID       primitive.ObjectID  `bson:"po_id"`
		AuthorType string              `bson:"authorType"`
		AuthorID   *primitive.ObjectID `bson:"authorId"`
		AuthorName string              `bson:"authorName"`
		CreatedAt  time.Time           `bson:"createdAt"`
	}
	err = p.db.Collection(colPONotes).FindOne(ctx, bson.M{"_id": noteOID, "company_id": companyOID}).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.PONote{}, false, false, nil
	}
	if err != nil {
		return store.PONote{}, false, false, fmt.Errorf("po note load: %w", err)
	}
	var aID store.ID
	if d.AuthorID != nil {
		aID = idOf(*d.AuthorID)
	}
	if !store.CanEditNote(aID, actor.ActorID(), d.CreatedAt, time.Now().UTC()) {
		return store.PONote{}, true, true, nil
	}
	now := time.Now().UTC()
	if _, err := p.db.Collection(colPONotes).UpdateOne(ctx, bson.M{"_id": noteOID},
		bson.M{"$set": bson.M{"text": text, "updatedAt": now}}); err != nil {
		return store.PONote{}, false, false, fmt.Errorf("po note edit: %w", err)
	}
	uidOID, _ := objectID(actor.UID)
	name := p.actorName(ctx, actor)
	if err := p.logHistory(ctx, d.POID, uidOID, companyOID, actor, name, "Note edited", nil, d.AuthorName); err != nil {
		return store.PONote{}, false, false, err
	}
	n := store.PONote{ID: noteID, POID: idOf(d.POID), AuthorType: d.AuthorType, AuthorID: aID,
		AuthorName: d.AuthorName, Text: text, CreatedAt: d.CreatedAt, UpdatedAt: now}
	return n, true, false, nil
}

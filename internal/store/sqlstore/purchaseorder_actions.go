package sqlstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

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

	fp := store.POFingerprint(po)
	at, aid, aname := poHistoryActor(ctx, p.pool, actor)

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return store.PurchaseOrder{}, store.POActionNotFound, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		UPDATE purchase_orders SET approval_state='approved', approved_by=$2, approved_by_name=$3,
		       approved_at=now(), approval_fingerprint=$4, updated_at=now()
		 WHERE id=$1`, string(poID), nullID(aid), aname, fp); err != nil {
		return store.PurchaseOrder{}, store.POActionNotFound, fmt.Errorf("po approve: %w", err)
	}
	if err := poLogHistory(ctx, tx, poID, uid, companyID, at, aid, aname, "Approved", nil, "Approved by "+aname); err != nil {
		return store.PurchaseOrder{}, store.POActionNotFound, err
	}
	if err := tx.Commit(ctx); err != nil {
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
	at, aid, aname := poHistoryActor(ctx, p.pool, actor)

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return store.PurchaseOrder{}, store.POActionNotFound, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE purchase_orders SET approval_state='draft', updated_at=now() WHERE id=$1`, string(poID)); err != nil {
		return store.PurchaseOrder{}, store.POActionNotFound, fmt.Errorf("po revoke: %w", err)
	}
	if err := poLogHistory(ctx, tx, poID, uid, companyID, at, aid, aname, "Approval revoked", nil, "Withdrawn by hand."); err != nil {
		return store.PurchaseOrder{}, store.POActionNotFound, err
	}
	if err := tx.Commit(ctx); err != nil {
		return store.PurchaseOrder{}, store.POActionNotFound, err
	}
	out, _, err := p.loadOne(ctx, uid, companyID, poID)
	return out, store.POActionOK, err
}

func (p *purchaseOrders) AddNote(ctx context.Context, uid, companyID, poID store.ID, actor store.NoteActor, text string) (store.PONote, bool, error) {
	var exists bool
	if err := p.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM purchase_orders WHERE id=$1 AND uid=$2 AND company_id=$3)`,
		string(poID), string(uid), string(companyID)).Scan(&exists); err != nil {
		return store.PONote{}, false, fmt.Errorf("po note lookup: %w", err)
	}
	if !exists {
		return store.PONote{}, false, nil
	}
	at, aid, aname := poHistoryActor(ctx, p.pool, actor)

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return store.PONote{}, false, err
	}
	defer tx.Rollback(ctx)
	var n store.PONote
	var noteID string
	err = tx.QueryRow(ctx, `
		INSERT INTO purchase_order_notes (po_id, uid, company_id, author_type, author_id, author_name, text)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id, created_at, updated_at`,
		string(poID), string(uid), string(companyID), at, nullID(aid), aname, text).Scan(&noteID, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		return store.PONote{}, false, fmt.Errorf("insert po note: %w", err)
	}
	if err := poLogHistory(ctx, tx, poID, uid, companyID, at, aid, aname, "Note added", nil, aname); err != nil {
		return store.PONote{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return store.PONote{}, false, err
	}
	n.ID = store.ID(noteID)
	n.POID = poID
	n.AuthorType, n.AuthorID, n.AuthorName, n.Text = at, aid, aname, text
	return n, true, nil
}

func (p *purchaseOrders) EditNote(ctx context.Context, companyID, noteID store.ID, actor store.NoteActor, text string) (store.PONote, bool, bool, error) {
	var n store.PONote
	var poID, authorType string
	var authorID *string
	var authorName string
	var createdAt time.Time
	err := p.pool.QueryRow(ctx, `SELECT po_id, author_type, author_id, author_name, created_at FROM purchase_order_notes WHERE id=$1 AND company_id=$2`,
		string(noteID), string(companyID)).Scan(&poID, &authorType, &authorID, &authorName, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.PONote{}, false, false, nil
	}
	if err != nil {
		return store.PONote{}, false, false, fmt.Errorf("po note load: %w", err)
	}
	var aID store.ID
	if authorID != nil {
		aID = store.ID(*authorID)
	}
	if !store.CanEditNote(aID, actor.ActorID(), createdAt, time.Now().UTC()) {
		return store.PONote{}, true, true, nil
	}

	at, aid, aname := poHistoryActor(ctx, p.pool, actor)
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return store.PONote{}, false, false, err
	}
	defer tx.Rollback(ctx)
	if err := tx.QueryRow(ctx, `UPDATE purchase_order_notes SET text=$2, updated_at=now() WHERE id=$1 RETURNING created_at, updated_at`,
		string(noteID), text).Scan(&n.CreatedAt, &n.UpdatedAt); err != nil {
		return store.PONote{}, false, false, fmt.Errorf("po note edit: %w", err)
	}
	if err := poLogHistory(ctx, tx, store.ID(poID), actor.UID, companyID, at, aid, aname, "Note edited", nil, authorName); err != nil {
		return store.PONote{}, false, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return store.PONote{}, false, false, err
	}
	n.ID = noteID
	n.POID = store.ID(poID)
	n.AuthorType, n.AuthorID, n.AuthorName, n.Text = authorType, aID, authorName, text
	return n, true, false, nil
}

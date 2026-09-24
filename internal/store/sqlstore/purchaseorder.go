package sqlstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type purchaseOrders struct{ pool *pgxpool.Pool }

func (s *Store) PurchaseOrders() store.PurchaseOrders { return &purchaseOrders{pool: s.pool} }

// poHistoryActor normalises the session actor to (type, id, name) for a history row, mapping
// superadmin to admin (Node's historyActor) so it fits the audit vocabulary.
func poHistoryActor(ctx context.Context, pool *pgxpool.Pool, actor store.NoteActor) (string, store.ID, string) {
	role := actor.Role
	if role == "superadmin" {
		role = "admin"
	}
	na := store.NoteActor{Role: role, UID: actor.UID, PersonID: actor.PersonID}
	name := resolveActorName(ctx, pool, na)
	if role == "admin" {
		return "admin", actor.UID, name
	}
	return role, actor.PersonID, name
}

func poLogHistory(ctx context.Context, q pgx.Tx, poID, uid, companyID store.ID, actorType string, actorID store.ID, actorName, action string, changes []store.Change, detail string) error {
	if changes == nil {
		changes = []store.Change{}
	}
	blob, err := json.Marshal(changes)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `
		INSERT INTO purchase_order_history (po_id, uid, company_id, actor_type, actor_id, actor_name, action, changes, detail)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		string(poID), string(uid), string(companyID), actorType, nullID(actorID), actorName, action, json.RawMessage(blob), detail)
	return err
}

func nullID(id store.ID) any {
	if id == "" {
		return nil
	}
	return string(id)
}

func (p *purchaseOrders) Numbers(ctx context.Context, uid, companyID store.ID) ([]string, error) {
	rows, err := p.pool.Query(ctx, `SELECT po_number FROM purchase_orders WHERE uid=$1 AND company_id=$2`, string(uid), string(companyID))
	if err != nil {
		return nil, fmt.Errorf("po numbers: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (p *purchaseOrders) Create(ctx context.Context, uid, companyID store.ID, poNumber string, actor store.NoteActor, in store.POWrite) (store.PurchaseOrder, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return store.PurchaseOrder{}, err
	}
	defer tx.Rollback(ctx)

	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO purchase_orders (uid, company_id, supplier_id, po_number, date, total)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		string(uid), string(companyID), nullID(in.SupplierID), poNumber, in.Date, in.Total).Scan(&id)
	if err != nil {
		return store.PurchaseOrder{}, fmt.Errorf("insert po: %w", err)
	}
	if err := insertPORows(ctx, tx, store.ID(id), in.Rows); err != nil {
		return store.PurchaseOrder{}, err
	}
	at, aid, aname := poHistoryActor(ctx, p.pool, actor)
	if err := poLogHistory(ctx, tx, store.ID(id), uid, companyID, at, aid, aname, "Created", nil, poNumber); err != nil {
		return store.PurchaseOrder{}, fmt.Errorf("po create history: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return store.PurchaseOrder{}, err
	}
	po, _, err := p.loadOne(ctx, uid, companyID, store.ID(id))
	return po, err
}

func insertPORows(ctx context.Context, q pgx.Tx, poID store.ID, rows []store.PORow) error {
	for i, r := range rows {
		if _, err := q.Exec(ctx, `
			INSERT INTO purchase_order_rows (po_id, position, description, material, hsn, gst, has_dimensions, length, width, rate, qty, unit, discount, charges)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
			string(poID), i, r.Description, r.Material, r.Hsn, r.Gst, r.HasDimensions, defaultStr(r.Length, "1"), defaultStr(r.Width, "1"),
			r.Rate, defaultQty(r.Qty), r.Unit, r.Discount, r.Charges); err != nil {
			return fmt.Errorf("insert po row: %w", err)
		}
	}
	return nil
}

func defaultStr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
func defaultQty(q float64) float64 {
	if q == 0 {
		return 1
	}
	return q
}

func (p *purchaseOrders) List(ctx context.Context, uid, companyID, supplierID store.ID) ([]store.PurchaseOrder, error) {
	sql := `
		SELECT po.id, po.uid, po.company_id, po.supplier_id, po.po_number, po.date, po.total,
		       po.approval_state, po.approved_by, po.approved_by_name, po.approved_at, po.approval_fingerprint,
		       po.purchase_invoice_id, po.converted_at, po.sent_count, po.sent_fingerprint, po.sent_at, po.confirm_sent_at,
		       po.created_at, po.updated_at,
		       s.name, s.firm, s.phone, s.address, s.gst
		  FROM purchase_orders po
		  LEFT JOIN persons s ON s.id = po.supplier_id
		 WHERE po.uid = $1 AND po.company_id = $2`
	args := []any{string(uid), string(companyID)}
	if supplierID != "" {
		sql += ` AND po.supplier_id = $3`
		args = append(args, string(supplierID))
	}
	sql += ` ORDER BY po.created_at DESC`
	rows, err := p.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("po list: %w", err)
	}
	defer rows.Close()
	out := []store.PurchaseOrder{}
	byID := map[store.ID]int{}
	ids := []string{}
	for rows.Next() {
		po, err := scanPO(rows)
		if err != nil {
			return nil, err
		}
		byID[po.ID] = len(out)
		out = append(out, po)
		ids = append(ids, string(po.ID))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return out, nil
	}
	rowsByPO, err := p.rowsFor(ctx, ids)
	if err != nil {
		return nil, err
	}
	for id, idx := range byID {
		out[idx].Rows = rowsByPO[id]
	}
	return out, nil
}

func (p *purchaseOrders) Detail(ctx context.Context, uid, companyID, poID store.ID) (store.PurchaseOrder, []store.POHistoryRow, []store.PONote, bool, error) {
	po, found, err := p.loadOne(ctx, uid, companyID, poID)
	if err != nil || !found {
		return store.PurchaseOrder{}, nil, nil, found, err
	}
	history, err := p.historyFor(ctx, companyID, poID)
	if err != nil {
		return po, nil, nil, false, err
	}
	notes, err := p.notesFor(ctx, companyID, poID)
	if err != nil {
		return po, nil, nil, false, err
	}
	return po, history, notes, true, nil
}

func (p *purchaseOrders) Update(ctx context.Context, uid, companyID, poID store.ID, actor store.NoteActor, in store.POUpdate) (store.PurchaseOrder, store.POUpdateResult, error) {
	var res store.POUpdateResult
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return store.PurchaseOrder{}, res, err
	}
	defer tx.Rollback(ctx)

	var invoiceID *string
	var approvalState, approvalFP string
	err = tx.QueryRow(ctx, `SELECT purchase_invoice_id, approval_state, approval_fingerprint FROM purchase_orders WHERE id=$1 AND uid=$2 AND company_id=$3 FOR UPDATE`,
		string(poID), string(uid), string(companyID)).Scan(&invoiceID, &approvalState, &approvalFP)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.PurchaseOrder{}, res, nil // found=false
	}
	if err != nil {
		return store.PurchaseOrder{}, res, fmt.Errorf("po update load: %w", err)
	}
	res.Found = true
	if invoiceID != nil {
		res.Converted = true
		return store.PurchaseOrder{}, res, nil // 409
	}

	set := "total=$2, updated_at=now()"
	args := []any{string(poID), in.Total}
	n := 3
	if in.SetSupplier {
		set += fmt.Sprintf(", supplier_id=$%d", n)
		args = append(args, nullID(in.SupplierID))
		n++
	}
	if in.SetDate {
		set += fmt.Sprintf(", date=$%d", n)
		args = append(args, in.Date)
		n++
	}

	wasApproved := approvalState == "approved"
	stillMatches := in.NewFingerprint == approvalFP
	if wasApproved && !stillMatches {
		set += ", approval_state='draft'"
		res.RevokedApproval = true
	}

	if _, err := tx.Exec(ctx, `UPDATE purchase_orders SET `+set+` WHERE id=$1`, args...); err != nil {
		return store.PurchaseOrder{}, res, fmt.Errorf("po update: %w", err)
	}
	if in.SetRows {
		if _, err := tx.Exec(ctx, `DELETE FROM purchase_order_rows WHERE po_id=$1`, string(poID)); err != nil {
			return store.PurchaseOrder{}, res, fmt.Errorf("po update clear rows: %w", err)
		}
		if err := insertPORows(ctx, tx, poID, in.Rows); err != nil {
			return store.PurchaseOrder{}, res, err
		}
	}

	at, aid, aname := poHistoryActor(ctx, p.pool, actor)
	if len(in.Changes) > 0 {
		if err := poLogHistory(ctx, tx, poID, uid, companyID, at, aid, aname, "Updated", in.Changes, ""); err != nil {
			return store.PurchaseOrder{}, res, err
		}
	}
	if res.RevokedApproval {
		if err := poLogHistory(ctx, tx, poID, uid, companyID, at, aid, aname, "Approval revoked", in.Changes, "Price-bearing details changed after approval."); err != nil {
			return store.PurchaseOrder{}, res, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return store.PurchaseOrder{}, res, err
	}
	po, _, err := p.loadOne(ctx, uid, companyID, poID)
	return po, res, err
}

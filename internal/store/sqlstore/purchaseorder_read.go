package sqlstore

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type rowScanner interface{ Scan(dest ...any) error }

const poSelectCols = `
	po.id, po.uid, po.company_id, po.supplier_id, po.po_number, po.date, po.total,
	po.approval_state, po.approved_by, po.approved_by_name, po.approved_at, po.approval_fingerprint,
	po.purchase_invoice_id, po.converted_at, po.sent_count, po.sent_fingerprint, po.sent_at, po.confirm_sent_at,
	po.created_at, po.updated_at,
	s.name, s.firm, s.phone, s.address, s.gst`

func scanPO(row rowScanner) (store.PurchaseOrder, error) {
	var po store.PurchaseOrder
	var uid, companyID string
	var supplierID, approvedBy, invoiceID *string
	var approvedAt, convertedAt, sentAt, confirmSentAt *time.Time
	var sName, sFirm, sPhone, sAddress, sGst *string
	err := row.Scan(&po.ID, &uid, &companyID, &supplierID, &po.PoNumber, &po.Date, &po.Total,
		&po.Approval.State, &approvedBy, &po.Approval.ApprovedByName, &approvedAt, &po.Approval.Fingerprint,
		&invoiceID, &convertedAt, &po.Send.Count, &po.Send.Fingerprint, &sentAt, &confirmSentAt,
		&po.CreatedAt, &po.UpdatedAt,
		&sName, &sFirm, &sPhone, &sAddress, &sGst)
	if err != nil {
		return store.PurchaseOrder{}, err
	}
	po.UID, po.CompanyID = store.ID(uid), store.ID(companyID)
	if supplierID != nil {
		po.SupplierID = store.ID(*supplierID)
	}
	if approvedBy != nil {
		po.Approval.ApprovedBy = store.ID(*approvedBy)
	}
	po.Approval.ApprovedAt = approvedAt
	if invoiceID != nil {
		po.PurchaseInvoiceID = store.ID(*invoiceID)
	}
	po.ConvertedAt = convertedAt
	po.Send.SentAt = sentAt
	po.Send.ConfirmSentAt = confirmSentAt
	// Supplier populated only when the join matched a persons row.
	if sName != nil || sFirm != nil {
		po.Supplier = &store.POSupplier{
			ID: po.SupplierID, Name: deref(sName), Firm: deref(sFirm),
			Phone: deref(sPhone), Address: deref(sAddress), Gst: deref(sGst),
		}
	}
	po.Rows = []store.PORow{}
	return po, nil
}

func (p *purchaseOrders) loadOne(ctx context.Context, uid, companyID, poID store.ID) (store.PurchaseOrder, bool, error) {
	row := p.pool.QueryRow(ctx, `SELECT `+poSelectCols+`
		  FROM purchase_orders po LEFT JOIN persons s ON s.id = po.supplier_id
		 WHERE po.id = $1 AND po.uid = $2 AND po.company_id = $3`,
		string(poID), string(uid), string(companyID))
	po, err := scanPO(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.PurchaseOrder{}, false, nil
	}
	if err != nil {
		return store.PurchaseOrder{}, false, err
	}
	byPO, err := p.rowsFor(ctx, []string{string(poID)})
	if err != nil {
		return store.PurchaseOrder{}, false, err
	}
	po.Rows = byPO[poID]
	if po.Rows == nil {
		po.Rows = []store.PORow{}
	}
	return po, true, nil
}

func (p *purchaseOrders) rowsFor(ctx context.Context, poIDs []string) (map[store.ID][]store.PORow, error) {
	out := map[store.ID][]store.PORow{}
	rows, err := p.pool.Query(ctx, `
		SELECT po_id, id, description, material, hsn, gst, has_dimensions, length, width, rate, qty, unit, discount, charges
		  FROM purchase_order_rows WHERE po_id = ANY($1) ORDER BY position ASC, id ASC`, poIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var poID string
		var r store.PORow
		if err := rows.Scan(&poID, &r.ID, &r.Description, &r.Material, &r.Hsn, &r.Gst, &r.HasDimensions,
			&r.Length, &r.Width, &r.Rate, &r.Qty, &r.Unit, &r.Discount, &r.Charges); err != nil {
			return nil, err
		}
		out[store.ID(poID)] = append(out[store.ID(poID)], r)
	}
	return out, rows.Err()
}

func (p *purchaseOrders) historyFor(ctx context.Context, companyID, poID store.ID) ([]store.POHistoryRow, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, po_id, actor_type, actor_id, actor_name, action, changes, detail, created_at
		  FROM purchase_order_history WHERE po_id=$1 AND company_id=$2 ORDER BY created_at DESC`,
		string(poID), string(companyID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []store.POHistoryRow{}
	for rows.Next() {
		var h store.POHistoryRow
		var poIDCol string
		var actorIDPtr *string
		var changesRaw []byte
		if err := rows.Scan(&h.ID, &poIDCol, &h.ActorType, &actorIDPtr, &h.ActorName, &h.Action, &changesRaw, &h.Detail, &h.CreatedAt); err != nil {
			return nil, err
		}
		h.POID = store.ID(poIDCol)
		if actorIDPtr != nil {
			h.ActorID = store.ID(*actorIDPtr)
		}
		h.Changes = []store.Change{}
		if len(changesRaw) > 0 {
			_ = json.Unmarshal(changesRaw, &h.Changes)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (p *purchaseOrders) notesFor(ctx context.Context, companyID, poID store.ID) ([]store.PONote, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, po_id, author_type, author_id, author_name, text, created_at, updated_at
		  FROM purchase_order_notes WHERE po_id=$1 AND company_id=$2 ORDER BY created_at DESC`,
		string(poID), string(companyID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []store.PONote{}
	for rows.Next() {
		var nrow store.PONote
		var poIDCol string
		var authorID *string
		if err := rows.Scan(&nrow.ID, &poIDCol, &nrow.AuthorType, &authorID, &nrow.AuthorName, &nrow.Text, &nrow.CreatedAt, &nrow.UpdatedAt); err != nil {
			return nil, err
		}
		nrow.POID = store.ID(poIDCol)
		if authorID != nil {
			nrow.AuthorID = store.ID(*authorID)
		}
		out = append(out, nrow)
	}
	return out, rows.Err()
}

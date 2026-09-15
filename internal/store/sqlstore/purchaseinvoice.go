package sqlstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type purchaseInvoices struct{ pool *pgxpool.Pool }

func (s *Store) PurchaseInvoices() store.PurchaseInvoices { return &purchaseInvoices{pool: s.pool} }

func (p *purchaseInvoices) List(ctx context.Context, uid, companyID store.ID) ([]store.PurchaseInvoice, error) {
	// populate supplier_id via LEFT JOIN on persons (#6); a dangling supplier yields NULLs -> nil.
	rows, err := p.pool.Query(ctx, `
		SELECT pi.id, pi.uid, pi.company_id, pi.date, pi.invoice_number, pi.total, pi.amount,
		       pi.created_at, pi.updated_at,
		       s.id, s.name, s.firm, s.phone
		  FROM purchase_invoices pi
		  LEFT JOIN persons s ON s.id = pi.supplier_id
		 WHERE pi.uid = $1 AND pi.company_id = $2
		 ORDER BY pi.created_at DESC, pi.id DESC`, string(uid), string(companyID))
	if err != nil {
		return nil, fmt.Errorf("listing purchase invoices: %w", err)
	}
	defer rows.Close()

	out := make([]store.PurchaseInvoice, 0)
	ids := make([]string, 0)
	byID := map[string]int{}
	for rows.Next() {
		var inv store.PurchaseInvoice
		var companyIDCol *string
		var sID, sName, sFirm, sPhone *string
		if err := rows.Scan(&inv.ID, &inv.UID, &companyIDCol, &inv.Date, &inv.InvoiceNumber,
			&inv.Total, &inv.Amount, &inv.CreatedAt, &inv.UpdatedAt,
			&sID, &sName, &sFirm, &sPhone); err != nil {
			return nil, fmt.Errorf("reading purchase invoices: %w", err)
		}
		if companyIDCol != nil {
			inv.CompanyID = store.ID(*companyIDCol)
		}
		if sID != nil {
			inv.Supplier = &store.PurchaseSupplier{
				ID: store.ID(*sID), Name: deref(sName), Firm: deref(sFirm), Phone: deref(sPhone),
			}
		}
		inv.Rows = make([]store.PurchaseInvoiceRow, 0)
		byID[string(inv.ID)] = len(out)
		ids = append(ids, string(inv.ID))
		out = append(out, inv)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return out, nil
	}

	// #6: one batched query for every invoice's rows.
	rRows, err := p.pool.Query(ctx, `
		SELECT invoice_id, id, description, material, hsn, gst, has_dimensions, length, width,
		       rate, qty, unit, discount, charges, created_at, updated_at
		  FROM purchase_invoice_rows
		 WHERE invoice_id = ANY ($1)
		 ORDER BY position ASC, id ASC`, ids)
	if err != nil {
		return nil, fmt.Errorf("listing purchase invoice rows: %w", err)
	}
	defer rRows.Close()
	for rRows.Next() {
		var iid string
		var r store.PurchaseInvoiceRow
		var hasDim bool
		if err := rRows.Scan(&iid, &r.ID, &r.Description, &r.Material, &r.Hsn, &r.Gst, &hasDim,
			&r.Length, &r.Width, &r.Rate, &r.Qty, &r.Unit, &r.Discount, &r.Charges,
			&r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("reading purchase invoice rows: %w", err)
		}
		hd := hasDim
		r.HasDimensions = &hd
		if i, ok := byID[iid]; ok {
			out[i].Rows = append(out[i].Rows, r)
		}
	}
	return out, rRows.Err()
}

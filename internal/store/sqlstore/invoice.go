package sqlstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type invoices struct{ pool *pgxpool.Pool }

func (s *Store) Invoices() store.Invoices { return &invoices{pool: s.pool} }

func (i *invoices) List(ctx context.Context, companyID store.ID) ([]store.Invoice, error) {
	// Client populated via LEFT JOIN (#6); a dangling client_id yields NULLs -> nil client.
	rows, err := i.pool.Query(ctx, `
		SELECT iv.id, iv.invoice_id, iv.date, iv.amount, iv.total_amount, iv.created_at,
		       c.id, c.uid, c.client_name, c.client_firm, c.client_phone, c.client_address, c.client_gst
		  FROM invoices iv
		  LEFT JOIN clients c ON c.id = iv.client_id
		 WHERE iv.company_id = $1
		 ORDER BY iv.created_at DESC, iv.id DESC`, string(companyID))
	if err != nil {
		return nil, fmt.Errorf("listing invoices: %w", err)
	}
	defer rows.Close()

	out := make([]store.Invoice, 0)
	ids := make([]string, 0)
	byID := map[string]int{}
	for rows.Next() {
		var inv store.Invoice
		var cID, cUID, cName, cFirm, cPhone, cAddr, cGST *string
		if err := rows.Scan(&inv.ID, &inv.InvoiceID, &inv.Date, &inv.Amount, &inv.TotalAmount, &inv.CreatedAt,
			&cID, &cUID, &cName, &cFirm, &cPhone, &cAddr, &cGST); err != nil {
			return nil, fmt.Errorf("reading invoices: %w", err)
		}
		if cID != nil {
			inv.Client = &store.InvoiceClient{
				ID: store.ID(*cID), UID: store.ID(deref(cUID)), ClientName: deref(cName),
				ClientFirm: deref(cFirm), ClientPhone: deref(cPhone), ClientAddress: deref(cAddr), ClientGST: deref(cGST),
			}
		}
		inv.Entries = make([]store.Entry, 0)
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

	// #6: one batched query for every invoice's entries, ordered by creation to reproduce the
	// invoice.entries array order.
	eRows, err := i.pool.Query(ctx, `
		SELECT invoice_id, id, description, material, hsn, rate, qty, has_dimensions, length, width,
		       date, amount, cgst, sgst, igst, discount, charges, advance, total, created_at, updated_at
		  FROM entries
		 WHERE invoice_id = ANY ($1)
		 ORDER BY created_at ASC, id ASC`, ids)
	if err != nil {
		return nil, fmt.Errorf("listing entries: %w", err)
	}
	defer eRows.Close()
	for eRows.Next() {
		var iid string
		var e store.Entry
		var hasDim bool
		if err := eRows.Scan(&iid, &e.ID, &e.Description, &e.Material, &e.Hsn, &e.Rate, &e.Qty, &hasDim,
			&e.Length, &e.Width, &e.Date, &e.Amount, &e.Cgst, &e.Sgst, &e.Igst, &e.Discount, &e.Charges,
			&e.Advance, &e.Total, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("reading entries: %w", err)
		}
		hd := hasDim
		e.HasDimensions = &hd
		if idx, ok := byID[iid]; ok {
			out[idx].Entries = append(out[idx].Entries, e)
		}
	}
	return out, eRows.Err()
}

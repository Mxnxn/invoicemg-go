package sqlstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type quotations struct{ pool *pgxpool.Pool }

func (s *Store) Quotations() store.Quotations { return &quotations{pool: s.pool} }

func (q *quotations) List(ctx context.Context, uid, companyID, clientID store.ID) ([]store.Quotation, error) {
	// The populate is a LEFT JOIN here - the client's fields come back in the same statement
	// (#6), and a dangling client_id yields NULLs, which map to a nil Client (populate's null).
	rows, err := q.pool.Query(ctx, `
		SELECT qt.id, qt.uid, qt.company_id, qt.quotation_number, qt.date,
		       qt.created_at, qt.updated_at,
		       c.id, c.client_name, c.client_firm, c.client_phone, c.client_address, c.client_gst
		  FROM quotations qt
		  LEFT JOIN clients c ON c.id = qt.client_id
		 WHERE qt.uid = $1 AND qt.company_id = $2 AND ($3 = '' OR qt.client_id = $3)
		 ORDER BY qt.created_at DESC, qt.id DESC`, string(uid), string(companyID), string(clientID))
	if err != nil {
		return nil, fmt.Errorf("listing quotations: %w", err)
	}
	defer rows.Close()

	out := make([]store.Quotation, 0)
	ids := make([]string, 0)
	byID := map[string]int{}
	for rows.Next() {
		var qt store.Quotation
		var companyIDCol *string
		var cID, cName, cFirm, cPhone, cAddr, cGST *string
		if err := rows.Scan(&qt.ID, &qt.UID, &companyIDCol, &qt.QuotationNumber, &qt.Date,
			&qt.CreatedAt, &qt.UpdatedAt,
			&cID, &cName, &cFirm, &cPhone, &cAddr, &cGST); err != nil {
			return nil, fmt.Errorf("reading quotations: %w", err)
		}
		if companyIDCol != nil {
			qt.CompanyID = store.ID(*companyIDCol)
		}
		if cID != nil { // the LEFT JOIN matched a client
			qt.Client = &store.QuotationClient{
				ID: store.ID(*cID), ClientName: deref(cName), ClientFirm: deref(cFirm),
				ClientPhone: deref(cPhone), ClientAddress: deref(cAddr), ClientGST: deref(cGST),
			}
		}
		qt.Rows = make([]store.QuotationRow, 0)
		byID[string(qt.ID)] = len(out)
		ids = append(ids, string(qt.ID))
		out = append(out, qt)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return out, nil
	}

	// #6 again: one batched query for every quotation's rows, keyed on the id set, ordered by
	// position to reproduce the Mongo array order.
	rRows, err := q.pool.Query(ctx, `
		SELECT quotation_id, id, material, description, has_dimensions, length, width, qty, rate,
		       cgst, sgst, igst, discount, charges, job_id, created_at, updated_at
		  FROM quotation_rows
		 WHERE quotation_id = ANY ($1)
		 ORDER BY position ASC, id ASC`, ids)
	if err != nil {
		return nil, fmt.Errorf("listing quotation rows: %w", err)
	}
	defer rRows.Close()
	for rRows.Next() {
		var qid string
		var r store.QuotationRow
		var hasDim bool
		var jobID *string
		if err := rRows.Scan(&qid, &r.ID, &r.Material, &r.Description, &hasDim, &r.Length, &r.Width,
			&r.Qty, &r.Rate, &r.Cgst, &r.Sgst, &r.Igst, &r.Discount, &r.Charges, &jobID,
			&r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("reading quotation rows: %w", err)
		}
		hd := hasDim
		r.HasDimensions = &hd
		if jobID != nil {
			r.JobID = store.ID(*jobID)
		}
		if i, ok := byID[qid]; ok {
			out[i].Rows = append(out[i].Rows, r)
		}
	}
	return out, rRows.Err()
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

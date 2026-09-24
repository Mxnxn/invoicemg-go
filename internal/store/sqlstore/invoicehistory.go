package sqlstore

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

func (i *invoices) History(ctx context.Context, companyID, invoiceID store.ID) (store.InvoiceHistory, bool, error) {
	var out store.InvoiceHistory

	var number string
	err := i.pool.QueryRow(ctx, `SELECT invoice_id FROM invoices WHERE id = $1 AND company_id = $2`,
		string(invoiceID), string(companyID)).Scan(&number)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, false, nil
	}
	if err != nil {
		return out, false, fmt.Errorf("invoice history invoice: %w", err)
	}
	out.InvoiceNumber = number

	// The invoice's entries.
	eRows, err := i.pool.Query(ctx, `
		SELECT id, date, material, description, qty, rate, amount, advance, cgst, sgst, igst, discount, charges, created_at, updated_at
		  FROM entries WHERE invoice_id = $1 ORDER BY id ASC`, string(invoiceID))
	if err != nil {
		return out, false, fmt.Errorf("invoice history entries: %w", err)
	}
	entryIDs := []string{}
	for eRows.Next() {
		var e store.InvoiceHistoryEntry
		var id string
		if err := eRows.Scan(&id, &e.Date, &e.Material, &e.Description, &e.Qty, &e.Rate, &e.Amount, &e.Advance,
			&e.Cgst, &e.Sgst, &e.Igst, &e.Discount, &e.Charges, &e.CreatedAt, &e.UpdatedAt); err != nil {
			eRows.Close()
			return out, false, fmt.Errorf("reading invoice history entries: %w", err)
		}
		e.EntryID = store.ID(id)
		out.Entries = append(out.Entries, e)
		entryIDs = append(entryIDs, id)
	}
	eRows.Close()
	if err := eRows.Err(); err != nil {
		return out, false, err
	}

	if len(entryIDs) == 0 {
		return out, true, nil
	}

	// Jobs those entries came from (the row->entry chain).
	jobIDs := []string{}
	jRows, err := i.pool.Query(ctx, `
		SELECT DISTINCT j.id, j.challan_number
		  FROM jobs j JOIN job_rows r ON r.job_id = j.id
		 WHERE j.company_id = $1 AND r.entry_id = ANY ($2)`, string(companyID), entryIDs)
	if err != nil {
		return out, false, fmt.Errorf("invoice history jobs: %w", err)
	}
	for jRows.Next() {
		var id, challan string
		if err := jRows.Scan(&id, &challan); err != nil {
			jRows.Close()
			return out, false, fmt.Errorf("reading invoice history jobs: %w", err)
		}
		out.Jobs = append(out.Jobs, store.InvoiceHistoryJob{JobID: store.ID(id), ChallanNumber: challan})
		jobIDs = append(jobIDs, id)
	}
	jRows.Close()
	if err := jRows.Err(); err != nil {
		return out, false, err
	}

	if len(jobIDs) == 0 {
		return out, true, nil
	}

	tRows, err := i.pool.Query(ctx, `
		SELECT job_id, action, detail, actor_name, created_at
		  FROM job_history WHERE job_id = ANY ($1)
		 ORDER BY created_at DESC LIMIT 200`, jobIDs)
	if err != nil {
		return out, false, fmt.Errorf("invoice history trail: %w", err)
	}
	defer tRows.Close()
	for tRows.Next() {
		var t store.InvoiceHistoryTrail
		var jobID string
		if err := tRows.Scan(&jobID, &t.Action, &t.Detail, &t.ActorName, &t.At); err != nil {
			return out, false, fmt.Errorf("reading invoice history trail: %w", err)
		}
		t.JobID = store.ID(jobID)
		out.Trail = append(out.Trail, t)
	}
	return out, true, tRows.Err()
}

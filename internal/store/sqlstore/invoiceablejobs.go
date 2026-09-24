package sqlstore

import (
	"context"
	"fmt"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

func (j *jobs) InvoiceableJobs(ctx context.Context, companyID, clientID store.ID) ([]store.InvoiceableJob, error) {
	rows, err := j.pool.Query(ctx, `
		SELECT id, challan_number, received_date, created_at, queue, total
		  FROM jobs
		 WHERE client_id = $1 AND company_id = $2
		 ORDER BY created_at DESC
		 LIMIT 200`, string(clientID), string(companyID))
	if err != nil {
		return nil, fmt.Errorf("invoiceable jobs: %w", err)
	}
	defer rows.Close()

	out := []store.InvoiceableJob{}
	byID := map[store.ID]int{}
	ids := []string{}
	for rows.Next() {
		var id, challan, queue string
		var received *time.Time
		var created time.Time
		var total float64
		if err := rows.Scan(&id, &challan, &received, &created, &queue, &total); err != nil {
			return nil, fmt.Errorf("reading invoiceable jobs: %w", err)
		}
		// receivedDate || createdAt's date (Node's fallback for pre-receivedDate jobs).
		date := created.Format("2006-01-02")
		if received != nil {
			date = received.Format("2006-01-02")
		}
		byID[store.ID(id)] = len(out)
		out = append(out, store.InvoiceableJob{ID: store.ID(id), ChallanNumber: challan, ReceivedDate: date, Queue: queue, Total: total, Rows: []store.InvoiceableRow{}})
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return out, nil
	}

	rRows, err := j.pool.Query(ctx, `
		SELECT r.job_id, r.entry_id, r.queue, r.igst, en.amount, en.has_issued
		  FROM job_rows r
		  LEFT JOIN entries en ON en.id = r.entry_id
		 WHERE r.job_id = ANY ($1)
		 ORDER BY r.position ASC, r.id ASC`, ids)
	if err != nil {
		return nil, fmt.Errorf("invoiceable job rows: %w", err)
	}
	defer rRows.Close()
	for rRows.Next() {
		var jobID, queue string
		var entryID *string
		var igst float64
		var amount *float64
		var hasIssued *bool
		if err := rRows.Scan(&jobID, &entryID, &queue, &igst, &amount, &hasIssued); err != nil {
			return nil, fmt.Errorf("reading invoiceable job rows: %w", err)
		}
		idx, ok := byID[store.ID(jobID)]
		if !ok {
			continue
		}
		row := store.InvoiceableRow{Queue: queue, Igst: igst}
		if entryID != nil {
			row.EntryID = store.ID(*entryID)
			row.HasEntry = true
			row.Amount = derefFloat(amount)
			row.HasIssued = derefBool(hasIssued)
		}
		out[idx].Rows = append(out[idx].Rows, row)
	}
	return out, rRows.Err()
}

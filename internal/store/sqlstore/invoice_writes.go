package sqlstore

import (
	"context"
	"fmt"
	"math"

	"github.com/mxnxn/invoicemg-go/internal/entrymath"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

func (i *invoices) Numbers(ctx context.Context, companyID store.ID) ([]string, error) {
	rows, err := i.pool.Query(ctx, `SELECT invoice_id FROM invoices WHERE company_id=$1`, string(companyID))
	if err != nil {
		return nil, fmt.Errorf("invoice numbers: %w", err)
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

func (i *invoices) EntryJobLabels(ctx context.Context, companyID store.ID, entryIDs []store.ID) (map[string]string, error) {
	out := map[string]string{}
	if len(entryIDs) == 0 {
		return out, nil
	}
	ids := make([]string, len(entryIDs))
	for k, id := range entryIDs {
		ids[k] = string(id)
	}
	rows, err := i.pool.Query(ctx, `
		SELECT r.entry_id, j.challan_number
		  FROM job_rows r JOIN jobs j ON j.id = r.job_id
		 WHERE j.company_id = $1 AND r.entry_id = ANY($2)`, string(companyID), ids)
	if err != nil {
		return nil, fmt.Errorf("entry job labels: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var entryID, challan string
		if err := rows.Scan(&entryID, &challan); err != nil {
			return nil, err
		}
		out[entryID] = challan
	}
	return out, rows.Err()
}

func (i *invoices) Received(ctx context.Context, companyID, invoiceID store.ID) ([]store.InvoiceReceivedRow, error) {
	rows, err := i.pool.Query(ctx, `
		SELECT rc.id, rc.date, rc.amount, rc.note, rc.invoice_id, rc.created_at, bk.id, COALESCE(bk.name,'')
		  FROM invoice_received rc LEFT JOIN banks bk ON bk.id = rc.bank_id
		 WHERE rc.company_id=$1 AND rc.invoice_id=$2
		 ORDER BY rc.created_at ASC, rc.id ASC`, string(companyID), string(invoiceID))
	if err != nil {
		return nil, fmt.Errorf("invoice received: %w", err)
	}
	defer rows.Close()
	out := make([]store.InvoiceReceivedRow, 0)
	for rows.Next() {
		var r store.InvoiceReceivedRow
		var invID, bankID *string
		if err := rows.Scan(&r.ID, &r.Date, &r.Amount, &r.Note, &invID, &r.CreatedAt, &bankID, &r.BankName); err != nil {
			return nil, err
		}
		if invID != nil {
			r.InvoiceID = store.ID(*invID)
		}
		if bankID != nil {
			r.BankID = store.ID(*bankID)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (i *invoices) Remove(ctx context.Context, companyID, invoiceID store.ID) (bool, error) {
	tx, err := i.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	var exists bool
	if err := tx.QueryRow(ctx, `SELECT true FROM invoices WHERE id=$1 AND company_id=$2`, string(invoiceID), string(companyID)).Scan(&exists); noRows(err) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("looking up invoice: %w", err)
	}
	// Un-issue the entries this invoice billed.
	if _, err := tx.Exec(ctx, `UPDATE entries SET invoice_id=NULL, has_issued=false WHERE invoice_id=$1`, string(invoiceID)); err != nil {
		return false, fmt.Errorf("un-issue entries: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM invoices WHERE id=$1 AND company_id=$2`, string(invoiceID), string(companyID)); err != nil {
		return false, fmt.Errorf("delete invoice: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func (i *invoices) Save(ctx context.Context, uid, companyID store.ID, in store.InvoiceSaveInput) (store.ID, error) {
	tx, err := i.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	// Re-issue in place if an invoice with this number already exists for the company.
	var existingID string
	err = tx.QueryRow(ctx, `SELECT id FROM invoices WHERE invoice_id=$1 AND company_id=$2`, in.InvNo, string(companyID)).Scan(&existingID)
	reissue := err == nil
	if err != nil && !noRows(err) {
		return "", fmt.Errorf("looking up invoice number: %w", err)
	}

	var invoiceID string
	if reissue {
		invoiceID = existingID
		// Release the previously-bundled entries first.
		if _, err := tx.Exec(ctx, `UPDATE entries SET invoice_id=NULL, has_issued=false WHERE invoice_id=$1`, invoiceID); err != nil {
			return "", fmt.Errorf("release entries: %w", err)
		}
	} else {
		if err := tx.QueryRow(ctx, `
			INSERT INTO invoices (uid, company_id, client_id, invoice_id, date) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
			string(uid), string(companyID), string(in.ClientID), in.InvNo, in.Date).Scan(&invoiceID); err != nil {
			return "", fmt.Errorf("insert invoice: %w", err)
		}
	}

	var paid, total float64
	for _, entryID := range in.EntryIDs {
		var amount, advance float64
		err := tx.QueryRow(ctx, `
			UPDATE entries SET invoice_id=$2, has_issued=true WHERE id=$1 RETURNING amount, advance`,
			string(entryID), invoiceID).Scan(&amount, &advance)
		if noRows(err) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("issue entry: %w", err)
		}
		paid = entrymath.RoundOffWithAmount(paid + advance)
		total = entrymath.RoundOffWithAmount(total + entrymath.RoundOffWithAmount(amount*1.18))
	}

	if _, err := tx.Exec(ctx, `UPDATE invoices SET client_id=$2, date=$3, amount=$4, total_amount=$5, updated_at=now() WHERE id=$1`,
		invoiceID, string(in.ClientID), in.Date, paid, total); err != nil {
		return "", fmt.Errorf("update invoice totals: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return store.ID(invoiceID), nil
}

func (i *invoices) Paid(ctx context.Context, companyID store.ID, in store.InvoicePaidInput) (bool, error) {
	tx, err := i.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	var invAmount, invTotal float64
	var invUID, invClient *string
	err = tx.QueryRow(ctx, `SELECT amount, total_amount, uid, client_id FROM invoices WHERE id=$1 AND company_id=$2`,
		string(in.InvoiceID), string(companyID)).Scan(&invAmount, &invTotal, &invUID, &invClient)
	if noRows(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("looking up invoice: %w", err)
	}

	// The invoice's bundled entries, in order.
	entRows, err := tx.Query(ctx, `SELECT id, amount, advance, total, cgst, sgst FROM entries WHERE invoice_id=$1 ORDER BY created_at ASC, id ASC`, string(in.InvoiceID))
	if err != nil {
		return false, fmt.Errorf("invoice entries: %w", err)
	}
	type ent struct {
		id                                 string
		amount, advance, total, cgst, sgst float64
	}
	var entries []ent
	entrySet := map[string]ent{}
	for entRows.Next() {
		var e ent
		if err := entRows.Scan(&e.id, &e.amount, &e.advance, &e.total, &e.cgst, &e.sgst); err != nil {
			entRows.Close()
			return false, err
		}
		entries = append(entries, e)
		entrySet[e.id] = e
	}
	entRows.Close()
	if err := entRows.Err(); err != nil {
		return false, err
	}

	applyEntry := func(id string, newAdvance, newTotal float64) error {
		_, err := tx.Exec(ctx, `UPDATE entries SET advance=$2, total=$3 WHERE id=$1`, id, newAdvance, newTotal)
		return err
	}
	// bumpJob raises the job that owns this entry to reflect the payment, capped at job.total.
	bumpJob := func(entryID string, applied float64) error {
		if !(applied > 0) {
			return nil
		}
		var jobID string
		var jobTotal, jobAdvance float64
		err := tx.QueryRow(ctx, `
			SELECT j.id, j.total, j.advance FROM jobs j JOIN job_rows r ON r.job_id=j.id
			 WHERE r.entry_id=$1 AND j.company_id=$2 LIMIT 1`, entryID, string(companyID)).Scan(&jobID, &jobTotal, &jobAdvance)
		if noRows(err) {
			return nil
		}
		if err != nil {
			return err
		}
		next := r2(math.Min(jobTotal, jobAdvance+applied))
		_, err = tx.Exec(ctx, `UPDATE jobs SET advance=$2 WHERE id=$1`, jobID, next)
		return err
	}

	received := in.ReceivedAmount

	if in.Mode == "manual" {
		var appliedTotal float64
		for _, a := range in.Allocations {
			e, ok := entrySet[string(a.EntryID)]
			if !ok {
				continue
			}
			applied := math.Max(0, math.Min(e.total, a.Amount))
			if applied <= 0 {
				continue
			}
			if err := applyEntry(e.id, r2(e.advance+applied), r2(e.total-applied)); err != nil {
				return false, err
			}
			if err := bumpJob(e.id, applied); err != nil {
				return false, err
			}
			appliedTotal += applied
		}
		if _, err := tx.Exec(ctx, `UPDATE invoices SET amount=$2 WHERE id=$1`, string(in.InvoiceID), r2(invAmount+appliedTotal)); err != nil {
			return false, err
		}
	} else if received+invAmount >= invTotal {
		// Enough to fully close: mop up every entry to fully paid.
		for _, e := range entries {
			taxed := e.amount * (1 + e.cgst/100 + e.sgst/100)
			applied := math.Max(0, taxed-e.advance)
			if err := applyEntry(e.id, taxed, 0); err != nil {
				return false, err
			}
			if err := bumpJob(e.id, applied); err != nil {
				return false, err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE invoices SET amount=$2 WHERE id=$1`, string(in.InvoiceID), invTotal); err != nil {
			return false, err
		}
	} else {
		// Partial: auto-fill across the entries in order.
		rem := received
		for _, e := range entries {
			if rem <= 0 {
				break
			}
			if rem > e.total {
				applied := e.total
				if err := applyEntry(e.id, e.advance+applied, 0); err != nil {
					return false, err
				}
				if err := bumpJob(e.id, applied); err != nil {
					return false, err
				}
				rem -= e.total
			} else {
				applied := rem
				if err := applyEntry(e.id, e.advance+applied, e.total-applied); err != nil {
					return false, err
				}
				if err := bumpJob(e.id, applied); err != nil {
					return false, err
				}
				rem = 0
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE invoices SET amount=$2 WHERE id=$1`, string(in.InvoiceID), invAmount+received); err != nil {
			return false, err
		}
	}

	// Log the receipt (the static amount, not the applied sum).
	var bankID any
	if in.BankID != "" {
		bankID = string(in.BankID)
	}
	uid := ""
	if invUID != nil {
		uid = *invUID
	}
	client := ""
	if invClient != nil {
		client = *invClient
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO invoice_received (uid, company_id, client_id, invoice_id, bank_id, date, amount, note)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		uid, string(companyID), client, string(in.InvoiceID), bankID, in.Date, in.ReceivedAmount, in.Note); err != nil {
		return false, fmt.Errorf("log invoice received: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

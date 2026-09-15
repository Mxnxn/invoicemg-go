package sqlstore

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

// r2 is Node's Number(x.toFixed(2)) for money: round half away from zero at the paisa.
func r2(v float64) float64 { return math.Floor(v*100+0.5) / 100 }

func (b *batchReceives) OpenJobs(ctx context.Context, uid, companyID, clientID store.ID) ([]store.BatchOpenJob, error) {
	// advance < total = not fully paid. NULLS FIRST + created_at mirrors Mongo's {receivedDate:1,
	// createdAt:1} (a missing/null date sorts before real ones). entryCount is the job's rows.
	rows, err := b.pool.Query(ctx, `
		SELECT j.id, j.challan_number, COALESCE(to_char(j.received_date, 'YYYY-MM-DD'), ''), j.total, j.advance,
		       (SELECT count(*) FROM job_rows r WHERE r.job_id = j.id)
		  FROM jobs j
		 WHERE j.uid = $1 AND j.company_id = $2 AND j.client_id = $3 AND j.advance < j.total
		 ORDER BY j.received_date ASC NULLS FIRST, j.created_at ASC, j.id ASC`,
		string(uid), string(companyID), string(clientID))
	if err != nil {
		return nil, fmt.Errorf("open jobs: %w", err)
	}
	defer rows.Close()
	out := make([]store.BatchOpenJob, 0)
	for rows.Next() {
		var j store.BatchOpenJob
		if err := rows.Scan(&j.ID, &j.ChallanNumber, &j.ReceivedDate, &j.Total, &j.Advance, &j.EntryCount); err != nil {
			return nil, err
		}
		j.Remaining = r2(j.Total - j.Advance)
		out = append(out, j)
	}
	return out, rows.Err()
}

// storedAlloc / storedEntryAlloc are the JSONB shapes persisted on batch_receives. entry_id is
// kept alongside invoice_id so a delete can reverse the entry as well as the invoice.
type storedAlloc struct {
	JobID  string  `json:"job_id"`
	Amount float64 `json:"amount"`
}
type storedEntryAlloc struct {
	EntryID   string  `json:"entry_id"`
	InvoiceID string  `json:"invoice_id"`
	Amount    float64 `json:"amount"`
}

func (b *batchReceives) Create(ctx context.Context, uid, companyID store.ID, in store.BatchReceiveWrite) (store.BatchReceive, error) {
	tx, err := b.pool.Begin(ctx)
	if err != nil {
		return store.BatchReceive{}, err
	}
	defer tx.Rollback(ctx)

	allocs := []storedAlloc{}
	entryAllocs := []storedEntryAlloc{}

	if in.Mode == "auto" {
		remaining := in.Amount
		rows, err := tx.Query(ctx, `
			SELECT id, total, advance FROM jobs
			 WHERE uid=$1 AND company_id=$2 AND client_id=$3 AND advance < total
			 ORDER BY received_date ASC NULLS FIRST, created_at ASC, id ASC`,
			string(uid), string(companyID), string(in.ClientID))
		if err != nil {
			return store.BatchReceive{}, fmt.Errorf("auto jobs: %w", err)
		}
		type jobGap struct {
			id      string
			applied float64
		}
		var plan []jobGap
		for rows.Next() {
			var id string
			var total, advance float64
			if err := rows.Scan(&id, &total, &advance); err != nil {
				rows.Close()
				return store.BatchReceive{}, err
			}
			if remaining <= 0 {
				continue
			}
			gap := r2(total - advance)
			if gap <= 0 {
				continue
			}
			applied := math.Min(gap, remaining)
			plan = append(plan, jobGap{id: id, applied: applied})
			remaining = r2(remaining - applied)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return store.BatchReceive{}, err
		}
		for _, p := range plan {
			if _, err := tx.Exec(ctx, `UPDATE jobs SET advance = advance + $2 WHERE id = $1`, p.id, p.applied); err != nil {
				return store.BatchReceive{}, fmt.Errorf("apply job advance: %w", err)
			}
			ea, err := applyJobPaymentDownstream(ctx, tx, p.id, p.applied, companyID)
			if err != nil {
				return store.BatchReceive{}, err
			}
			allocs = append(allocs, storedAlloc{JobID: p.id, Amount: p.applied})
			entryAllocs = append(entryAllocs, ea...)
		}
	} else {
		for _, a := range in.Allocations {
			if a.JobID == "" || a.Amount <= 0 {
				continue
			}
			var exists bool
			err := tx.QueryRow(ctx, `SELECT true FROM jobs WHERE id=$1 AND uid=$2 AND company_id=$3 AND client_id=$4`,
				string(a.JobID), string(uid), string(companyID), string(in.ClientID)).Scan(&exists)
			if noRows(err) {
				continue
			}
			if err != nil {
				return store.BatchReceive{}, fmt.Errorf("manual job lookup: %w", err)
			}
			if _, err := tx.Exec(ctx, `UPDATE jobs SET advance = advance + $2 WHERE id = $1`, string(a.JobID), a.Amount); err != nil {
				return store.BatchReceive{}, fmt.Errorf("apply job advance: %w", err)
			}
			ea, err := applyJobPaymentDownstream(ctx, tx, string(a.JobID), a.Amount, companyID)
			if err != nil {
				return store.BatchReceive{}, err
			}
			allocs = append(allocs, storedAlloc{JobID: string(a.JobID), Amount: a.Amount})
			entryAllocs = append(entryAllocs, ea...)
		}
	}

	allocJSON, _ := json.Marshal(allocs)
	entryJSON, _ := json.Marshal(entryAllocs)
	var bankID any
	if in.BankID != "" {
		bankID = string(in.BankID)
	}
	var id string
	if err := tx.QueryRow(ctx, `
		INSERT INTO batch_receives (uid, company_id, client_id, date, amount, note, bank_id, mode, allocations, entry_allocations)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`,
		string(uid), string(companyID), string(in.ClientID), in.Date, in.Amount, in.Note, bankID, in.Mode, allocJSON, entryJSON).Scan(&id); err != nil {
		return store.BatchReceive{}, fmt.Errorf("insert batch receive: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return store.BatchReceive{}, err
	}
	return b.getOne(ctx, uid, companyID, store.ID(id))
}

// applyJobPaymentDownstream ports routes/BatchReceive.applyJobPaymentDownstream: spread a job
// payment over its rows' linked entries (advance up, remaining total down), and when an entry is
// invoiced bump that invoice's collected amount. Returns the per-entry allocations for reversal.
func applyJobPaymentDownstream(ctx context.Context, tx pgx.Tx, jobID string, applied float64, companyID store.ID) ([]storedEntryAlloc, error) {
	rows, err := tx.Query(ctx, `SELECT entry_id FROM job_rows WHERE job_id=$1 AND entry_id IS NOT NULL ORDER BY position ASC, id ASC`, jobID)
	if err != nil {
		return nil, fmt.Errorf("job rows: %w", err)
	}
	var entryIDs []string
	for rows.Next() {
		var e *string
		if err := rows.Scan(&e); err != nil {
			rows.Close()
			return nil, err
		}
		if e != nil {
			entryIDs = append(entryIDs, *e)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	remaining := applied
	var out []storedEntryAlloc
	for _, entryID := range entryIDs {
		if remaining <= 0 {
			break
		}
		var total, advance float64
		var invoiceID *string
		err := tx.QueryRow(ctx, `SELECT total, advance, invoice_id FROM entries WHERE id=$1 AND company_id=$2`, entryID, string(companyID)).
			Scan(&total, &advance, &invoiceID)
		if noRows(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("entry lookup: %w", err)
		}
		if total <= 0 {
			continue
		}
		applied2 := r2(math.Min(total, remaining))
		if applied2 <= 0 {
			continue
		}
		if _, err := tx.Exec(ctx, `UPDATE entries SET advance = advance + $2, total = total - $2 WHERE id = $1`, entryID, applied2); err != nil {
			return nil, fmt.Errorf("apply entry payment: %w", err)
		}
		inv := ""
		if invoiceID != nil {
			inv = *invoiceID
			if _, err := tx.Exec(ctx, `UPDATE invoices SET amount = amount + $2 WHERE id = $1`, inv, applied2); err != nil {
				return nil, fmt.Errorf("apply invoice payment: %w", err)
			}
		}
		out = append(out, storedEntryAlloc{EntryID: entryID, InvoiceID: inv, Amount: applied2})
		remaining = r2(remaining - applied2)
	}
	return out, nil
}

func (b *batchReceives) Delete(ctx context.Context, uid, companyID, batchID store.ID) (bool, error) {
	tx, err := b.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	var allocB, entryB []byte
	err = tx.QueryRow(ctx, `SELECT allocations, entry_allocations FROM batch_receives WHERE id=$1 AND uid=$2 AND company_id=$3`,
		string(batchID), string(uid), string(companyID)).Scan(&allocB, &entryB)
	if noRows(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("looking up batch receive: %w", err)
	}
	var allocs []storedAlloc
	var entryAllocs []storedEntryAlloc
	_ = json.Unmarshal(allocB, &allocs)
	_ = json.Unmarshal(entryB, &entryAllocs)

	// Reverse the job advances (never below zero).
	for _, a := range allocs {
		if a.JobID == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `UPDATE jobs SET advance = GREATEST(0, advance - $2) WHERE id=$1 AND company_id=$3`,
			a.JobID, a.Amount, string(companyID)); err != nil {
			return false, fmt.Errorf("reverse job advance: %w", err)
		}
	}
	// Reverse the entry/invoice propagation.
	for _, a := range entryAllocs {
		if a.EntryID != "" {
			if _, err := tx.Exec(ctx, `UPDATE entries SET advance = GREATEST(0, advance - $2), total = total + $2 WHERE id=$1 AND company_id=$3`,
				a.EntryID, a.Amount, string(companyID)); err != nil {
				return false, fmt.Errorf("reverse entry payment: %w", err)
			}
		}
		if a.InvoiceID != "" {
			if _, err := tx.Exec(ctx, `UPDATE invoices SET amount = GREATEST(0, amount - $2) WHERE id=$1 AND company_id=$3`,
				a.InvoiceID, a.Amount, string(companyID)); err != nil {
				return false, fmt.Errorf("reverse invoice payment: %w", err)
			}
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM batch_receives WHERE id=$1`, string(batchID)); err != nil {
		return false, fmt.Errorf("delete batch receive: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// getOne resolves a single batch receive the same way List does (client/bank populated,
// destinations built from allocations + entry allocations).
func (b *batchReceives) getOne(ctx context.Context, uid, companyID, batchID store.ID) (store.BatchReceive, error) {
	var br store.BatchReceive
	var companyCol, clientCol, bankCol *string
	var allocB, entryB []byte
	err := b.pool.QueryRow(ctx, `
		SELECT br.id, br.uid, br.company_id, br.date, br.amount, br.note, br.mode, br.allocations, br.entry_allocations,
		       br.created_at, br.updated_at,
		       c.id, COALESCE(c.client_name,''), COALESCE(c.client_firm,''), COALESCE(c.client_phone,''),
		       bk.id, COALESCE(bk.name,'')
		  FROM batch_receives br
		  LEFT JOIN clients c ON c.id = br.client_id
		  LEFT JOIN banks bk ON bk.id = br.bank_id
		 WHERE br.id=$1 AND br.uid=$2 AND br.company_id=$3`, string(batchID), string(uid), string(companyID)).
		Scan(&br.ID, &br.UID, &companyCol, &br.Date, &br.Amount, &br.Note, &br.Mode, &allocB, &entryB,
			&br.CreatedAt, &br.UpdatedAt, &clientCol, &br.ClientName, &br.ClientFirm, &br.ClientPhone, &bankCol, &br.BankName)
	if err != nil {
		return store.BatchReceive{}, fmt.Errorf("reading batch receive: %w", err)
	}
	if companyCol != nil {
		br.CompanyID = store.ID(*companyCol)
	}
	if clientCol != nil {
		br.ClientID = store.ID(*clientCol)
	}
	if bankCol != nil {
		br.BankID = store.ID(*bankCol)
	}
	var allocs []storedAlloc
	var entryAllocs []storedEntryAlloc
	_ = json.Unmarshal(allocB, &allocs)
	_ = json.Unmarshal(entryB, &entryAllocs)

	jobIDs, invIDs := map[string]bool{}, map[string]bool{}
	for _, a := range allocs {
		if a.JobID != "" {
			jobIDs[a.JobID] = true
		}
	}
	for _, a := range entryAllocs {
		if a.InvoiceID != "" {
			invIDs[a.InvoiceID] = true
		}
	}
	jobNums, err := b.labels(ctx, "SELECT id, challan_number FROM jobs WHERE id = ANY($1)", jobIDs)
	if err != nil {
		return store.BatchReceive{}, err
	}
	invNums, err := b.labels(ctx, "SELECT id, invoice_id FROM invoices WHERE id = ANY($1)", invIDs)
	if err != nil {
		return store.BatchReceive{}, err
	}
	var dests []store.ReceiptDestination
	for _, a := range allocs {
		if a.JobID != "" {
			dests = append(dests, store.ReceiptDestination{Kind: "job", ID: a.JobID, Label: jobNums[a.JobID], Amount: a.Amount})
		}
	}
	for _, a := range entryAllocs {
		if a.InvoiceID == "" {
			continue
		}
		merged := false
		for i := range dests {
			if dests[i].Kind == "invoice" && dests[i].ID == a.InvoiceID {
				dests[i].Amount = r2(dests[i].Amount + a.Amount)
				merged = true
				break
			}
		}
		if !merged {
			dests = append(dests, store.ReceiptDestination{Kind: "invoice", ID: a.InvoiceID, Label: invNums[a.InvoiceID], Amount: a.Amount})
		}
	}
	br.Destinations = dests
	return br, nil
}

// CreateSimple inserts a plain client receipt (no allocation), after confirming the client
// belongs to the caller (own company or a legacy null-company row). It is /client/batchUpdate,
// whose entry auto-apply is disabled in Node.
func (b *batchReceives) CreateSimple(ctx context.Context, uid, companyID, clientID store.ID, amount float64, date, note string) (store.BatchReceive, bool, error) {
	var one int
	err := b.pool.QueryRow(ctx, `
		SELECT 1 FROM clients
		 WHERE id = $1 AND uid = $2 AND (company_id = $3 OR company_id IS NULL)`,
		string(clientID), string(uid), string(companyID)).Scan(&one)
	if err == pgx.ErrNoRows {
		return store.BatchReceive{}, false, nil
	}
	if err != nil {
		return store.BatchReceive{}, false, fmt.Errorf("verify client: %w", err)
	}

	var out store.BatchReceive
	var companyCol, clientCol *string
	err = b.pool.QueryRow(ctx, `
		INSERT INTO batch_receives (uid, company_id, client_id, date, amount, note)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, uid, company_id, client_id, date, amount, note, mode, created_at, updated_at`,
		string(uid), string(companyID), string(clientID), date, amount, note).
		Scan(&out.ID, &out.UID, &companyCol, &clientCol, &out.Date, &out.Amount, &out.Note, &out.Mode,
			&out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return store.BatchReceive{}, false, fmt.Errorf("insert batch receive: %w", err)
	}
	if companyCol != nil {
		out.CompanyID = store.ID(*companyCol)
	}
	if clientCol != nil {
		out.ClientID = store.ID(*clientCol)
	}
	return out, true, nil
}

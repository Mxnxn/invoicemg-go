package sqlstore

import (
	"context"
	"fmt"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/docnumber"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

// AddRowToJob converts one quotation row into a job row - appending to the job already started
// from this quotation, or creating a new job - then stamps the row's job_id and logs a System
// note + history entry, all in one transaction (routes/Quotation.js /row/add-to-job).
func (q *quotations) AddRowToJob(ctx context.Context, uid, companyID, quotationID, rowID store.ID) (store.QuotationRowToJobResult, error) {
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return store.QuotationRowToJobResult{}, err
	}
	defer tx.Rollback(ctx)

	var clientID *string
	var quoteNumber string
	err = tx.QueryRow(ctx,
		`SELECT client_id, quotation_number FROM quotations WHERE id=$1 AND uid=$2 AND company_id=$3`,
		string(quotationID), string(uid), string(companyID)).Scan(&clientID, &quoteNumber)
	if noRows(err) {
		return store.QuotationRowToJobResult{Status: store.QRJQuotationNotFound}, nil
	}
	if err != nil {
		return store.QuotationRowToJobResult{}, fmt.Errorf("looking up quotation: %w", err)
	}

	var row store.JobRowPricing
	var material, description string
	var jobIDCol *string
	err = tx.QueryRow(ctx, `
		SELECT material, description, length, width, qty, rate, cgst, sgst, discount, charges, job_id
		  FROM quotation_rows WHERE id=$1 AND quotation_id=$2`,
		string(rowID), string(quotationID)).
		Scan(&material, &description, &row.Length, &row.Width, &row.Qty, &row.Rate, &row.Cgst, &row.Sgst,
			&row.Discount, &row.Charges, &jobIDCol)
	if noRows(err) {
		return store.QuotationRowToJobResult{Status: store.QRJRowNotFound}, nil
	}
	if err != nil {
		return store.QuotationRowToJobResult{}, fmt.Errorf("looking up quotation row: %w", err)
	}
	if jobIDCol != nil && *jobIDCol != "" {
		return store.QuotationRowToJobResult{Status: store.QRJAlreadyAdded}, nil
	}
	// Node's rowTotal uses only cgst+sgst; igst is left out here to match.
	rowTotal := store.JobRowGrossTotal(row)

	// A job already built from this quotation? Append to it; otherwise create a new one.
	var jobID, challan string
	var rowCount int
	err = tx.QueryRow(ctx, `
		SELECT j.id, j.challan_number,
		       (SELECT count(*) FROM job_rows jr2 WHERE jr2.job_id = j.id)
		  FROM jobs j
		 WHERE j.uid=$1 AND j.company_id=$2
		   AND EXISTS (SELECT 1 FROM job_rows jr WHERE jr.job_id=j.id AND jr.quotation_id=$3)
		 LIMIT 1`,
		string(uid), string(companyID), string(quotationID)).Scan(&jobID, &challan, &rowCount)

	isNew := false
	switch {
	case err == nil:
		newRowID := fmt.Sprintf("%s-%06d", challan, rowCount+1)
		if _, err := tx.Exec(ctx, `
			INSERT INTO job_rows (job_id, row_id, position, material, description, length, width, qty, rate, cgst, sgst, discount, charges, quotation_id)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
			jobID, newRowID, rowCount, material, description, row.Length, row.Width, row.Qty, row.Rate,
			row.Cgst, row.Sgst, row.Discount, row.Charges, string(quotationID)); err != nil {
			return store.QuotationRowToJobResult{}, fmt.Errorf("append job row: %w", err)
		}
		if _, err := tx.Exec(ctx, `UPDATE jobs SET total=total+$2, updated_at=now() WHERE id=$1`, jobID, rowTotal); err != nil {
			return store.QuotationRowToJobResult{}, fmt.Errorf("bump job total: %w", err)
		}
	case noRows(err):
		isNew = true
		rows, cerr := tx.Query(ctx, `SELECT challan_number FROM jobs WHERE uid=$1 AND company_id=$2`, string(uid), string(companyID))
		if cerr != nil {
			return store.QuotationRowToJobResult{}, cerr
		}
		existing := []string{}
		for rows.Next() {
			var cn string
			if err := rows.Scan(&cn); err != nil {
				rows.Close()
				return store.QuotationRowToJobResult{}, err
			}
			existing = append(existing, cn)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return store.QuotationRowToJobResult{}, err
		}
		challan = docnumber.Next(existing, "", time.Now().UTC())

		var clientArg any
		if clientID != nil {
			clientArg = *clientID
		}
		err = tx.QueryRow(ctx, `
			INSERT INTO jobs (uid, company_id, client_id, challan_number, total, queue, progress)
			VALUES ($1,$2,$3,$4,$5,'Created','Unassigned') RETURNING id`,
			string(uid), string(companyID), clientArg, challan, rowTotal).Scan(&jobID)
		if isUniqueViolation(err) {
			return store.QuotationRowToJobResult{Status: store.QRJDupChallan}, nil
		}
		if err != nil {
			return store.QuotationRowToJobResult{}, fmt.Errorf("insert job: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO job_rows (job_id, row_id, position, material, description, length, width, qty, rate, cgst, sgst, discount, charges, quotation_id)
			VALUES ($1,$2,0,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
			jobID, challan+"-000001", material, description, row.Length, row.Width, row.Qty, row.Rate,
			row.Cgst, row.Sgst, row.Discount, row.Charges, string(quotationID)); err != nil {
			return store.QuotationRowToJobResult{}, fmt.Errorf("insert job row: %w", err)
		}
	default:
		return store.QuotationRowToJobResult{}, fmt.Errorf("looking up quotation job: %w", err)
	}

	if _, err := tx.Exec(ctx, `UPDATE quotation_rows SET job_id=$2, updated_at=now() WHERE id=$1`, string(rowID), jobID); err != nil {
		return store.QuotationRowToJobResult{}, fmt.Errorf("stamp row job_id: %w", err)
	}

	detail := fmt.Sprintf("Row added from Quote number %s", quoteNumber)
	action := "Updated"
	if isNew {
		detail = fmt.Sprintf("Created using Quote number %s", quoteNumber)
		action = "Created"
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO job_notes (job_id, uid, company_id, author_type, author_id, author_name, text)
		VALUES ($1,$2,$3,'system',$2,'System',$4)`,
		jobID, string(uid), string(companyID), detail); err != nil {
		return store.QuotationRowToJobResult{}, fmt.Errorf("log note: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO job_history (job_id, uid, company_id, actor_type, actor_id, actor_name, action, detail)
		VALUES ($1,$2,$3,'system',$2,'System',$4,$5)`,
		jobID, string(uid), string(companyID), action, detail); err != nil {
		return store.QuotationRowToJobResult{}, fmt.Errorf("log history: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return store.QuotationRowToJobResult{}, err
	}

	quote, _, err := q.Get(ctx, uid, companyID, quotationID)
	if err != nil {
		return store.QuotationRowToJobResult{}, err
	}
	list, err := (&jobs{pool: q.pool}).List(ctx, uid, companyID, "")
	if err != nil {
		return store.QuotationRowToJobResult{}, err
	}
	var job store.Job
	for _, jb := range list {
		if string(jb.ID) == jobID {
			job = jb
			break
		}
	}
	return store.QuotationRowToJobResult{Status: store.QRJOk, IsNew: isNew, Quotation: quote, Job: job}, nil
}

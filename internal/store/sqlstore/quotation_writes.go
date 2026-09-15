package sqlstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

func (q *quotations) Numbers(ctx context.Context, uid, companyID store.ID) ([]string, error) {
	rows, err := q.pool.Query(ctx, `SELECT quotation_number FROM quotations WHERE uid=$1 AND company_id=$2`, string(uid), string(companyID))
	if err != nil {
		return nil, fmt.Errorf("quotation numbers: %w", err)
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

// get loads one populated quotation (client + rows), owner+company scoped.
func (q *quotations) get(ctx context.Context, uid, companyID, quotationID store.ID) (store.Quotation, bool, error) {
	var qt store.Quotation
	var companyCol *string
	var cID, cName, cFirm, cPhone, cAddr, cGST *string
	err := q.pool.QueryRow(ctx, `
		SELECT qt.id, qt.uid, qt.company_id, qt.quotation_number, qt.date, qt.created_at, qt.updated_at,
		       c.id, c.client_name, c.client_firm, c.client_phone, c.client_address, c.client_gst
		  FROM quotations qt LEFT JOIN clients c ON c.id = qt.client_id
		 WHERE qt.id=$1 AND qt.uid=$2 AND qt.company_id=$3`, string(quotationID), string(uid), string(companyID)).
		Scan(&qt.ID, &qt.UID, &companyCol, &qt.QuotationNumber, &qt.Date, &qt.CreatedAt, &qt.UpdatedAt,
			&cID, &cName, &cFirm, &cPhone, &cAddr, &cGST)
	if noRows(err) {
		return store.Quotation{}, false, nil
	}
	if err != nil {
		return store.Quotation{}, false, fmt.Errorf("reading quotation: %w", err)
	}
	if companyCol != nil {
		qt.CompanyID = store.ID(*companyCol)
	}
	if cID != nil {
		qt.Client = &store.QuotationClient{ID: store.ID(*cID), ClientName: deref(cName), ClientFirm: deref(cFirm),
			ClientPhone: deref(cPhone), ClientAddress: deref(cAddr), ClientGST: deref(cGST)}
	}
	qt.Rows = make([]store.QuotationRow, 0)
	rRows, err := q.pool.Query(ctx, `
		SELECT id, material, description, has_dimensions, length, width, qty, rate, cgst, sgst, igst, discount, charges, job_id, created_at, updated_at
		  FROM quotation_rows WHERE quotation_id=$1 ORDER BY position ASC, id ASC`, string(quotationID))
	if err != nil {
		return store.Quotation{}, false, fmt.Errorf("reading quotation rows: %w", err)
	}
	defer rRows.Close()
	for rRows.Next() {
		var r store.QuotationRow
		var hasDim bool
		var jobID *string
		if err := rRows.Scan(&r.ID, &r.Material, &r.Description, &hasDim, &r.Length, &r.Width, &r.Qty, &r.Rate,
			&r.Cgst, &r.Sgst, &r.Igst, &r.Discount, &r.Charges, &jobID, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return store.Quotation{}, false, err
		}
		hd := hasDim
		r.HasDimensions = &hd
		if jobID != nil {
			r.JobID = store.ID(*jobID)
		}
		qt.Rows = append(qt.Rows, r)
	}
	return qt, true, rRows.Err()
}

func (q *quotations) Get(ctx context.Context, uid, companyID, quotationID store.ID) (store.Quotation, bool, error) {
	return q.get(ctx, uid, companyID, quotationID)
}

// insertRows writes the row set in order. jobIDs carries the JobID to keep for a row whose input
// ID matches an existing row (update); a provided row ID is reused so the id stays stable.
func insertRows(ctx context.Context, tx pgx.Tx, quotationID store.ID, rows []store.QuotationRowInput, jobIDs map[store.ID]store.ID) error {
	for i, r := range rows {
		var jobID *string
		if j, ok := jobIDs[r.ID]; ok && j != "" {
			s := string(j)
			jobID = &s
		}
		if r.ID != "" {
			if _, err := tx.Exec(ctx, `
				INSERT INTO quotation_rows (id, quotation_id, position, material, description, length, width, qty, rate, cgst, sgst, discount, charges, job_id)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
				string(r.ID), string(quotationID), i, r.Material, r.Description, r.Length, r.Width, r.Qty, r.Rate, r.Cgst, r.Sgst, r.Discount, r.Charges, jobID); err != nil {
				return err
			}
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO quotation_rows (quotation_id, position, material, description, length, width, qty, rate, cgst, sgst, discount, charges, job_id)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
			string(quotationID), i, r.Material, r.Description, r.Length, r.Width, r.Qty, r.Rate, r.Cgst, r.Sgst, r.Discount, r.Charges, jobID); err != nil {
			return err
		}
	}
	return nil
}

func (q *quotations) Create(ctx context.Context, uid, companyID store.ID, in store.QuotationWrite) (store.Quotation, bool, error) {
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return store.Quotation{}, false, err
	}
	defer tx.Rollback(ctx)

	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO quotations (uid, company_id, client_id, quotation_number, date)
		VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		string(uid), string(companyID), string(in.ClientID), in.QuotationNumber, in.Date).Scan(&id)
	if isUniqueViolation(err) {
		return store.Quotation{}, true, nil
	}
	if err != nil {
		return store.Quotation{}, false, fmt.Errorf("insert quotation: %w", err)
	}
	if err := insertRows(ctx, tx, store.ID(id), in.Rows, nil); err != nil {
		return store.Quotation{}, false, fmt.Errorf("insert quotation rows: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return store.Quotation{}, false, err
	}
	out, _, err := q.get(ctx, uid, companyID, store.ID(id))
	return out, false, err
}

func (q *quotations) Update(ctx context.Context, uid, companyID, quotationID store.ID, in store.QuotationUpdate) (store.Quotation, bool, bool, error) {
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return store.Quotation{}, false, false, err
	}
	defer tx.Rollback(ctx)

	var exists bool
	if err := tx.QueryRow(ctx, `SELECT true FROM quotations WHERE id=$1 AND uid=$2 AND company_id=$3`,
		string(quotationID), string(uid), string(companyID)).Scan(&exists); noRows(err) {
		return store.Quotation{}, false, false, nil
	} else if err != nil {
		return store.Quotation{}, false, false, fmt.Errorf("looking up quotation: %w", err)
	}

	if in.ClientID != nil {
		if _, err := tx.Exec(ctx, `UPDATE quotations SET client_id=$2 WHERE id=$1`, string(quotationID), string(*in.ClientID)); err != nil {
			return store.Quotation{}, false, false, err
		}
	}
	if in.Date != nil {
		if _, err := tx.Exec(ctx, `UPDATE quotations SET date=$2 WHERE id=$1`, string(quotationID), *in.Date); err != nil {
			return store.Quotation{}, false, false, err
		}
	}
	if in.QuotationNumber != nil {
		_, err := tx.Exec(ctx, `UPDATE quotations SET quotation_number=$2 WHERE id=$1`, string(quotationID), *in.QuotationNumber)
		if isUniqueViolation(err) {
			return store.Quotation{}, true, false, nil
		}
		if err != nil {
			return store.Quotation{}, false, false, err
		}
	}
	if in.Rows != nil {
		// Carry JobID across the rewrite for rows the client returned with their existing id.
		jobIDs := map[store.ID]store.ID{}
		jr, err := tx.Query(ctx, `SELECT id, job_id FROM quotation_rows WHERE quotation_id=$1`, string(quotationID))
		if err != nil {
			return store.Quotation{}, false, false, err
		}
		for jr.Next() {
			var id string
			var job *string
			if err := jr.Scan(&id, &job); err != nil {
				jr.Close()
				return store.Quotation{}, false, false, err
			}
			if job != nil {
				jobIDs[store.ID(id)] = store.ID(*job)
			}
		}
		jr.Close()
		if err := jr.Err(); err != nil {
			return store.Quotation{}, false, false, err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM quotation_rows WHERE quotation_id=$1`, string(quotationID)); err != nil {
			return store.Quotation{}, false, false, err
		}
		if err := insertRows(ctx, tx, quotationID, *in.Rows, jobIDs); err != nil {
			return store.Quotation{}, false, false, err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE quotations SET updated_at=now() WHERE id=$1`, string(quotationID)); err != nil {
		return store.Quotation{}, false, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return store.Quotation{}, false, false, err
	}
	out, _, err := q.get(ctx, uid, companyID, quotationID)
	return out, false, true, err
}

func (q *quotations) Delete(ctx context.Context, uid, companyID, quotationID store.ID) (bool, error) {
	tag, err := q.pool.Exec(ctx, `DELETE FROM quotations WHERE id=$1 AND uid=$2 AND company_id=$3`,
		string(quotationID), string(uid), string(companyID))
	if err != nil {
		return false, fmt.Errorf("delete quotation: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (q *quotations) RowDelete(ctx context.Context, uid, companyID, quotationID, rowID store.ID) (store.Quotation, bool, error) {
	var exists bool
	if err := q.pool.QueryRow(ctx, `SELECT true FROM quotations WHERE id=$1 AND uid=$2 AND company_id=$3`,
		string(quotationID), string(uid), string(companyID)).Scan(&exists); noRows(err) {
		return store.Quotation{}, false, nil
	} else if err != nil {
		return store.Quotation{}, false, fmt.Errorf("looking up quotation: %w", err)
	}
	// A missing row is a no-op (Node's optional-chaining deleteOne).
	if _, err := q.pool.Exec(ctx, `DELETE FROM quotation_rows WHERE id=$1 AND quotation_id=$2`, string(rowID), string(quotationID)); err != nil {
		return store.Quotation{}, false, fmt.Errorf("delete quotation row: %w", err)
	}
	out, _, err := q.get(ctx, uid, companyID, quotationID)
	return out, true, err
}

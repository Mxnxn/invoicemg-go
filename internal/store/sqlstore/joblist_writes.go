package sqlstore

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

func (j *jobs) ChallanNumbers(ctx context.Context, uid, companyID store.ID) ([]string, error) {
	rows, err := j.pool.Query(ctx, `SELECT challan_number FROM jobs WHERE uid=$1 AND company_id=$2`, string(uid), string(companyID))
	if err != nil {
		return nil, fmt.Errorf("challan numbers: %w", err)
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

func (j *jobs) ByEntry(ctx context.Context, uid, companyID, entryID store.ID) (store.Job, bool, error) {
	// The job owning this entry, resolved via job_rows.entry_id, then re-read through the fully
	// populated list path so the returned job matches /jobs/list exactly.
	var clientID *string
	err := j.pool.QueryRow(ctx, `
		SELECT jb.client_id FROM jobs jb JOIN job_rows r ON r.job_id = jb.id
		 WHERE r.entry_id = $1 AND jb.uid = $2 AND jb.company_id = $3 LIMIT 1`,
		string(entryID), string(uid), string(companyID)).Scan(&clientID)
	if noRows(err) {
		return store.Job{}, false, nil
	}
	if err != nil {
		return store.Job{}, false, fmt.Errorf("job by entry: %w", err)
	}
	scope := ""
	if clientID != nil {
		scope = *clientID
	}
	list, err := j.List(ctx, uid, companyID, store.ID(scope))
	if err != nil {
		return store.Job{}, false, err
	}
	for _, job := range list {
		for _, r := range job.Rows {
			if r.Entry != nil && r.Entry.ID == entryID {
				return job, true, nil
			}
		}
	}
	return store.Job{}, false, nil
}

// resolveActorName resolves the acting actor's display name (admin user name, else person name),
// shared by the job history writers (Helpers/Lifecycle actorName).
func resolveActorName(ctx context.Context, pool *pgxpool.Pool, actor store.NoteActor) string {
	if actor.Role == "admin" {
		var name string
		if err := pool.QueryRow(ctx, `SELECT name FROM users WHERE id=$1`, string(actor.UID)).Scan(&name); err == nil && name != "" {
			return name
		}
		return "Admin"
	}
	var name string
	if err := pool.QueryRow(ctx, `SELECT name FROM persons WHERE id=$1`, string(actor.PersonID)).Scan(&name); err == nil && name != "" {
		return name
	}
	return "Unknown"
}

func (j *jobs) Create(ctx context.Context, in store.JobCreateInput) (store.Job, bool, error) {
	tx, err := j.pool.Begin(ctx)
	if err != nil {
		return store.Job{}, false, err
	}
	defer tx.Rollback(ctx)

	var receivedDate any
	if in.ReceivedDate != "" {
		receivedDate = in.ReceivedDate
	}
	nullable := func(id store.ID) any {
		if id == "" {
			return nil
		}
		return string(id)
	}

	var jobID string
	err = tx.QueryRow(ctx, `
		INSERT INTO jobs (uid, company_id, client_id, employee_id, vendor_id, challan_number, received_date, total, advance, queue, progress)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'Created',$10) RETURNING id`,
		string(in.UID), string(in.CompanyID), nullable(in.ClientID), nullable(in.EmployeeID), nullable(in.VendorID),
		in.ChallanNumber, receivedDate, in.Total, in.Advance, in.Progress).Scan(&jobID)
	if isUniqueViolation(err) {
		return store.Job{}, true, nil
	}
	if err != nil {
		return store.Job{}, false, fmt.Errorf("insert job: %w", err)
	}
	for i, row := range in.Rows {
		rowID := fmt.Sprintf("%s-%06d", in.ChallanNumber, i+1)
		if _, err := tx.Exec(ctx, `
			INSERT INTO job_rows (job_id, row_id, position, material, description, length, width, qty, rate, cgst, sgst, igst, discount, charges, quotation_id)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
			jobID, rowID, i, row.Material, row.Description, row.Length, row.Width, row.Qty, row.Rate,
			row.Cgst, row.Sgst, row.Igst, row.Discount, row.Charges, nullable(row.QuotationID)); err != nil {
			return store.Job{}, false, fmt.Errorf("insert job row: %w", err)
		}
	}
	name := resolveActorName(ctx, j.pool, in.Actor)
	if _, err := tx.Exec(ctx, `
		INSERT INTO job_history (job_id, uid, company_id, actor_type, actor_id, actor_name, action, detail)
		VALUES ($1,$2,$3,$4,$5,$6,'Created',$7)`,
		jobID, string(in.UID), string(in.CompanyID), in.Actor.Role, string(in.Actor.ActorID()), name,
		fmt.Sprintf("Queue: Created, Progress: %s", in.Progress)); err != nil {
		return store.Job{}, false, fmt.Errorf("log history: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return store.Job{}, false, err
	}

	list, err := j.List(ctx, in.UID, in.CompanyID, in.ClientID)
	if err != nil {
		return store.Job{}, false, err
	}
	for _, job := range list {
		if string(job.ID) == jobID {
			return job, false, nil
		}
	}
	return store.Job{}, false, nil
}

func (j *jobs) Update(ctx context.Context, in store.JobUpdateInput) (store.Job, bool, bool, bool, error) {
	tx, err := j.pool.Begin(ctx)
	if err != nil {
		return store.Job{}, false, false, false, err
	}
	defer tx.Rollback(ctx)

	var challan string
	err = tx.QueryRow(ctx, `SELECT challan_number FROM jobs WHERE id=$1 AND uid=$2 AND company_id=$3`,
		string(in.JobID), string(in.UID), string(in.CompanyID)).Scan(&challan)
	if noRows(err) {
		return store.Job{}, false, false, false, nil
	}
	if err != nil {
		return store.Job{}, false, false, false, fmt.Errorf("looking up job: %w", err)
	}

	changed := []string{}
	setField := func(col, label string, v any) error {
		_, err := tx.Exec(ctx, `UPDATE jobs SET `+col+`=$2 WHERE id=$1`, string(in.JobID), v)
		if err == nil {
			changed = append(changed, label)
		}
		return err
	}
	if in.ClientID != nil {
		if err := setField("client_id", "client_id", string(*in.ClientID)); err != nil {
			return store.Job{}, false, false, false, err
		}
	}
	if in.ChallanNumber != nil {
		_, err := tx.Exec(ctx, `UPDATE jobs SET challan_number=$2 WHERE id=$1`, string(in.JobID), *in.ChallanNumber)
		if isUniqueViolation(err) {
			return store.Job{}, false, true, false, nil
		}
		if err != nil {
			return store.Job{}, false, false, false, err
		}
		changed = append(changed, "challanNumber")
		challan = *in.ChallanNumber
	}
	if in.ReceivedDate != nil {
		var rd any
		if *in.ReceivedDate != "" {
			rd = *in.ReceivedDate
		}
		if err := setField("received_date", "receivedDate", rd); err != nil {
			return store.Job{}, false, false, false, err
		}
	}
	if in.Advance != nil {
		if err := setField("advance", "advance", *in.Advance); err != nil {
			return store.Job{}, false, false, false, err
		}
	}

	if in.RowsSet {
		// Load the current rows in order, with everything the diff and the total need.
		rows, err := tx.Query(ctx, `
			SELECT id, row_id, entry_id, material, description, length, width, qty, rate, cgst, sgst, igst, discount, charges, quotation_id
			  FROM job_rows WHERE job_id=$1 ORDER BY position ASC, id ASC`, string(in.JobID))
		if err != nil {
			return store.Job{}, false, false, false, fmt.Errorf("job rows: %w", err)
		}
		type existingRow struct {
			id, rowID                          string
			converted                          bool
			p                                  store.JobRowPricing
			material, description, quotationID string
		}
		var existing []existingRow
		byID := map[string]int{}
		for rows.Next() {
			var e existingRow
			var entryID, quotationID *string
			if err := rows.Scan(&e.id, &e.rowID, &entryID, &e.material, &e.description, &e.p.Length, &e.p.Width,
				&e.p.Qty, &e.p.Rate, &e.p.Cgst, &e.p.Sgst, &e.p.Igst, &e.p.Discount, &e.p.Charges, &quotationID); err != nil {
				rows.Close()
				return store.Job{}, false, false, false, err
			}
			e.converted = entryID != nil
			if quotationID != nil {
				e.quotationID = *quotationID
			}
			byID[e.id] = len(existing)
			existing = append(existing, e)
			_ = entryID
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return store.Job{}, false, false, false, err
		}

		incomingByID := map[string]store.JobRowPatch{}
		for _, r := range in.Rows {
			if r.ID != "" {
				incomingByID[string(r.ID)] = r
			}
		}

		// nextRow describes a row to write, in final order.
		type nextRow struct {
			id, rowID             string // id/"" (new), rowID/"" (assign)
			p                     store.JobRowPricing
			material, description string
			quotationID           store.ID
			isNew                 bool
		}
		var next []nextRow
		for _, e := range existing {
			if e.converted {
				next = append(next, nextRow{id: e.id, rowID: e.rowID, p: e.p, material: e.material, description: e.description, quotationID: store.ID(e.quotationID)})
				continue
			}
			inc, ok := incomingByID[e.id]
			if !ok {
				continue // removed in the edit form
			}
			next = append(next, nextRow{id: e.id, rowID: e.rowID, material: inc.Material, description: inc.Description,
				quotationID: inc.QuotationID, p: pricingOf(inc)})
		}
		for _, inc := range in.Rows {
			if inc.ID != "" {
				if _, ok := byID[string(inc.ID)]; ok {
					continue // matched an existing row above
				}
			}
			next = append(next, nextRow{isNew: true, material: inc.Material, description: inc.Description,
				quotationID: inc.QuotationID, p: pricingOf(inc)})
		}
		if len(next) == 0 {
			return store.Job{}, false, false, true, nil
		}

		// Rewrite the row set. Delete all, re-insert in final order; kept rows reuse their id and
		// rowId, new rows get a fresh id and a "<challan>-<final position>" rowId.
		if _, err := tx.Exec(ctx, `DELETE FROM job_rows WHERE job_id=$1`, string(in.JobID)); err != nil {
			return store.Job{}, false, false, false, err
		}
		var total float64
		for i, nr := range next {
			total += store.JobRowGrossTotal(nr.p)
			rowID := nr.rowID
			if rowID == "" {
				rowID = fmt.Sprintf("%s-%06d", challan, i+1)
			}
			var quot any
			if nr.quotationID != "" {
				quot = string(nr.quotationID)
			}
			if nr.isNew || nr.id == "" {
				if _, err := tx.Exec(ctx, `
					INSERT INTO job_rows (job_id, row_id, position, material, description, length, width, qty, rate, cgst, sgst, igst, discount, charges, quotation_id)
					VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
					string(in.JobID), rowID, i, nr.material, nr.description, nr.p.Length, nr.p.Width, nr.p.Qty, nr.p.Rate,
					nr.p.Cgst, nr.p.Sgst, nr.p.Igst, nr.p.Discount, nr.p.Charges, quot); err != nil {
					return store.Job{}, false, false, false, err
				}
			} else {
				if _, err := tx.Exec(ctx, `
					INSERT INTO job_rows (id, job_id, row_id, position, material, description, length, width, qty, rate, cgst, sgst, igst, discount, charges, quotation_id)
					VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
					nr.id, string(in.JobID), rowID, i, nr.material, nr.description, nr.p.Length, nr.p.Width, nr.p.Qty, nr.p.Rate,
					nr.p.Cgst, nr.p.Sgst, nr.p.Igst, nr.p.Discount, nr.p.Charges, quot); err != nil {
					return store.Job{}, false, false, false, err
				}
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE jobs SET total=$2 WHERE id=$1`, string(in.JobID), total); err != nil {
			return store.Job{}, false, false, false, err
		}
		changed = append(changed, "rows")
	}

	if _, err := tx.Exec(ctx, `UPDATE jobs SET updated_at=now() WHERE id=$1`, string(in.JobID)); err != nil {
		return store.Job{}, false, false, false, err
	}
	detail := "No fields changed"
	if len(changed) > 0 {
		detail = "Changed: " + strings.Join(changed, ", ")
	}
	name := resolveActorName(ctx, j.pool, in.Actor)
	if _, err := tx.Exec(ctx, `
		INSERT INTO job_history (job_id, uid, company_id, actor_type, actor_id, actor_name, action, detail)
		VALUES ($1,$2,$3,$4,$5,$6,'Updated',$7)`,
		string(in.JobID), string(in.UID), string(in.CompanyID), in.Actor.Role, string(in.Actor.ActorID()), name, detail); err != nil {
		return store.Job{}, false, false, false, fmt.Errorf("log history: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return store.Job{}, false, false, false, err
	}

	list, err := j.List(ctx, in.UID, in.CompanyID, "")
	if err != nil {
		return store.Job{}, false, false, false, err
	}
	for _, job := range list {
		if job.ID == in.JobID {
			return job, true, false, false, nil
		}
	}
	return store.Job{}, true, false, false, nil
}

func pricingOf(r store.JobRowPatch) store.JobRowPricing {
	length := r.Length
	if length == "" {
		length = "1"
	}
	width := r.Width
	if width == "" {
		width = "1"
	}
	return store.JobRowPricing{Length: length, Width: width, Qty: r.Qty, Rate: r.Rate,
		Cgst: r.Cgst, Sgst: r.Sgst, Igst: r.Igst, Discount: r.Discount, Charges: r.Charges}
}

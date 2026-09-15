package sqlstore

import (
	"context"
	"fmt"

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

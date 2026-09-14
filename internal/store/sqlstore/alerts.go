package sqlstore

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type alerts struct{ pool *pgxpool.Pool }

func (s *Store) Alerts() store.Alerts { return &alerts{pool: s.pool} }

func (a *alerts) Job(ctx context.Context, id store.ID) (store.AlertJob, error) {
	var job store.AlertJob
	var clientID, companyID, uid *string
	var createdAt *time.Time

	// received_date is a DATE; render it to the YYYY-MM-DD string the app speaks in SQL, so a
	// server in another timezone cannot shift the day (the same reason days.go does).
	err := a.pool.QueryRow(ctx, `
		SELECT id, challan_number, COALESCE(to_char(received_date, 'YYYY-MM-DD'), ''),
		       queue, client_id, company_id, uid, created_at
		  FROM jobs
		 WHERE id = $1`, string(id)).
		Scan(&job.ID, &job.ChallanNumber, &job.ReceivedDate, &job.Queue, &clientID, &companyID, &uid, &createdAt)
	if noRows(err) {
		return store.AlertJob{}, store.ErrNotFound
	}
	if err != nil {
		return store.AlertJob{}, fmt.Errorf("looking up job: %w", err)
	}
	job.CreatedAt = createdAt
	if clientID != nil {
		job.ClientID = store.ID(*clientID)
	}
	if companyID != nil {
		job.CompanyID = store.ID(*companyID)
	}
	if uid != nil {
		job.UID = store.ID(*uid)
	}

	// A row is finally addressable on its own here, so ORDER BY position reproduces the Mongo
	// array order, with id as the tiebreak (#19) so equal positions cannot arrange two ways.
	rows, err := a.pool.Query(ctx, `
		SELECT id, row_id, description, material, qty, has_dimensions,
		       length, width, rate, cgst, sgst, igst, discount, charges, queue, progress
		  FROM job_rows
		 WHERE job_id = $1
		 ORDER BY position ASC, id ASC`, string(id))
	if err != nil {
		return store.AlertJob{}, fmt.Errorf("looking up job rows: %w", err)
	}
	defer rows.Close()

	job.Rows = make([]store.AlertRow, 0)
	for rows.Next() {
		var r store.AlertRow
		var hasDim bool
		if err := rows.Scan(&r.ID, &r.RowID, &r.Description, &r.Material, &r.Qty, &hasDim,
			&r.Length, &r.Width, &r.Rate, &r.Cgst, &r.Sgst, &r.Igst, &r.Discount, &r.Charges,
			&r.Queue, &r.Progress); err != nil {
			return store.AlertJob{}, fmt.Errorf("reading job rows: %w", err)
		}
		// NOT NULL in the schema, so it is always a real bool; take its address so jobmath sees
		// a present value rather than nil. A future NULL would need a *bool scan target.
		hd := hasDim
		r.HasDimensions = &hd
		job.Rows = append(job.Rows, r)
	}
	return job, rows.Err()
}

func (a *alerts) Client(ctx context.Context, id store.ID) (store.AlertClient, error) {
	if id == "" {
		return store.AlertClient{}, store.ErrNotFound
	}
	var c store.AlertClient
	err := a.pool.QueryRow(ctx,
		`SELECT client_name, client_firm FROM clients WHERE id = $1`, string(id)).
		Scan(&c.Name, &c.Firm)
	if noRows(err) {
		return store.AlertClient{}, store.ErrNotFound
	}
	if err != nil {
		return store.AlertClient{}, fmt.Errorf("looking up client: %w", err)
	}
	return c, nil
}

func (a *alerts) Company(ctx context.Context, id store.ID) (store.AlertCompany, error) {
	if id == "" {
		return store.AlertCompany{}, store.ErrNotFound
	}
	// SCHEMA GAP (deploy/postgres/001-schema.sql): the Postgres companies table carries only
	// `name` so far - the letterhead columns Model/Company.js has (firm, phone, url, address,
	// gst) are not migrated yet, nor is a job_reviews table. The Mongo tenant returns all of
	// them; here the extra fields come back blank until the schema is extended. Flagged rather
	// than silently wrong: on the local Postgres stack the alert page shows the company name
	// but no logo or GST. Not a sideways-parity concern - the sideways tenant is Mongo.
	var c store.AlertCompany
	err := a.pool.QueryRow(ctx, `SELECT name FROM companies WHERE id = $1`, string(id)).Scan(&c.Name)
	if noRows(err) {
		return store.AlertCompany{}, store.ErrNotFound
	}
	if err != nil {
		return store.AlertCompany{}, fmt.Errorf("looking up company: %w", err)
	}
	return c, nil
}

func (a *alerts) Review(_ context.Context, _ store.ID) (store.AlertReview, error) {
	// SCHEMA GAP: there is no job_reviews table in Postgres yet (see Company above). Every job
	// reads as not-yet-reviewed here, so the page shows an empty form; the Mongo tenant answers
	// truthfully. Migrating job_reviews is part of finishing the Alert domain on Postgres, and
	// it is not a sideways-parity concern because the sideways tenant is Mongo.
	return store.AlertReview{}, store.ErrNotFound
}

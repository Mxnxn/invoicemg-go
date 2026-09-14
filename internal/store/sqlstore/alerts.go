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
	var c store.AlertCompany
	err := a.pool.QueryRow(ctx,
		`SELECT name, firm, phone, url, address, gst FROM companies WHERE id = $1`, string(id)).
		Scan(&c.Name, &c.Firm, &c.Phone, &c.URL, &c.Address, &c.Gst)
	if noRows(err) {
		return store.AlertCompany{}, store.ErrNotFound
	}
	if err != nil {
		return store.AlertCompany{}, fmt.Errorf("looking up company: %w", err)
	}
	return c, nil
}

func (a *alerts) Review(ctx context.Context, jobID store.ID) (store.AlertReview, error) {
	var r store.AlertReview
	var s store.ReviewScores
	err := a.pool.QueryRow(ctx, `
		SELECT id, quality, speed, communication, satisfaction, overall, comment, created_at
		  FROM job_reviews
		 WHERE job_id = $1`, string(jobID)).
		Scan(&r.ID, &s.Quality, &s.Speed, &s.Communication, &s.Satisfaction, &s.Overall, &r.Comment, &r.CreatedAt)
	if noRows(err) {
		return store.AlertReview{}, store.ErrNotFound
	}
	if err != nil {
		return store.AlertReview{}, fmt.Errorf("looking up review: %w", err)
	}
	r.Scores = s
	return r, nil
}

func (a *alerts) CreateReview(ctx context.Context, r store.NewReview) error {
	// routes/Alert.js's company fallback: the job's own company, else the owner's default, else
	// any they own. ORDER BY is_default DESC does in one query what Node does in two; id is the
	// tiebreak (#19) so the "any" case is deterministic.
	companyID := string(r.CompanyID)
	if companyID == "" {
		err := a.pool.QueryRow(ctx, `
			SELECT id FROM companies WHERE uid = $1
			 ORDER BY is_default DESC, created_at ASC, id ASC LIMIT 1`, string(r.UID)).Scan(&companyID)
		if noRows(err) {
			return store.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("resolving company: %w", err)
		}
	}

	_, err := a.pool.Exec(ctx, `
		INSERT INTO job_reviews
		  (uid, company_id, job_id, client_id, jobcard_id, challan_number, client_name,
		   quality, speed, communication, satisfaction, overall, comment)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		string(r.UID), companyID, string(r.JobID), string(r.ClientID), r.JobcardID, r.ChallanNumber,
		r.ClientName, r.Scores.Quality, r.Scores.Speed, r.Scores.Communication, r.Scores.Satisfaction,
		r.Scores.Overall, r.Comment)
	if err != nil {
		if isUniqueViolation(err) {
			return store.ErrDuplicate
		}
		return fmt.Errorf("creating review: %w", err)
	}
	return nil
}

package sqlstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type days struct{ pool *pgxpool.Pool }

// dateFormat renders a DATE column as the YYYY-MM-DD string the whole application speaks.
//
// Done in SQL rather than by scanning into a time.Time and formatting in Go, so a server
// running in a different timezone cannot shift the day. That exact bug - a calendar date
// parsed as UTC midnight and rendered one day earlier - is why these are DATE columns here
// and strings in Mongo.
const dateFormat = `to_char(%s, 'YYYY-MM-DD')`

func (d *days) CountsByDate(ctx context.Context, companyID store.ID) ([]store.DayCount, error) {
	// The Mongo version is a $group aggregation with a $size over a subdocument array, and a
	// $ifNull to stop a job with no rows failing the whole pipeline. Here rows are a table:
	// a LEFT JOIN counts them, and a job with none contributes 0 without a special case.
	rows, err := d.pool.Query(ctx, `
		SELECT `+fmt.Sprintf(dateFormat, "j.received_date")+` AS date,
		       COUNT(DISTINCT j.id)                           AS jobs,
		       COUNT(r.id)                                    AS cards
		  FROM jobs j
		  LEFT JOIN job_rows r ON r.job_id = j.id
		 WHERE j.company_id = $1 AND j.received_date IS NOT NULL
		 GROUP BY j.received_date`, string(companyID))
	if err != nil {
		return nil, fmt.Errorf("counting jobs by date: %w", err)
	}
	defer rows.Close()

	var out []store.DayCount
	for rows.Next() {
		var c store.DayCount
		if err := rows.Scan(&c.Date, &c.Jobs, &c.Cards); err != nil {
			return nil, fmt.Errorf("reading job counts: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (d *days) SheetDates(ctx context.Context, companyID store.ID) (map[string]store.ID, error) {
	// DISTINCT ON keeps the first sheet per date in one query. There is nothing to choose
	// between two sheets for one day, and the id is only a fallback link target now.
	rows, err := d.pool.Query(ctx, `
		SELECT DISTINCT ON (date) `+fmt.Sprintf(dateFormat, "date")+`, id
		  FROM sheets
		 WHERE company_id = $1
		 ORDER BY date, created_at ASC`, string(companyID))
	if err != nil {
		return nil, fmt.Errorf("listing sheets: %w", err)
	}
	defer rows.Close()

	out := map[string]store.ID{}
	for rows.Next() {
		var date, id string
		if err := rows.Scan(&date, &id); err != nil {
			return nil, fmt.Errorf("reading sheets: %w", err)
		}
		out[date] = store.ID(id)
	}
	return out, rows.Err()
}

func (d *days) OpenJobs(ctx context.Context, companyID store.ID, limit int) ([]store.OpenJob, error) {
	// One query for what Mongo needs three round trips to answer: the $elemMatch filter, the
	// per-row count, and the .populate() of the client. EXISTS uses the partial index
	// job_rows_open_idx (WHERE queue <> 'Done'), so the scan is over open rows only rather
	// than over every row ever created.
	rows, err := d.pool.Query(ctx, `
		SELECT j.id,
		       j.challan_number,
		       COALESCE(`+fmt.Sprintf(dateFormat, "j.received_date")+`, '') AS received_date,
		       j.total,
		       COUNT(r.id)                                     AS cards,
		       COUNT(r.id) FILTER (WHERE r.queue <> 'Done')    AS open_cards,
		       c.id IS NOT NULL                                AS has_client,
		       COALESCE(NULLIF(c.client_firm, ''), c.client_name, '') AS client_name,
		       COALESCE(c.client_phone, '')                    AS client_phone
		  FROM jobs j
		  LEFT JOIN job_rows r ON r.job_id = j.id
		  LEFT JOIN clients  c ON c.id = j.client_id
		 WHERE j.company_id = $1
		   AND EXISTS (SELECT 1 FROM job_rows o WHERE o.job_id = j.id AND o.queue <> 'Done')
		 GROUP BY j.id, c.id
		 -- Oldest first: the job-id sitting open the longest is the one worth looking at,
		 -- which is the opposite of how every other list here is sorted. NULLS FIRST matches
		 -- Mongo, where a missing receivedDate sorts before every date.
		 ORDER BY j.received_date ASC NULLS FIRST
		 LIMIT $2`, string(companyID), limit)
	if err != nil {
		return nil, fmt.Errorf("listing open jobs: %w", err)
	}
	defer rows.Close()

	var out []store.OpenJob
	for rows.Next() {
		var j store.OpenJob
		if err := rows.Scan(&j.ID, &j.ChallanNumber, &j.ReceivedDate, &j.Total,
			&j.Cards, &j.OpenCards, &j.HasClient, &j.ClientName, &j.ClientPhone); err != nil {
			return nil, fmt.Errorf("reading open jobs: %w", err)
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

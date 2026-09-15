package sqlstore

import (
	"context"
	"fmt"

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

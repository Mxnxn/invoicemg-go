package sqlstore

import (
	"context"
	"fmt"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

func (a *analytics) ProductionWip(ctx context.Context, companyID store.ID) (store.ProductionWipData, error) {
	var out store.ProductionWipData

	// Jobs, for the rowless-job fallback and a stable card order.
	type jobHead struct {
		queue      string
		employee   store.ID
		createdAt  time.Time
	}
	jobOrder := []store.ID{}
	jobs := map[store.ID]jobHead{}
	jr, err := a.pool.Query(ctx, `SELECT id, queue, employee_id, created_at FROM jobs WHERE company_id = $1 ORDER BY id ASC`, string(companyID))
	if err != nil {
		return out, fmt.Errorf("wip jobs: %w", err)
	}
	for jr.Next() {
		var id string
		var queue string
		var emp *string
		var created time.Time
		if err := jr.Scan(&id, &queue, &emp, &created); err != nil {
			jr.Close()
			return out, fmt.Errorf("reading wip jobs: %w", err)
		}
		h := jobHead{queue: queue, createdAt: created}
		if emp != nil {
			h.employee = store.ID(*emp)
		}
		jobs[store.ID(id)] = h
		jobOrder = append(jobOrder, store.ID(id))
	}
	jr.Close()
	if err := jr.Err(); err != nil {
		return out, err
	}

	// Rows, grouped by job (position order), with the row-wins key.
	rowsByJob := map[store.ID][]store.ProductionCard{}
	rr, err := a.pool.Query(ctx, `
		SELECT jr.job_id, COALESCE(NULLIF(jr.row_id, ''), jr.id), jr.queue, jr.employee_id, jr.created_at
		  FROM job_rows jr JOIN jobs j ON j.id = jr.job_id
		 WHERE j.company_id = $1
		 ORDER BY jr.job_id, jr.position ASC`, string(companyID))
	if err != nil {
		return out, fmt.Errorf("wip rows: %w", err)
	}
	for rr.Next() {
		var jobID, key, queue string
		var emp *string
		var created time.Time
		if err := rr.Scan(&jobID, &key, &queue, &emp, &created); err != nil {
			rr.Close()
			return out, fmt.Errorf("reading wip rows: %w", err)
		}
		c := store.ProductionCard{JobID: store.ID(jobID), Key: key, Queue: queue, CreatedAt: created}
		if emp != nil {
			c.EmployeeID = store.ID(*emp)
		}
		rowsByJob[store.ID(jobID)] = append(rowsByJob[store.ID(jobID)], c)
	}
	rr.Close()
	if err := rr.Err(); err != nil {
		return out, err
	}

	for _, id := range jobOrder {
		if cards := rowsByJob[id]; len(cards) > 0 {
			out.Cards = append(out.Cards, cards...)
			continue
		}
		h := jobs[id]
		out.Cards = append(out.Cards, store.ProductionCard{JobID: id, Key: "", Queue: h.queue, EmployeeID: h.employee, CreatedAt: h.createdAt})
	}

	if out.Events, err = a.queueAdvancedEvents(ctx, companyID, nil); err != nil {
		return out, err
	}

	// Holder names for the cards' employees, in one query.
	ids := map[string]struct{}{}
	for _, c := range out.Cards {
		if c.EmployeeID != "" {
			ids[string(c.EmployeeID)] = struct{}{}
		}
	}
	out.PersonNames, err = a.personNames(ctx, ids)
	return out, err
}

func (a *analytics) ProductionThroughput(ctx context.Context, companyID store.ID, windowStart time.Time) (store.ProductionThroughputData, error) {
	var out store.ProductionThroughputData
	var err error
	if out.Events, err = a.queueAdvancedEvents(ctx, companyID, &windowStart); err != nil {
		return out, err
	}

	// Row queues grouped by job, for jobs created in the window.
	rowQ := map[store.ID][]string{}
	rr, err := a.pool.Query(ctx, `
		SELECT jr.job_id, jr.queue FROM job_rows jr JOIN jobs j ON j.id = jr.job_id
		 WHERE j.company_id = $1 AND j.created_at >= $2`, string(companyID), windowStart)
	if err != nil {
		return out, fmt.Errorf("throughput rows: %w", err)
	}
	for rr.Next() {
		var jobID, q string
		if err := rr.Scan(&jobID, &q); err != nil {
			rr.Close()
			return out, fmt.Errorf("reading throughput rows: %w", err)
		}
		rowQ[store.ID(jobID)] = append(rowQ[store.ID(jobID)], q)
	}
	rr.Close()
	if err := rr.Err(); err != nil {
		return out, err
	}

	jr, err := a.pool.Query(ctx, `SELECT id, created_at, updated_at, queue FROM jobs WHERE company_id = $1 AND created_at >= $2`, string(companyID), windowStart)
	if err != nil {
		return out, fmt.Errorf("throughput jobs: %w", err)
	}
	defer jr.Close()
	for jr.Next() {
		var id, queue string
		var created, updated time.Time
		if err := jr.Scan(&id, &created, &updated, &queue); err != nil {
			return out, fmt.Errorf("reading throughput jobs: %w", err)
		}
		out.DoneJobs = append(out.DoneJobs, store.ProductionDoneJob{
			CreatedAt: created, UpdatedAt: updated, Queue: queue, RowQueues: rowQ[store.ID(id)],
		})
	}
	return out, jr.Err()
}

// queueAdvancedEvents reads the company's "Queue advanced" history ascending; a non-nil since caps
// it to createdAt >= since.
func (a *analytics) queueAdvancedEvents(ctx context.Context, companyID store.ID, since *time.Time) ([]store.ProductionEvent, error) {
	sql := `SELECT job_id, detail, from_stage, to_stage, row_key, actor_name, created_at
	          FROM job_history WHERE company_id = $1 AND action = 'Queue advanced'`
	args := []any{string(companyID)}
	if since != nil {
		sql += ` AND created_at >= $2`
		args = append(args, *since)
	}
	sql += ` ORDER BY created_at ASC`
	rows, err := a.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("queue-advanced events: %w", err)
	}
	defer rows.Close()
	var out []store.ProductionEvent
	for rows.Next() {
		var e store.ProductionEvent
		var jobID string
		if err := rows.Scan(&jobID, &e.Detail, &e.FromStage, &e.ToStage, &e.RowKey, &e.ActorName, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("reading queue-advanced events: %w", err)
		}
		e.JobID = store.ID(jobID)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (a *analytics) personNames(ctx context.Context, ids map[string]struct{}) (map[store.ID]string, error) {
	out := map[store.ID]string{}
	if len(ids) == 0 {
		return out, nil
	}
	list := make([]string, 0, len(ids))
	for id := range ids {
		list = append(list, id)
	}
	rows, err := a.pool.Query(ctx, `SELECT id, name FROM persons WHERE id = ANY ($1)`, list)
	if err != nil {
		return nil, fmt.Errorf("person names: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, fmt.Errorf("reading person names: %w", err)
		}
		out[store.ID(id)] = name
	}
	return out, rows.Err()
}

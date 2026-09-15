package sqlstore

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

// jobTx runs fn inside a transaction after confirming the job exists (owner+company scoped),
// logs the history entries fn returns, and re-reads the populated job. fn reports a status; a
// non-OK status rolls back with no history.
func (j *jobs) jobTx(ctx context.Context, uid, companyID, jobID store.ID, actor store.NoteActor,
	fn func(tx pgx.Tx) (store.JobTxStatus, []histEntry, error)) (store.Job, store.JobTxStatus, error) {
	tx, err := j.pool.Begin(ctx)
	if err != nil {
		return store.Job{}, store.JobTxJobNotFound, err
	}
	defer tx.Rollback(ctx)

	var exists bool
	err = tx.QueryRow(ctx, `SELECT true FROM jobs WHERE id=$1 AND uid=$2 AND company_id=$3`,
		string(jobID), string(uid), string(companyID)).Scan(&exists)
	if noRows(err) {
		return store.Job{}, store.JobTxJobNotFound, nil
	}
	if err != nil {
		return store.Job{}, store.JobTxJobNotFound, fmt.Errorf("looking up job: %w", err)
	}
	status, hist, err := fn(tx)
	if err != nil {
		return store.Job{}, store.JobTxJobNotFound, err
	}
	if status != store.JobTxOK {
		return store.Job{}, status, nil
	}
	name := resolveActorName(ctx, j.pool, actor)
	for _, h := range hist {
		if _, err := tx.Exec(ctx, `
			INSERT INTO job_history (job_id, uid, company_id, actor_type, actor_id, actor_name, action, detail)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			string(jobID), string(uid), string(companyID), actor.Role, string(actor.ActorID()), name, h.action, h.detail); err != nil {
			return store.Job{}, store.JobTxJobNotFound, fmt.Errorf("log history: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE jobs SET updated_at=now() WHERE id=$1`, string(jobID)); err != nil {
		return store.Job{}, store.JobTxJobNotFound, err
	}
	if err := tx.Commit(ctx); err != nil {
		return store.Job{}, store.JobTxJobNotFound, err
	}
	list, err := j.List(ctx, uid, companyID, "")
	if err != nil {
		return store.Job{}, store.JobTxJobNotFound, err
	}
	for _, job := range list {
		if job.ID == jobID {
			return job, store.JobTxOK, nil
		}
	}
	return store.Job{}, store.JobTxOK, nil
}

type histEntry struct{ action, detail string }

// personName resolves a person's display name for a history detail ("Unknown" when gone).
func (j *jobs) personName(ctx context.Context, tx pgx.Tx, personID store.ID) string {
	if personID == "" {
		return ""
	}
	var name string
	if err := tx.QueryRow(ctx, `SELECT name FROM persons WHERE id=$1`, string(personID)).Scan(&name); err == nil && name != "" {
		return name
	}
	return "Unknown"
}

func (j *jobs) Assign(ctx context.Context, uid, companyID, jobID store.ID, actor store.NoteActor, kind string, personID store.ID) (store.Job, store.JobTxStatus, error) {
	return j.jobTx(ctx, uid, companyID, jobID, actor, func(tx pgx.Tx) (store.JobTxStatus, []histEntry, error) {
		var pid any
		if personID != "" {
			pid = string(personID)
		}
		if kind == "employee" {
			if _, err := tx.Exec(ctx, `UPDATE jobs SET employee_id=$2, vendor_id=NULL WHERE id=$1`, string(jobID), pid); err != nil {
				return 0, nil, err
			}
		} else {
			if _, err := tx.Exec(ctx, `UPDATE jobs SET vendor_id=$2, employee_id=NULL WHERE id=$1`, string(jobID), pid); err != nil {
				return 0, nil, err
			}
		}
		// Recompute progress unless the job is Complete.
		if _, err := tx.Exec(ctx, `
			UPDATE jobs SET progress = CASE WHEN progress='Complete' THEN progress
			  WHEN (employee_id IS NOT NULL OR vendor_id IS NOT NULL) THEN 'In Progress' ELSE 'Unassigned' END
			 WHERE id=$1`, string(jobID)); err != nil {
			return 0, nil, err
		}
		action, detail := "Unassigned", fmt.Sprintf("Cleared %s assignment", kind)
		if personID != "" {
			label := "Vendor"
			if kind == "employee" {
				label = "Employee"
			}
			action, detail = "Assigned", fmt.Sprintf("%s: %s", label, j.personName(ctx, tx, personID))
		}
		return store.JobTxOK, []histEntry{{action, detail}}, nil
	})
}

func (j *jobs) Progress(ctx context.Context, uid, companyID, jobID store.ID, actor store.NoteActor, progress string) (store.Job, store.JobTxStatus, error) {
	return j.jobTx(ctx, uid, companyID, jobID, actor, func(tx pgx.Tx) (store.JobTxStatus, []histEntry, error) {
		var prevProgress, prevQueue string
		var hasEmp, hasVen bool
		if err := tx.QueryRow(ctx, `SELECT progress, queue, employee_id IS NOT NULL, vendor_id IS NOT NULL FROM jobs WHERE id=$1`, string(jobID)).
			Scan(&prevProgress, &prevQueue, &hasEmp, &hasVen); err != nil {
			return 0, nil, err
		}
		if progress != "Unassigned" && !hasEmp && !hasVen {
			return store.JobTxNeedsAssignee, nil, nil
		}
		if progress == "Unassigned" {
			if _, err := tx.Exec(ctx, `UPDATE jobs SET employee_id=NULL, vendor_id=NULL, progress='Unassigned' WHERE id=$1`, string(jobID)); err != nil {
				return 0, nil, err
			}
			return store.JobTxOK, []histEntry{{"Progress changed", fmt.Sprintf("%s → Unassigned (assignment cleared)", prevProgress)}}, nil
		}
		if progress != "Complete" {
			if _, err := tx.Exec(ctx, `UPDATE jobs SET progress=$2 WHERE id=$1`, string(jobID), progress); err != nil {
				return 0, nil, err
			}
			return store.JobTxOK, []histEntry{{"Progress changed", fmt.Sprintf("%s → %s", prevProgress, progress)}}, nil
		}
		// Complete: advance the queue stage, clear the assignee, reset progress.
		next, advanced := nextQueueStage(prevQueue)
		if !advanced {
			next = prevQueue
		}
		if _, err := tx.Exec(ctx, `UPDATE jobs SET queue=$2, employee_id=NULL, vendor_id=NULL, progress='Unassigned' WHERE id=$1`, string(jobID), next); err != nil {
			return 0, nil, err
		}
		hist := []histEntry{}
		if advanced {
			hist = append(hist, histEntry{"Queue advanced", fmt.Sprintf("%s → %s", prevQueue, next)})
		}
		hist = append(hist, histEntry{"Progress changed", fmt.Sprintf("%s → Complete", prevProgress)})
		return store.JobTxOK, hist, nil
	})
}

func (j *jobs) SetQueue(ctx context.Context, uid, companyID, jobID store.ID, actor store.NoteActor, queue string) (store.Job, store.JobTxStatus, error) {
	return j.jobTx(ctx, uid, companyID, jobID, actor, func(tx pgx.Tx) (store.JobTxStatus, []histEntry, error) {
		var prev string
		if err := tx.QueryRow(ctx, `SELECT queue FROM jobs WHERE id=$1`, string(jobID)).Scan(&prev); err != nil {
			return 0, nil, err
		}
		if prev == queue {
			return store.JobTxOK, nil, nil // no-op, no history
		}
		if _, err := tx.Exec(ctx, `UPDATE jobs SET queue=$2, employee_id=NULL, vendor_id=NULL, progress='Unassigned' WHERE id=$1`, string(jobID), queue); err != nil {
			return 0, nil, err
		}
		return store.JobTxOK, []histEntry{{"Queue advanced", fmt.Sprintf("%s → %s", prev, queue)}}, nil
	})
}

func (j *jobs) QueueOrder(ctx context.Context, uid, companyID, jobID store.ID, actor store.NoteActor, order []string) (store.Job, store.JobTxStatus, error) {
	return j.jobTx(ctx, uid, companyID, jobID, actor, func(tx pgx.Tx) (store.JobTxStatus, []histEntry, error) {
		if _, err := tx.Exec(ctx, `UPDATE jobs SET queue_order=$2 WHERE id=$1`, string(jobID), order); err != nil {
			return 0, nil, err
		}
		return store.JobTxOK, []histEntry{{"Queue reordered", strings.Join(order, " → ")}}, nil
	})
}

// rowRef loads a row's current state (owner+company scoped) for the row transitions.
func (j *jobs) rowRef(ctx context.Context, tx pgx.Tx, jobID, rowID store.ID) (rowID2, label, queue, progress string, hasEmp bool, found bool, err error) {
	var rid, rowid, q, p string
	var emp bool
	e := tx.QueryRow(ctx, `SELECT id, COALESCE(NULLIF(row_id,''), id), queue, progress, employee_id IS NOT NULL FROM job_rows WHERE id=$1 AND job_id=$2`,
		string(rowID), string(jobID)).Scan(&rid, &rowid, &q, &p, &emp)
	if noRows(e) {
		return "", "", "", "", false, false, nil
	}
	if e != nil {
		return "", "", "", "", false, false, e
	}
	return rid, rowid, q, p, emp, true, nil
}

func (j *jobs) RowAssign(ctx context.Context, uid, companyID, jobID, rowID store.ID, actor store.NoteActor, employeeID store.ID) (store.Job, store.JobTxStatus, error) {
	return j.jobTx(ctx, uid, companyID, jobID, actor, func(tx pgx.Tx) (store.JobTxStatus, []histEntry, error) {
		_, label, _, progress, _, found, err := j.rowRef(ctx, tx, jobID, rowID)
		if err != nil {
			return 0, nil, err
		}
		if !found {
			return store.JobTxRowNotFound, nil, nil
		}
		var emp any
		if employeeID != "" {
			emp = string(employeeID)
		}
		nextProgress := progress
		if employeeID != "" {
			if progress == "Assign" {
				nextProgress = "In Progress"
			}
		} else if progress != "Complete" {
			nextProgress = "Assign"
		}
		if _, err := tx.Exec(ctx, `UPDATE job_rows SET employee_id=$2, progress=$3 WHERE id=$1`, string(rowID), emp, nextProgress); err != nil {
			return 0, nil, err
		}
		action, detail := "Unassigned", fmt.Sprintf("Row %s: Cleared employee assignment", label)
		if employeeID != "" {
			action, detail = "Assigned", fmt.Sprintf("Row %s: Employee: %s", label, j.personName(ctx, tx, employeeID))
		}
		return store.JobTxOK, []histEntry{{action, detail}}, nil
	})
}

func (j *jobs) RowSetQueue(ctx context.Context, uid, companyID, jobID, rowID store.ID, actor store.NoteActor, queue string) (store.Job, store.JobTxStatus, error) {
	return j.jobTx(ctx, uid, companyID, jobID, actor, func(tx pgx.Tx) (store.JobTxStatus, []histEntry, error) {
		_, label, prev, _, _, found, err := j.rowRef(ctx, tx, jobID, rowID)
		if err != nil {
			return 0, nil, err
		}
		if !found {
			return store.JobTxRowNotFound, nil, nil
		}
		if prev == queue {
			return store.JobTxOK, nil, nil
		}
		if _, err := tx.Exec(ctx, `UPDATE job_rows SET queue=$2, employee_id=NULL, progress='Assign' WHERE id=$1`, string(rowID), queue); err != nil {
			return 0, nil, err
		}
		return store.JobTxOK, []histEntry{{"Queue advanced", fmt.Sprintf("Row %s: %s → %s", label, prev, queue)}}, nil
	})
}

func (j *jobs) RowQueueOrder(ctx context.Context, uid, companyID, jobID, rowID store.ID, actor store.NoteActor, order []string) (store.Job, store.JobTxStatus, error) {
	return j.jobTx(ctx, uid, companyID, jobID, actor, func(tx pgx.Tx) (store.JobTxStatus, []histEntry, error) {
		_, label, _, _, _, found, err := j.rowRef(ctx, tx, jobID, rowID)
		if err != nil {
			return 0, nil, err
		}
		if !found {
			return store.JobTxRowNotFound, nil, nil
		}
		if _, err := tx.Exec(ctx, `UPDATE job_rows SET queue_order=$2 WHERE id=$1`, string(rowID), order); err != nil {
			return 0, nil, err
		}
		return store.JobTxOK, []histEntry{{"Queue reordered", fmt.Sprintf("Row %s: %s", label, strings.Join(order, " → "))}}, nil
	})
}

func (j *jobs) RowProgress(ctx context.Context, uid, companyID, jobID, rowID store.ID, actor store.NoteActor, progress string) (store.Job, store.JobTxStatus, error) {
	return j.jobTx(ctx, uid, companyID, jobID, actor, func(tx pgx.Tx) (store.JobTxStatus, []histEntry, error) {
		_, label, queue, prevProgress, hasEmp, found, err := j.rowRef(ctx, tx, jobID, rowID)
		if err != nil {
			return 0, nil, err
		}
		if !found {
			return store.JobTxRowNotFound, nil, nil
		}
		if progress != "Assign" && !hasEmp {
			return store.JobTxNeedsAssignee, nil, nil
		}
		hist := []histEntry{{"Progress changed", fmt.Sprintf("Row %s: %s → %s", label, prevProgress, progress)}}
		if progress == "Complete" && prevProgress != "Complete" {
			next, advanced := nextQueueStage(queue)
			nextQ := queue
			if advanced {
				nextQ = next
			}
			if _, err := tx.Exec(ctx, `UPDATE job_rows SET queue=$2, employee_id=NULL, progress='Assign' WHERE id=$1`, string(rowID), nextQ); err != nil {
				return 0, nil, err
			}
			if advanced {
				hist = append(hist, histEntry{"Queue advanced", fmt.Sprintf("Row %s: %s → %s", label, queue, nextQ)})
			}
		} else {
			if _, err := tx.Exec(ctx, `UPDATE job_rows SET progress=$2 WHERE id=$1`, string(rowID), progress); err != nil {
				return 0, nil, err
			}
		}
		return store.JobTxOK, hist, nil
	})
}

// nextQueueStage mirrors store.QueueStages advancement without exporting the internal helper.
func nextQueueStage(cur string) (string, bool) {
	stages := store.QueueStages
	for i, s := range stages {
		if s == cur {
			if i < len(stages)-1 {
				return stages[i+1], true
			}
			return "", false
		}
	}
	return "", false
}

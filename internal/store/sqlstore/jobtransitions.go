package sqlstore

import (
	"context"
	"fmt"
	"strings"
	"time"

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

func (j *jobs) ConvertToEntries(ctx context.Context, uid, companyID store.ID, jobIDs []store.ID, actor store.NoteActor) ([]store.Entry, []store.Job, bool, error) {
	ids := make([]string, len(jobIDs))
	for i, id := range jobIDs {
		ids[i] = string(id)
	}
	// Jobs (owner+company scoped) that have at least one Done, unconverted row.
	rows, err := j.pool.Query(ctx, `
		SELECT DISTINCT jb.id FROM jobs jb JOIN job_rows r ON r.job_id = jb.id
		 WHERE jb.id = ANY($1) AND jb.uid = $2 AND jb.company_id = $3 AND r.queue = 'Done' AND r.entry_id IS NULL`,
		ids, string(uid), string(companyID))
	if err != nil {
		return nil, nil, false, fmt.Errorf("convertible jobs: %w", err)
	}
	var jobList []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, nil, false, err
		}
		jobList = append(jobList, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, false, err
	}
	if len(jobList) == 0 {
		return nil, nil, false, nil
	}

	tx, err := j.pool.Begin(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	defer tx.Rollback(ctx)

	name := resolveActorName(ctx, j.pool, actor)
	type matInfo struct{ hsn, unit string }
	matCache := map[string]matInfo{}
	// One lookup per distinct product name across the batch - hsn and unit are both snapshotted
	// off the same Material row (like Node's materialFor).
	matFor := func(material string) (matInfo, error) {
		if material == "" {
			return matInfo{}, nil
		}
		if m, ok := matCache[material]; ok {
			return m, nil
		}
		var m matInfo
		err := tx.QueryRow(ctx, `SELECT hsn, unit FROM materials WHERE material_name=$1 AND company_id=$2 LIMIT 1`, material, string(companyID)).Scan(&m.hsn, &m.unit)
		if noRows(err) {
			m = matInfo{}
		} else if err != nil {
			return matInfo{}, err
		}
		matCache[material] = m
		return m, nil
	}

	var created []store.Entry
	for _, jobID := range jobList {
		var jobUID, challan, receivedDate string
		var clientID *string
		var jobTotal, jobAdvance float64
		if err := tx.QueryRow(ctx, `SELECT uid, client_id, COALESCE(to_char(received_date,'YYYY-MM-DD'),''), challan_number, total, advance FROM jobs WHERE id=$1`, jobID).
			Scan(&jobUID, &clientID, &receivedDate, &challan, &jobTotal, &jobAdvance); err != nil {
			return nil, nil, false, err
		}
		rr, err := tx.Query(ctx, `
			SELECT id, material, description, rate, qty, length, width, cgst, sgst, igst, discount, charges
			  FROM job_rows WHERE job_id=$1 AND queue='Done' AND entry_id IS NULL ORDER BY position ASC, id ASC`, jobID)
		if err != nil {
			return nil, nil, false, err
		}
		type pend struct {
			rowID string
			in    store.ConvertJobRow
		}
		var pending []pend
		for rr.Next() {
			var p pend
			if err := rr.Scan(&p.rowID, &p.in.Material, &p.in.Description, &p.in.Rate, &p.in.Qty, &p.in.Length, &p.in.Width,
				&p.in.Cgst, &p.in.Sgst, &p.in.Igst, &p.in.Discount, &p.in.Charges); err != nil {
				rr.Close()
				return nil, nil, false, err
			}
			pending = append(pending, p)
		}
		rr.Close()
		if err := rr.Err(); err != nil {
			return nil, nil, false, err
		}

		entryDate := receivedDate
		if entryDate == "" {
			entryDate = time.Now().UTC().Format("2006-01-02T15:04:05.000Z07:00")
		}
		converted := 0
		for _, p := range pending {
			mi, err := matFor(p.in.Material)
			if err != nil {
				return nil, nil, false, err
			}
			ce := store.ConvertRow(p.in, jobTotal, jobAdvance, mi.hsn, mi.unit, challan)
			var clientArg any
			if clientID != nil {
				clientArg = *clientID
			}
			var entryID string
			if err := tx.QueryRow(ctx, `
				INSERT INTO entries (uid, company_id, client_id, description, material, hsn, rate, qty, length, width, date, amount, cgst, sgst, igst, discount, charges, advance, total, unit, has_issued)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,false) RETURNING id`,
				jobUID, string(companyID), clientArg, ce.Description, ce.Material, ce.Hsn, ce.Rate, ce.Qty, ce.Length, ce.Width,
				entryDate, ce.Amount, ce.Cgst, ce.Sgst, ce.Igst, ce.Discount, ce.Charges, ce.Advance, ce.Total, ce.Unit).Scan(&entryID); err != nil {
				return nil, nil, false, fmt.Errorf("insert entry: %w", err)
			}
			if _, err := tx.Exec(ctx, `UPDATE job_rows SET entry_id=$2 WHERE id=$1`, p.rowID, entryID); err != nil {
				return nil, nil, false, err
			}
			created = append(created, store.Entry{
				ID: store.ID(entryID), Description: ce.Description, Material: ce.Material, Hsn: ce.Hsn, Unit: ce.Unit, Rate: ce.Rate, Qty: ce.Qty,
				Length: ce.Length, Width: ce.Width, Date: entryDate, Amount: ce.Amount, Cgst: ce.Cgst, Sgst: ce.Sgst, Igst: ce.Igst,
				Discount: ce.Discount, Charges: ce.Charges, Advance: ce.Advance, Total: ce.Total,
			})
			converted++
		}
		suffix := "s"
		if converted == 1 {
			suffix = ""
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO job_history (job_id, uid, company_id, actor_type, actor_id, actor_name, action, detail)
			VALUES ($1,$2,$3,$4,$5,$6,'Converted to Entry',$7)`,
			jobID, string(uid), string(companyID), actor.Role, string(actor.ActorID()), name,
			fmt.Sprintf("%d row%s converted to entries", converted, suffix)); err != nil {
			return nil, nil, false, err
		}
		if _, err := tx.Exec(ctx, `UPDATE jobs SET updated_at=now() WHERE id=$1`, jobID); err != nil {
			return nil, nil, false, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, false, err
	}

	// Re-read the updated jobs, populated.
	list, err := j.List(ctx, uid, companyID, "")
	if err != nil {
		return nil, nil, false, err
	}
	want := map[string]bool{}
	for _, id := range jobList {
		want[id] = true
	}
	var jobs []store.Job
	for _, job := range list {
		if want[string(job.ID)] {
			jobs = append(jobs, job)
		}
	}
	return created, jobs, true, nil
}

func (j *jobs) Unlock(ctx context.Context, uid, companyID, jobID store.ID, actor store.NoteActor, wanted bool) (store.Job, bool, store.JobTxStatus, error) {
	changed := false
	job, status, err := j.jobTx(ctx, uid, companyID, jobID, actor, func(tx pgx.Tx) (store.JobTxStatus, []histEntry, error) {
		var cur bool
		if err := tx.QueryRow(ctx, `SELECT unlocked FROM jobs WHERE id=$1`, string(jobID)).Scan(&cur); err != nil {
			return 0, nil, err
		}
		if cur == wanted {
			return store.JobTxOK, nil, nil // no change: no write, no history
		}
		changed = true
		if _, err := tx.Exec(ctx, `UPDATE jobs SET unlocked=$2 WHERE id=$1`, string(jobID), wanted); err != nil {
			return 0, nil, err
		}
		detail := "Re-locked"
		if wanted {
			detail = "Unlocked for editing"
		}
		return store.JobTxOK, []histEntry{{"Updated", detail}}, nil
	})
	return job, changed, status, err
}

// RowsCompleteAll marks every not-Done row Done in bulk, refusing (JobTxLocked) when the invoice
// lock forbids queue changes, and logging one STRUCTURED "Queue advanced" row per moved card.
// "Invoiced" here is the entry-based signal (a row's entry is has_issued) - the relational model
// has no invoices.job_ids array, so Node's isJobInvoiced part 1 does not apply.
func (j *jobs) RowsCompleteAll(ctx context.Context, uid, companyID, jobID store.ID, actor store.NoteActor) (store.Job, int, store.JobTxStatus, error) {
	moved := 0
	job, status, err := j.jobTx(ctx, uid, companyID, jobID, actor, func(tx pgx.Tx) (store.JobTxStatus, []histEntry, error) {
		var unlocked bool
		if err := tx.QueryRow(ctx, `SELECT unlocked FROM jobs WHERE id=$1`, string(jobID)).Scan(&unlocked); err != nil {
			return 0, nil, err
		}
		var invoiced bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM job_rows jr JOIN entries en ON en.id = jr.entry_id
				 WHERE jr.job_id = $1 AND jr.entry_id IS NOT NULL AND en.has_issued
			)`, string(jobID)).Scan(&invoiced); err != nil {
			return 0, nil, err
		}
		if invoiced && !unlocked {
			return store.JobTxLocked, nil, nil
		}

		type mr struct{ key, prev string }
		var moving []mr
		rows, err := tx.Query(ctx,
			`SELECT COALESCE(NULLIF(row_id,''), id), queue FROM job_rows WHERE job_id = $1 AND queue <> 'Done'`,
			string(jobID))
		if err != nil {
			return 0, nil, err
		}
		for rows.Next() {
			var m mr
			if err := rows.Scan(&m.key, &m.prev); err != nil {
				rows.Close()
				return 0, nil, err
			}
			moving = append(moving, m)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return 0, nil, err
		}
		moved = len(moving)
		if moved == 0 {
			return store.JobTxOK, nil, nil // every row already Done
		}
		if _, err := tx.Exec(ctx,
			`UPDATE job_rows SET queue='Done', employee_id=NULL, progress='Assign' WHERE job_id = $1 AND queue <> 'Done'`,
			string(jobID)); err != nil {
			return 0, nil, err
		}
		name := resolveActorName(ctx, j.pool, actor)
		for _, m := range moving {
			if _, err := tx.Exec(ctx, `
				INSERT INTO job_history (job_id, uid, company_id, actor_type, actor_id, actor_name, action, detail, from_stage, to_stage, row_key)
				VALUES ($1,$2,$3,$4,$5,$6,'Queue advanced',$7,$8,'Done',$9)`,
				string(jobID), string(uid), string(companyID), actor.Role, string(actor.ActorID()), name,
				fmt.Sprintf("Row %s: %s → Done", m.key, m.prev), m.prev, m.key); err != nil {
				return 0, nil, fmt.Errorf("log row history: %w", err)
			}
		}
		return store.JobTxOK, nil, nil
	})
	return job, moved, status, err
}

// Delete moves a job to Trash (source "Job") with a full snapshot, logs a "Trashed" history
// entry, then removes the job - all in one transaction. Reproduces refusedByLock("delete").
func (j *jobs) Delete(ctx context.Context, uid, companyID, jobID store.ID, actor store.NoteActor) (store.JobTxStatus, error) {
	tx, err := j.pool.Begin(ctx)
	if err != nil {
		return store.JobTxJobNotFound, err
	}
	defer tx.Rollback(ctx)

	var clientID *string
	var challan, received, queue, progress string
	var total, advance float64
	var employeeID, vendorID *string
	var queueOrder []string
	var unlocked bool
	err = tx.QueryRow(ctx, `
		SELECT client_id, challan_number, COALESCE(to_char(received_date,'YYYY-MM-DD'),''),
		       total, advance, queue, progress, employee_id, vendor_id, queue_order, unlocked
		  FROM jobs WHERE id=$1 AND uid=$2 AND company_id=$3`,
		string(jobID), string(uid), string(companyID)).
		Scan(&clientID, &challan, &received, &total, &advance, &queue, &progress, &employeeID, &vendorID, &queueOrder, &unlocked)
	if noRows(err) {
		return store.JobTxJobNotFound, nil
	}
	if err != nil {
		return store.JobTxJobNotFound, fmt.Errorf("looking up job: %w", err)
	}

	var invoiced bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM job_rows jr JOIN entries en ON en.id = jr.entry_id
			 WHERE jr.job_id=$1 AND jr.entry_id IS NOT NULL AND en.has_issued
		)`, string(jobID)).Scan(&invoiced); err != nil {
		return store.JobTxJobNotFound, err
	}
	if invoiced && !unlocked {
		return store.JobTxLocked, nil
	}

	var rowCount int
	var description string
	var rowsJSON []byte
	if err := tx.QueryRow(ctx, `
		SELECT count(*),
		       COALESCE(string_agg(NULLIF(COALESCE(NULLIF(description,''), material), ''), ', ' ORDER BY position), ''),
		       COALESCE(json_agg(json_build_object(
		           'material', material, 'description', description, 'hasDimensions', has_dimensions,
		           'length', length, 'width', width, 'qty', qty, 'rate', rate,
		           'cgst', cgst, 'sgst', sgst, 'igst', igst, 'discount', discount, 'charges', charges,
		           'quotation_id', quotation_id, 'entry_id', entry_id) ORDER BY position), '[]'::json)::text
		  FROM job_rows WHERE job_id=$1`, string(jobID)).
		Scan(&rowCount, &description, &rowsJSON); err != nil {
		return store.JobTxJobNotFound, fmt.Errorf("summarising rows: %w", err)
	}
	plural := "s"
	if rowCount == 1 {
		plural = ""
	}
	material := fmt.Sprintf("%d item%s", rowCount, plural)

	if _, err := tx.Exec(ctx, `
		INSERT INTO trash (uid, company_id, client_id, description, material, qty, date, amount, total, advance,
		                   source, challan_number, employee_id, vendor_id, queue, progress, queue_order, received_date, rows)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'Job',$11,$12,$13,$14,$15,$16,$17,$18::jsonb)`,
		string(uid), string(companyID), clientID, description, material, rowCount, received, total, total, advance,
		challan, employeeID, vendorID, queue, progress, queueOrder, received, string(rowsJSON)); err != nil {
		return store.JobTxJobNotFound, fmt.Errorf("insert trash: %w", err)
	}
	name := resolveActorName(ctx, j.pool, actor)
	if _, err := tx.Exec(ctx, `
		INSERT INTO job_history (job_id, uid, company_id, actor_type, actor_id, actor_name, action, detail)
		VALUES ($1,$2,$3,$4,$5,$6,'Trashed',$7)`,
		string(jobID), string(uid), string(companyID), actor.Role, string(actor.ActorID()), name,
		fmt.Sprintf("Moved to trash (job %s)", challan)); err != nil {
		return store.JobTxJobNotFound, fmt.Errorf("log history: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM jobs WHERE id=$1`, string(jobID)); err != nil {
		return store.JobTxJobNotFound, fmt.Errorf("delete job: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return store.JobTxJobNotFound, err
	}
	return store.JobTxOK, nil
}

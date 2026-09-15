package mongostore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type histEntry struct{ action, detail string }

// jobTx confirms the job exists (owner+company), runs fn, logs the history it returns, and
// re-reads the populated job. A non-OK status makes no history and no populate.
func (j *jobs) jobTx(ctx context.Context, uid, companyID, jobID store.ID, actor store.NoteActor,
	fn func(jobOID primitive.ObjectID) (store.JobTxStatus, []histEntry, error)) (store.Job, store.JobTxStatus, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return store.Job{}, store.JobTxJobNotFound, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.Job{}, store.JobTxJobNotFound, err
	}
	jobOID, err := objectID(jobID)
	if err != nil {
		return store.Job{}, store.JobTxJobNotFound, nil
	}
	n, err := j.db.Collection(colJobs).CountDocuments(ctx, bson.M{"_id": jobOID, "uid": uidOID, "company_id": companyOID})
	if err != nil {
		return store.Job{}, store.JobTxJobNotFound, fmt.Errorf("looking up job: %w", err)
	}
	if n == 0 {
		return store.Job{}, store.JobTxJobNotFound, nil
	}
	status, hist, err := fn(jobOID)
	if err != nil {
		return store.Job{}, store.JobTxJobNotFound, err
	}
	if status != store.JobTxOK {
		return store.Job{}, status, nil
	}
	if _, err := j.db.Collection(colJobs).UpdateByID(ctx, jobOID, bson.M{"$set": bson.M{"updatedAt": time.Now().UTC()}}); err != nil {
		return store.Job{}, store.JobTxJobNotFound, err
	}
	name := j.actorName(ctx, actor)
	actorID, _ := objectID(actor.ActorID())
	for _, h := range hist {
		if _, err := j.db.Collection(colJobHistories).InsertOne(ctx, bson.M{
			"job_id": jobOID, "uid": uidOID, "company_id": companyOID, "actorType": actor.Role,
			"actorId": actorID, "actorName": name, "action": h.action, "detail": h.detail, "createdAt": time.Now().UTC(), "__v": 0,
		}); err != nil {
			return store.Job{}, store.JobTxJobNotFound, fmt.Errorf("log history: %w", err)
		}
	}
	list, err := j.List(ctx, uid, companyID, "")
	if err != nil {
		return store.Job{}, store.JobTxJobNotFound, err
	}
	for _, job := range list {
		if job.ID == idOf(jobOID) {
			return job, store.JobTxOK, nil
		}
	}
	return store.Job{}, store.JobTxOK, nil
}

func (j *jobs) personName(ctx context.Context, personID store.ID) string {
	if personID == "" {
		return ""
	}
	if oid, err := objectID(personID); err == nil {
		var p struct {
			Name string `bson:"name"`
		}
		if j.db.Collection(colPersons).FindOne(ctx, bson.M{"_id": oid}).Decode(&p) == nil && p.Name != "" {
			return p.Name
		}
	}
	return "Unknown"
}

// jobDoc loads the mutable job-level state.
type jobStateDoc struct {
	EmployeeID *primitive.ObjectID `bson:"employee_id"`
	VendorID   *primitive.ObjectID `bson:"vendor_id"`
	Queue      string              `bson:"queue"`
	Progress   string              `bson:"progress"`
	Rows       []jobRowFull        `bson:"rows"`
}

func (j *jobs) loadState(ctx context.Context, jobOID primitive.ObjectID) (jobStateDoc, error) {
	var d jobStateDoc
	err := j.db.Collection(colJobs).FindOne(ctx, bson.M{"_id": jobOID}).Decode(&d)
	return d, err
}

func (j *jobs) Assign(ctx context.Context, uid, companyID, jobID store.ID, actor store.NoteActor, kind string, personID store.ID) (store.Job, store.JobTxStatus, error) {
	return j.jobTx(ctx, uid, companyID, jobID, actor, func(jobOID primitive.ObjectID) (store.JobTxStatus, []histEntry, error) {
		st, err := j.loadState(ctx, jobOID)
		if err != nil {
			return 0, nil, err
		}
		var pid *primitive.ObjectID
		if personID != "" {
			if oid, err := objectID(personID); err == nil {
				pid = &oid
			}
		}
		set := bson.M{}
		if kind == "employee" {
			set["employee_id"] = pid
			set["vendor_id"] = nil
		} else {
			set["vendor_id"] = pid
			set["employee_id"] = nil
		}
		hasAssignee := pid != nil
		if st.Progress != "Complete" {
			if hasAssignee {
				set["progress"] = "In Progress"
			} else {
				set["progress"] = "Unassigned"
			}
		}
		if _, err := j.db.Collection(colJobs).UpdateByID(ctx, jobOID, bson.M{"$set": set}); err != nil {
			return 0, nil, err
		}
		action, detail := "Unassigned", fmt.Sprintf("Cleared %s assignment", kind)
		if personID != "" {
			label := "Vendor"
			if kind == "employee" {
				label = "Employee"
			}
			action, detail = "Assigned", fmt.Sprintf("%s: %s", label, j.personName(ctx, personID))
		}
		return store.JobTxOK, []histEntry{{action, detail}}, nil
	})
}

func (j *jobs) Progress(ctx context.Context, uid, companyID, jobID store.ID, actor store.NoteActor, progress string) (store.Job, store.JobTxStatus, error) {
	return j.jobTx(ctx, uid, companyID, jobID, actor, func(jobOID primitive.ObjectID) (store.JobTxStatus, []histEntry, error) {
		st, err := j.loadState(ctx, jobOID)
		if err != nil {
			return 0, nil, err
		}
		hasAssignee := st.EmployeeID != nil || st.VendorID != nil
		if progress != "Unassigned" && !hasAssignee {
			return store.JobTxNeedsAssignee, nil, nil
		}
		if progress == "Unassigned" {
			if _, err := j.db.Collection(colJobs).UpdateByID(ctx, jobOID, bson.M{"$set": bson.M{"employee_id": nil, "vendor_id": nil, "progress": "Unassigned"}}); err != nil {
				return 0, nil, err
			}
			return store.JobTxOK, []histEntry{{"Progress changed", fmt.Sprintf("%s → Unassigned (assignment cleared)", st.Progress)}}, nil
		}
		if progress != "Complete" {
			if _, err := j.db.Collection(colJobs).UpdateByID(ctx, jobOID, bson.M{"$set": bson.M{"progress": progress}}); err != nil {
				return 0, nil, err
			}
			return store.JobTxOK, []histEntry{{"Progress changed", fmt.Sprintf("%s → %s", st.Progress, progress)}}, nil
		}
		next, advanced := nextStageM(st.Queue)
		if !advanced {
			next = st.Queue
		}
		if _, err := j.db.Collection(colJobs).UpdateByID(ctx, jobOID, bson.M{"$set": bson.M{"queue": next, "employee_id": nil, "vendor_id": nil, "progress": "Unassigned"}}); err != nil {
			return 0, nil, err
		}
		hist := []histEntry{}
		if advanced {
			hist = append(hist, histEntry{"Queue advanced", fmt.Sprintf("%s → %s", st.Queue, next)})
		}
		hist = append(hist, histEntry{"Progress changed", fmt.Sprintf("%s → Complete", st.Progress)})
		return store.JobTxOK, hist, nil
	})
}

func (j *jobs) SetQueue(ctx context.Context, uid, companyID, jobID store.ID, actor store.NoteActor, queue string) (store.Job, store.JobTxStatus, error) {
	return j.jobTx(ctx, uid, companyID, jobID, actor, func(jobOID primitive.ObjectID) (store.JobTxStatus, []histEntry, error) {
		st, err := j.loadState(ctx, jobOID)
		if err != nil {
			return 0, nil, err
		}
		if st.Queue == queue {
			return store.JobTxOK, nil, nil
		}
		if _, err := j.db.Collection(colJobs).UpdateByID(ctx, jobOID, bson.M{"$set": bson.M{"queue": queue, "employee_id": nil, "vendor_id": nil, "progress": "Unassigned"}}); err != nil {
			return 0, nil, err
		}
		return store.JobTxOK, []histEntry{{"Queue advanced", fmt.Sprintf("%s → %s", st.Queue, queue)}}, nil
	})
}

func (j *jobs) QueueOrder(ctx context.Context, uid, companyID, jobID store.ID, actor store.NoteActor, order []string) (store.Job, store.JobTxStatus, error) {
	return j.jobTx(ctx, uid, companyID, jobID, actor, func(jobOID primitive.ObjectID) (store.JobTxStatus, []histEntry, error) {
		if _, err := j.db.Collection(colJobs).UpdateByID(ctx, jobOID, bson.M{"$set": bson.M{"queueOrder": order}}); err != nil {
			return 0, nil, err
		}
		return store.JobTxOK, []histEntry{{"Queue reordered", strings.Join(order, " → ")}}, nil
	})
}

// mutateRow finds the row by id in the loaded state, applies change, and writes the whole rows
// array back. found=false when no row matches.
func (j *jobs) mutateRow(ctx context.Context, jobOID primitive.ObjectID, st jobStateDoc, rowID store.ID, change func(*jobRowFull)) (label string, found bool, err error) {
	rowOID, e := objectID(rowID)
	if e != nil {
		return "", false, nil
	}
	idx := -1
	for i := range st.Rows {
		if st.Rows[i].ID == rowOID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return "", false, nil
	}
	change(&st.Rows[idx])
	label = st.Rows[idx].RowID
	if label == "" {
		label = st.Rows[idx].ID.Hex()
	}
	if _, err := j.db.Collection(colJobs).UpdateByID(ctx, jobOID, bson.M{"$set": bson.M{"rows": st.Rows}}); err != nil {
		return "", false, err
	}
	return label, true, nil
}

func (j *jobs) RowAssign(ctx context.Context, uid, companyID, jobID, rowID store.ID, actor store.NoteActor, employeeID store.ID) (store.Job, store.JobTxStatus, error) {
	return j.jobTx(ctx, uid, companyID, jobID, actor, func(jobOID primitive.ObjectID) (store.JobTxStatus, []histEntry, error) {
		st, err := j.loadState(ctx, jobOID)
		if err != nil {
			return 0, nil, err
		}
		var empOID *primitive.ObjectID
		if employeeID != "" {
			if oid, err := objectID(employeeID); err == nil {
				empOID = &oid
			}
		}
		label, found, err := j.mutateRow(ctx, jobOID, st, rowID, func(r *jobRowFull) {
			r.EmployeeID = empOID
			if empOID != nil {
				if r.Progress == "Assign" {
					r.Progress = "In Progress"
				}
			} else if r.Progress != "Complete" {
				r.Progress = "Assign"
			}
		})
		if err != nil {
			return 0, nil, err
		}
		if !found {
			return store.JobTxRowNotFound, nil, nil
		}
		action, detail := "Unassigned", fmt.Sprintf("Row %s: Cleared employee assignment", label)
		if employeeID != "" {
			action, detail = "Assigned", fmt.Sprintf("Row %s: Employee: %s", label, j.personName(ctx, employeeID))
		}
		return store.JobTxOK, []histEntry{{action, detail}}, nil
	})
}

func (j *jobs) RowSetQueue(ctx context.Context, uid, companyID, jobID, rowID store.ID, actor store.NoteActor, queue string) (store.Job, store.JobTxStatus, error) {
	return j.jobTx(ctx, uid, companyID, jobID, actor, func(jobOID primitive.ObjectID) (store.JobTxStatus, []histEntry, error) {
		st, err := j.loadState(ctx, jobOID)
		if err != nil {
			return 0, nil, err
		}
		prev := ""
		label, found, err := j.mutateRow(ctx, jobOID, st, rowID, func(r *jobRowFull) {
			prev = r.Queue
			if prev != queue {
				r.Queue = queue
				r.EmployeeID = nil
				r.Progress = "Assign"
			}
		})
		if err != nil {
			return 0, nil, err
		}
		if !found {
			return store.JobTxRowNotFound, nil, nil
		}
		if prev == queue {
			return store.JobTxOK, nil, nil
		}
		return store.JobTxOK, []histEntry{{"Queue advanced", fmt.Sprintf("Row %s: %s → %s", label, prev, queue)}}, nil
	})
}

func (j *jobs) RowQueueOrder(ctx context.Context, uid, companyID, jobID, rowID store.ID, actor store.NoteActor, order []string) (store.Job, store.JobTxStatus, error) {
	return j.jobTx(ctx, uid, companyID, jobID, actor, func(jobOID primitive.ObjectID) (store.JobTxStatus, []histEntry, error) {
		st, err := j.loadState(ctx, jobOID)
		if err != nil {
			return 0, nil, err
		}
		label, found, err := j.mutateRow(ctx, jobOID, st, rowID, func(r *jobRowFull) { r.QueueOrder = order })
		if err != nil {
			return 0, nil, err
		}
		if !found {
			return store.JobTxRowNotFound, nil, nil
		}
		return store.JobTxOK, []histEntry{{"Queue reordered", fmt.Sprintf("Row %s: %s", label, strings.Join(order, " → "))}}, nil
	})
}

func (j *jobs) RowProgress(ctx context.Context, uid, companyID, jobID, rowID store.ID, actor store.NoteActor, progress string) (store.Job, store.JobTxStatus, error) {
	return j.jobTx(ctx, uid, companyID, jobID, actor, func(jobOID primitive.ObjectID) (store.JobTxStatus, []histEntry, error) {
		st, err := j.loadState(ctx, jobOID)
		if err != nil {
			return 0, nil, err
		}
		var prevProgress, prevQueue, nextQueue string
		var advanced, needsAssignee bool
		label, found, err := j.mutateRow(ctx, jobOID, st, rowID, func(r *jobRowFull) {
			prevProgress, prevQueue = r.Progress, r.Queue
			if progress != "Assign" && r.EmployeeID == nil {
				needsAssignee = true
				return
			}
			if progress == "Complete" && prevProgress != "Complete" {
				next, adv := nextStageM(r.Queue)
				if adv {
					r.Queue, advanced, nextQueue = next, true, next
				}
				r.EmployeeID = nil
				r.Progress = "Assign"
			} else {
				r.Progress = progress
			}
		})
		if err != nil {
			return 0, nil, err
		}
		if !found {
			return store.JobTxRowNotFound, nil, nil
		}
		if needsAssignee {
			return store.JobTxNeedsAssignee, nil, nil
		}
		hist := []histEntry{{"Progress changed", fmt.Sprintf("Row %s: %s → %s", label, prevProgress, progress)}}
		if advanced {
			hist = append(hist, histEntry{"Queue advanced", fmt.Sprintf("Row %s: %s → %s", label, prevQueue, nextQueue)})
		}
		return store.JobTxOK, hist, nil
	})
}

func nextStageM(cur string) (string, bool) {
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

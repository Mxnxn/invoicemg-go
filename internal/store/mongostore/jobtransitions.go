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

func (j *jobs) ConvertToEntries(ctx context.Context, uid, companyID store.ID, jobIDs []store.ID, actor store.NoteActor) ([]store.Entry, []store.Job, bool, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return nil, nil, false, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return nil, nil, false, err
	}
	oids := make([]primitive.ObjectID, 0, len(jobIDs))
	for _, id := range jobIDs {
		if oid, err := objectID(id); err == nil {
			oids = append(oids, oid)
		}
	}
	cur, err := j.db.Collection(colJobs).Find(ctx, bson.M{
		"_id": bson.M{"$in": oids}, "uid": uidOID, "company_id": companyOID,
		"rows": bson.M{"$elemMatch": bson.M{"queue": "Done", "entry_id": nil}},
	})
	if err != nil {
		return nil, nil, false, fmt.Errorf("convertible jobs: %w", err)
	}
	var jobs []struct {
		ID            primitive.ObjectID  `bson:"_id"`
		UID           primitive.ObjectID  `bson:"uid"`
		ClientID      *primitive.ObjectID `bson:"client_id"`
		ChallanNumber string              `bson:"challanNumber"`
		ReceivedDate  string              `bson:"receivedDate"`
		Total         float64             `bson:"total"`
		Advance       float64             `bson:"advance"`
		Rows          []jobRowFull        `bson:"rows"`
	}
	if err := cur.All(ctx, &jobs); err != nil {
		return nil, nil, false, err
	}
	if len(jobs) == 0 {
		return nil, nil, false, nil
	}

	name := j.actorName(ctx, actor)
	actorID, _ := objectID(actor.ActorID())
	hsnCache := map[string]string{}
	hsnFor := func(material string) string {
		if material == "" {
			return ""
		}
		if h, ok := hsnCache[material]; ok {
			return h
		}
		var m struct {
			Hsn string `bson:"hsn"`
		}
		_ = j.db.Collection(colMaterials).FindOne(ctx, bson.M{"material_name": material, "company_id": companyOID}).Decode(&m)
		hsnCache[material] = m.Hsn
		return m.Hsn
	}

	var created []store.Entry
	for _, job := range jobs {
		entryDate := job.ReceivedDate
		if entryDate == "" {
			entryDate = time.Now().UTC().Format("2006-01-02T15:04:05.000Z07:00")
		}
		converted := 0
		newRows := make([]jobRowFull, len(job.Rows))
		copy(newRows, job.Rows)
		for i := range newRows {
			row := newRows[i]
			if row.EntryID != nil || row.Queue != "Done" {
				continue
			}
			ce := store.ConvertRow(store.ConvertJobRow{
				Material: row.Material, Description: row.Description, Rate: row.Rate, Qty: row.Qty,
				Length: row.Length, Width: row.Width, Cgst: row.Cgst, Sgst: row.Sgst, Igst: row.Igst,
				Discount: row.Discount, Charges: row.Charges,
			}, job.Total, job.Advance, hsnFor(row.Material), job.ChallanNumber)
			entryOID := primitive.NewObjectID()
			now := time.Now().UTC()
			doc := bson.M{
				"_id": entryOID, "uid": job.UID, "company_id": companyOID, "client_id": job.ClientID,
				"description": ce.Description, "material": ce.Material, "hsn": ce.Hsn, "rate": ce.Rate, "qty": ce.Qty,
				"length": ce.Length, "width": ce.Width, "date": entryDate, "amount": ce.Amount,
				"cgst": ce.Cgst, "sgst": ce.Sgst, "igst": ce.Igst, "discount": ce.Discount, "charges": ce.Charges,
				"advance": ce.Advance, "total": ce.Total, "has_issued": false, "issued": nil,
				"createdAt": now, "updatedAt": now, "__v": 0,
			}
			if row.QuotationID != nil {
				doc["quotation_id"] = *row.QuotationID
			}
			if _, err := j.db.Collection(colEntries).InsertOne(ctx, doc); err != nil {
				return nil, nil, false, fmt.Errorf("insert entry: %w", err)
			}
			// Sheet + client linkage (Mongo reads these explicit arrays); match a sheet on the
			// normalized date, else create one.
			if err := j.linkEntryToSheet(ctx, companyOID, job.UID, entryDate, entryOID); err != nil {
				return nil, nil, false, err
			}
			if job.ClientID != nil {
				if _, err := j.db.Collection(colClients).UpdateByID(ctx, *job.ClientID, bson.M{"$push": bson.M{"entries": entryOID}}); err != nil {
					return nil, nil, false, err
				}
			}
			newRows[i].EntryID = &entryOID
			created = append(created, store.Entry{
				ID: idOf(entryOID), Description: ce.Description, Material: ce.Material, Hsn: ce.Hsn, Rate: ce.Rate, Qty: ce.Qty,
				Length: ce.Length, Width: ce.Width, Date: entryDate, Amount: ce.Amount, Cgst: ce.Cgst, Sgst: ce.Sgst, Igst: ce.Igst,
				Discount: ce.Discount, Charges: ce.Charges, Advance: ce.Advance, Total: ce.Total,
			})
			converted++
		}
		if _, err := j.db.Collection(colJobs).UpdateByID(ctx, job.ID, bson.M{"$set": bson.M{"rows": newRows, "updatedAt": time.Now().UTC()}}); err != nil {
			return nil, nil, false, err
		}
		suffix := "s"
		if converted == 1 {
			suffix = ""
		}
		if _, err := j.db.Collection(colJobHistories).InsertOne(ctx, bson.M{
			"job_id": job.ID, "uid": uidOID, "company_id": companyOID, "actorType": actor.Role,
			"actorId": actorID, "actorName": name, "action": "Converted to Entry",
			"detail": fmt.Sprintf("%d row%s converted to entries", converted, suffix), "createdAt": time.Now().UTC(), "__v": 0,
		}); err != nil {
			return nil, nil, false, err
		}
	}

	list, err := j.List(ctx, uid, companyID, "")
	if err != nil {
		return nil, nil, false, err
	}
	want := map[primitive.ObjectID]bool{}
	for _, jb := range jobs {
		want[jb.ID] = true
	}
	var populated []store.Job
	for _, job := range list {
		if oid, err := objectID(job.ID); err == nil && want[oid] {
			populated = append(populated, job)
		}
	}
	return created, populated, true, nil
}

// linkEntryToSheet pushes an entry onto the sheet for its date (matched on the normalized date),
// creating the sheet when none exists - the Mongo equivalent of routes/Entry's sheet linkage.
func (j *jobs) linkEntryToSheet(ctx context.Context, companyOID, uidOID primitive.ObjectID, date string, entryOID primitive.ObjectID) error {
	target := store.NormalizeDate(date)
	cur, err := j.db.Collection(colSheets).Find(ctx, bson.M{"company_id": companyOID})
	if err != nil {
		return err
	}
	var sheets []struct {
		ID   primitive.ObjectID `bson:"_id"`
		Date string             `bson:"date"`
	}
	if err := cur.All(ctx, &sheets); err != nil {
		return err
	}
	for _, s := range sheets {
		if store.NormalizeDate(s.Date) == target {
			_, err := j.db.Collection(colSheets).UpdateByID(ctx, s.ID, bson.M{"$push": bson.M{"entries": entryOID}})
			return err
		}
	}
	_, err = j.db.Collection(colSheets).InsertOne(ctx, bson.M{
		"date": date, "uid": uidOID, "company_id": companyOID, "entries": bson.A{entryOID}, "createdAt": time.Now().UTC(), "__v": 0,
	})
	return err
}

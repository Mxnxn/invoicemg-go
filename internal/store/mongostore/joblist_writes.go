package mongostore

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

func (j *jobs) ChallanNumbers(ctx context.Context, uid, companyID store.ID) ([]string, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return nil, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	cur, err := j.db.Collection(colJobs).Find(ctx, bson.M{"uid": uidOID, "company_id": companyOID},
		options.Find().SetProjection(bson.M{"challanNumber": 1}))
	if err != nil {
		return nil, fmt.Errorf("challan numbers: %w", err)
	}
	var docs []struct {
		N string `bson:"challanNumber"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.N)
	}
	return out, nil
}

func (j *jobs) ByEntry(ctx context.Context, uid, companyID, entryID store.ID) (store.Job, bool, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return store.Job{}, false, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.Job{}, false, err
	}
	entryOID, err := objectID(entryID)
	if err != nil {
		return store.Job{}, false, nil
	}
	var doc struct {
		ClientID interface{} `bson:"client_id"`
	}
	err = j.db.Collection(colJobs).FindOne(ctx, bson.M{"rows.entry_id": entryOID, "uid": uidOID, "company_id": companyOID},
		options.FindOne().SetProjection(bson.M{"client_id": 1})).Decode(&doc)
	if err != nil {
		return store.Job{}, false, nil
	}
	list, err := j.List(ctx, uid, companyID, "")
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

// actorName resolves the acting actor's display name (Helpers/Lifecycle actorName).
func (j *jobs) actorName(ctx context.Context, actor store.NoteActor) string {
	if actor.Role == "admin" {
		if oid, err := objectID(actor.UID); err == nil {
			var u struct {
				Name string `bson:"name"`
			}
			if j.db.Collection(colUsers).FindOne(ctx, bson.M{"_id": oid}).Decode(&u) == nil && u.Name != "" {
				return u.Name
			}
		}
		return "Admin"
	}
	if oid, err := objectID(actor.PersonID); err == nil {
		var p struct {
			Name string `bson:"name"`
		}
		if j.db.Collection(colPersons).FindOne(ctx, bson.M{"_id": oid}).Decode(&p) == nil && p.Name != "" {
			return p.Name
		}
	}
	return "Unknown"
}

func (j *jobs) Create(ctx context.Context, in store.JobCreateInput) (store.Job, bool, error) {
	uidOID, err := objectID(in.UID)
	if err != nil {
		return store.Job{}, false, err
	}
	companyOID, err := objectID(in.CompanyID)
	if err != nil {
		return store.Job{}, false, err
	}
	clientOID, err := objectID(in.ClientID)
	if err != nil {
		return store.Job{}, false, store.ErrBadID
	}

	rowDocs := bson.A{}
	for i, row := range in.Rows {
		rowDoc := bson.M{
			"_id": primitive.NewObjectID(), "rowId": fmt.Sprintf("%s-%06d", in.ChallanNumber, i+1),
			"material": row.Material, "description": row.Description, "length": row.Length, "width": row.Width,
			"qty": row.Qty, "rate": row.Rate, "cgst": row.Cgst, "sgst": row.Sgst, "igst": row.Igst,
			"discount": row.Discount, "charges": row.Charges, "queue": "Created", "progress": "Assign",
		}
		if row.QuotationID != "" {
			if qOID, err := objectID(row.QuotationID); err == nil {
				rowDoc["quotation_id"] = qOID
			}
		}
		rowDocs = append(rowDocs, rowDoc)
	}
	now := time.Now().UTC()
	doc := bson.M{
		"uid": uidOID, "company_id": companyOID, "client_id": clientOID,
		"challanNumber": in.ChallanNumber, "receivedDate": in.ReceivedDate, "rows": rowDocs,
		"total": in.Total, "advance": in.Advance, "queue": "Created", "progress": in.Progress,
		"createdAt": now, "updatedAt": now, "__v": 0,
	}
	if in.EmployeeID != "" {
		if oid, err := objectID(in.EmployeeID); err == nil {
			doc["employee_id"] = oid
		}
	}
	if in.VendorID != "" {
		if oid, err := objectID(in.VendorID); err == nil {
			doc["vendor_id"] = oid
		}
	}
	res, err := j.db.Collection(colJobs).InsertOne(ctx, doc)
	if isDuplicate(err) {
		return store.Job{}, true, nil
	}
	if err != nil {
		return store.Job{}, false, fmt.Errorf("insert job: %w", err)
	}
	jobOID, _ := res.InsertedID.(primitive.ObjectID)

	name := j.actorName(ctx, in.Actor)
	actorID, _ := objectID(in.Actor.ActorID())
	if _, err := j.db.Collection(colJobHistories).InsertOne(ctx, bson.M{
		"job_id": jobOID, "uid": uidOID, "company_id": companyOID, "actorType": in.Actor.Role,
		"actorId": actorID, "actorName": name, "action": "Created",
		"detail": fmt.Sprintf("Queue: Created, Progress: %s", in.Progress), "createdAt": now, "__v": 0,
	}); err != nil {
		return store.Job{}, false, fmt.Errorf("log history: %w", err)
	}

	list, err := j.List(ctx, in.UID, in.CompanyID, in.ClientID)
	if err != nil {
		return store.Job{}, false, err
	}
	for _, job := range list {
		if job.ID == idOf(jobOID) {
			return job, false, nil
		}
	}
	return store.Job{}, false, nil
}

func (j *jobs) Update(ctx context.Context, in store.JobUpdateInput) (store.Job, bool, bool, bool, error) {
	uidOID, err := objectID(in.UID)
	if err != nil {
		return store.Job{}, false, false, false, err
	}
	companyOID, err := objectID(in.CompanyID)
	if err != nil {
		return store.Job{}, false, false, false, err
	}
	jobOID, err := objectID(in.JobID)
	if err != nil {
		return store.Job{}, false, false, false, nil
	}
	scope := bson.M{"_id": jobOID, "uid": uidOID, "company_id": companyOID}
	var job struct {
		ChallanNumber string       `bson:"challanNumber"`
		Rows          []jobRowFull `bson:"rows"`
	}
	err = j.db.Collection(colJobs).FindOne(ctx, scope).Decode(&job)
	if err == mongo.ErrNoDocuments {
		return store.Job{}, false, false, false, nil
	}
	if err != nil {
		return store.Job{}, false, false, false, fmt.Errorf("looking up job: %w", err)
	}
	challan := job.ChallanNumber

	set := bson.M{"updatedAt": time.Now().UTC()}
	changed := []string{}
	if in.ClientID != nil {
		if oid, err := objectID(*in.ClientID); err == nil {
			set["client_id"] = oid
			changed = append(changed, "client_id")
		}
	}
	if in.ChallanNumber != nil {
		set["challanNumber"] = *in.ChallanNumber
		changed = append(changed, "challanNumber")
		challan = *in.ChallanNumber
	}
	if in.ReceivedDate != nil {
		set["receivedDate"] = *in.ReceivedDate
		changed = append(changed, "receivedDate")
	}
	if in.Advance != nil {
		set["advance"] = *in.Advance
		changed = append(changed, "advance")
	}

	if in.RowsSet {
		incomingByID := map[string]store.JobRowPatch{}
		matched := map[string]bool{}
		for _, r := range in.Rows {
			if r.ID != "" {
				incomingByID[string(r.ID)] = r
			}
		}
		existingIDs := map[string]bool{}
		for _, e := range job.Rows {
			existingIDs[e.ID.Hex()] = true
		}
		nextDocs := bson.A{}
		var total float64
		appendRow := func(id primitive.ObjectID, rowID string, material, description string, quotationID *primitive.ObjectID, p store.JobRowPricing, entryID *primitive.ObjectID, queue, progress string) {
			total += store.JobRowGrossTotal(p)
			doc := bson.M{
				"_id": id, "rowId": rowID, "material": material, "description": description,
				"length": p.Length, "width": p.Width, "qty": p.Qty, "rate": p.Rate,
				"cgst": p.Cgst, "sgst": p.Sgst, "igst": p.Igst, "discount": p.Discount, "charges": p.Charges,
				"queue": queue, "progress": progress,
			}
			if quotationID != nil {
				doc["quotation_id"] = *quotationID
			}
			if entryID != nil {
				doc["entry_id"] = *entryID
			}
			nextDocs = append(nextDocs, doc)
		}
		for _, e := range job.Rows {
			if e.EntryID != nil {
				appendRow(e.ID, e.RowID, e.Material, e.Description, e.QuotationID,
					store.JobRowPricing{Length: e.Length, Width: e.Width, Qty: e.Qty, Rate: e.Rate, Cgst: e.Cgst, Sgst: e.Sgst, Igst: e.Igst, Discount: e.Discount, Charges: e.Charges},
					e.EntryID, e.Queue, e.Progress)
				continue
			}
			inc, ok := incomingByID[e.ID.Hex()]
			if !ok {
				continue
			}
			matched[e.ID.Hex()] = true
			var quot *primitive.ObjectID
			if inc.QuotationID != "" {
				if q, err := objectID(inc.QuotationID); err == nil {
					quot = &q
				}
			}
			appendRow(e.ID, e.RowID, inc.Material, inc.Description, quot, patchPricing(inc), nil, e.Queue, e.Progress)
		}
		newCount := 0
		for _, inc := range in.Rows {
			if inc.ID != "" && existingIDs[string(inc.ID)] {
				continue
			}
			_ = newCount
			var quot *primitive.ObjectID
			if inc.QuotationID != "" {
				if q, err := objectID(inc.QuotationID); err == nil {
					quot = &q
				}
			}
			appendRow(primitive.NewObjectID(), "", inc.Material, inc.Description, quot, patchPricing(inc), nil, "Created", "Assign")
		}
		if len(nextDocs) == 0 {
			return store.Job{}, false, false, true, nil
		}
		// Assign rowIds by final position for rows that lack one.
		for i, d := range nextDocs {
			dm := d.(bson.M)
			if dm["rowId"] == "" {
				dm["rowId"] = fmt.Sprintf("%s-%06d", challan, i+1)
			}
		}
		set["rows"] = nextDocs
		set["total"] = total
		changed = append(changed, "rows")
	}

	if _, err := j.db.Collection(colJobs).UpdateOne(ctx, scope, bson.M{"$set": set}); err != nil {
		return store.Job{}, false, false, false, fmt.Errorf("update job: %w", err)
	}

	detail := "No fields changed"
	if len(changed) > 0 {
		detail = "Changed: " + joinComma(changed)
	}
	name := j.actorName(ctx, in.Actor)
	actorID, _ := objectID(in.Actor.ActorID())
	if _, err := j.db.Collection(colJobHistories).InsertOne(ctx, bson.M{
		"job_id": jobOID, "uid": uidOID, "company_id": companyOID, "actorType": in.Actor.Role,
		"actorId": actorID, "actorName": name, "action": "Updated", "detail": detail, "createdAt": time.Now().UTC(), "__v": 0,
	}); err != nil {
		return store.Job{}, false, false, false, fmt.Errorf("log history: %w", err)
	}

	list, err := j.List(ctx, in.UID, in.CompanyID, "")
	if err != nil {
		return store.Job{}, false, false, false, err
	}
	for _, job := range list {
		if job.ID == idOf(jobOID) {
			return job, true, false, false, nil
		}
	}
	return store.Job{}, true, false, false, nil
}

func patchPricing(r store.JobRowPatch) store.JobRowPricing {
	length := r.Length
	if length == "" {
		length = "1"
	}
	width := r.Width
	if width == "" {
		width = "1"
	}
	return store.JobRowPricing{Length: length, Width: width, Qty: r.Qty, Rate: r.Rate,
		Cgst: r.Cgst, Sgst: r.Sgst, Igst: r.Igst, Discount: r.Discount, Charges: r.Charges}
}

func joinComma(s []string) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += ", "
		}
		out += v
	}
	return out
}

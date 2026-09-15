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

type jobs struct{ db *mongo.Database }

func (s *Store) Jobs() store.Jobs { return &jobs{db: s.db} }

type jobRowFull struct {
	ID            primitive.ObjectID  `bson:"_id"`
	RowID         string              `bson:"rowId"`
	Material      string              `bson:"material"`
	Description   string              `bson:"description"`
	Qty           float64             `bson:"qty"`
	HasDimensions *bool               `bson:"hasDimensions"`
	Length        string              `bson:"length"`
	Width         string              `bson:"width"`
	Rate          float64             `bson:"rate"`
	Cgst          float64             `bson:"cgst"`
	Sgst          float64             `bson:"sgst"`
	Igst          float64             `bson:"igst"`
	Discount      float64             `bson:"discount"`
	Charges       float64             `bson:"charges"`
	Queue         string              `bson:"queue"`
	Progress      string              `bson:"progress"`
	QueueOrder    []string            `bson:"queueOrder"`
	EmployeeID    *primitive.ObjectID `bson:"employee_id"`
	QuotationID   *primitive.ObjectID `bson:"quotation_id"`
	EntryID       *primitive.ObjectID `bson:"entry_id"`
	CreatedAt     time.Time           `bson:"createdAt"`
	UpdatedAt     time.Time           `bson:"updatedAt"`
}

type alertChannelDoc struct {
	Status    string     `bson:"status"`
	StatusAt  *time.Time `bson:"statusAt"`
	Error     string     `bson:"error"`
	SentAt    *time.Time `bson:"sentAt"`
	Count     int        `bson:"count"`
	RowIDs    []string   `bson:"rowIds"`
	Signature string     `bson:"signature"`
}

type jobFull struct {
	ID            primitive.ObjectID  `bson:"_id"`
	ChallanNumber string              `bson:"challanNumber"`
	ReceivedDate  string              `bson:"receivedDate"`
	Total         float64             `bson:"total"`
	Advance       float64             `bson:"advance"`
	Queue         string              `bson:"queue"`
	Progress      string              `bson:"progress"`
	QueueOrder    []string            `bson:"queueOrder"`
	Unlocked      bool                `bson:"unlocked"`
	ClientID      *primitive.ObjectID `bson:"client_id"`
	EmployeeID    *primitive.ObjectID `bson:"employee_id"`
	VendorID      *primitive.ObjectID `bson:"vendor_id"`
	Rows          []jobRowFull        `bson:"rows"`
	Alerts        *struct {
		Created *alertChannelDoc `bson:"created"`
		Done    *alertChannelDoc `bson:"done"`
	} `bson:"alerts"`
	// Legacy top-level done-channel fields, the fallback readChannel uses.
	AlertStatus    string     `bson:"alertStatus"`
	AlertStatusAt  *time.Time `bson:"alertStatusAt"`
	AlertError     string     `bson:"alertError"`
	AlertSentAt    *time.Time `bson:"alertSentAt"`
	AlertCount     int        `bson:"alertCount"`
	AlertedRowIDs  []string   `bson:"alertedRowIds"`
	AlertSignature string     `bson:"alertSignature"`
	CreatedAt      time.Time  `bson:"createdAt"`
	UpdatedAt      time.Time  `bson:"updatedAt"`
	Version        int        `bson:"__v"`
}

func (j *jobs) List(ctx context.Context, uid, companyID, clientID store.ID) ([]store.Job, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return nil, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	filter := bson.M{"uid": uidOID, "company_id": companyOID}
	if clientID != "" {
		cOID, err := objectID(clientID)
		if err != nil {
			return nil, store.ErrBadID
		}
		filter["client_id"] = cOID
	}
	cur, err := j.db.Collection(colJobs).Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("listing jobs: %w", err)
	}
	defer cur.Close(ctx)
	var docs []jobFull
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("reading jobs: %w", err)
	}

	// Collect every id to populate, across all jobs, and resolve each set in ONE query (#6).
	personIDs := map[primitive.ObjectID]struct{}{}
	clientIDs := map[primitive.ObjectID]struct{}{}
	quotationIDs := map[primitive.ObjectID]struct{}{}
	entryIDs := map[primitive.ObjectID]struct{}{}
	for _, d := range docs {
		addID(personIDs, d.EmployeeID)
		addID(personIDs, d.VendorID)
		addID(clientIDs, d.ClientID)
		for _, r := range d.Rows {
			addID(personIDs, r.EmployeeID)
			addID(quotationIDs, r.QuotationID)
			addID(entryIDs, r.EntryID)
		}
	}
	persons, err := j.personNames(ctx, keys(personIDs))
	if err != nil {
		return nil, err
	}
	clients, err := j.jobClients(ctx, keys(clientIDs))
	if err != nil {
		return nil, err
	}
	quotations, err := j.quotationNumbers(ctx, keys(quotationIDs))
	if err != nil {
		return nil, err
	}
	entries, err := j.jobEntries(ctx, keys(entryIDs))
	if err != nil {
		return nil, err
	}
	invoiceByEntry, err := j.invoiceNumbersByEntry(ctx, companyOID, keys(entryIDs))
	if err != nil {
		return nil, err
	}

	out := make([]store.Job, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.toStore(persons, clients, quotations, entries, invoiceByEntry))
	}
	return out, nil
}

func addID(set map[primitive.ObjectID]struct{}, id *primitive.ObjectID) {
	if id != nil {
		set[*id] = struct{}{}
	}
}

func (j *jobs) personNames(ctx context.Context, ids []primitive.ObjectID) (map[primitive.ObjectID]string, error) {
	out := map[primitive.ObjectID]string{}
	if len(ids) == 0 {
		return out, nil
	}
	cur, err := j.db.Collection(colPersons).Find(ctx, bson.M{"_id": bson.M{"$in": ids}},
		options.Find().SetProjection(bson.M{"name": 1}))
	if err != nil {
		return nil, fmt.Errorf("populating people: %w", err)
	}
	defer cur.Close(ctx)
	var rows []struct {
		ID   primitive.ObjectID `bson:"_id"`
		Name string             `bson:"name"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ID] = r.Name
	}
	return out, nil
}

func (j *jobs) jobClients(ctx context.Context, ids []primitive.ObjectID) (map[primitive.ObjectID]store.JobClient, error) {
	out := map[primitive.ObjectID]store.JobClient{}
	if len(ids) == 0 {
		return out, nil
	}
	cur, err := j.db.Collection(colClients).Find(ctx, bson.M{"_id": bson.M{"$in": ids}},
		options.Find().SetProjection(bson.M{"clientName": 1, "clientFirm": 1, "clientPhone": 1, "clientAddress": 1, "notifyOnCreate": 1, "notifyOnUpdate": 1}))
	if err != nil {
		return nil, fmt.Errorf("populating job clients: %w", err)
	}
	defer cur.Close(ctx)
	var rows []struct {
		ID             primitive.ObjectID `bson:"_id"`
		ClientName     string             `bson:"clientName"`
		ClientFirm     string             `bson:"clientFirm"`
		ClientPhone    string             `bson:"clientPhone"`
		ClientAddress  string             `bson:"clientAddress"`
		NotifyOnCreate *bool              `bson:"notifyOnCreate"`
		NotifyOnUpdate *bool              `bson:"notifyOnUpdate"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ID] = store.JobClient{
			ID: idOf(r.ID), ClientName: r.ClientName, ClientFirm: r.ClientFirm, ClientPhone: r.ClientPhone,
			ClientAddress: r.ClientAddress, NotifyOnCreate: r.NotifyOnCreate, NotifyOnUpdate: r.NotifyOnUpdate,
		}
	}
	return out, nil
}

func (j *jobs) quotationNumbers(ctx context.Context, ids []primitive.ObjectID) (map[primitive.ObjectID]string, error) {
	out := map[primitive.ObjectID]string{}
	if len(ids) == 0 {
		return out, nil
	}
	cur, err := j.db.Collection(colQuotations).Find(ctx, bson.M{"_id": bson.M{"$in": ids}},
		options.Find().SetProjection(bson.M{"quotationNumber": 1}))
	if err != nil {
		return nil, fmt.Errorf("populating quotations: %w", err)
	}
	defer cur.Close(ctx)
	var rows []struct {
		ID              primitive.ObjectID `bson:"_id"`
		QuotationNumber string             `bson:"quotationNumber"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ID] = r.QuotationNumber
	}
	return out, nil
}

func (j *jobs) jobEntries(ctx context.Context, ids []primitive.ObjectID) (map[primitive.ObjectID]store.JobRowEntry, error) {
	out := map[primitive.ObjectID]store.JobRowEntry{}
	if len(ids) == 0 {
		return out, nil
	}
	cur, err := j.db.Collection(colEntries).Find(ctx, bson.M{"_id": bson.M{"$in": ids}},
		options.Find().SetProjection(bson.M{"has_issued": 1, "total": 1, "advance": 1}))
	if err != nil {
		return nil, fmt.Errorf("populating job entries: %w", err)
	}
	defer cur.Close(ctx)
	var rows []struct {
		ID        primitive.ObjectID `bson:"_id"`
		HasIssued bool               `bson:"has_issued"`
		Total     float64            `bson:"total"`
		Advance   float64            `bson:"advance"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ID] = store.JobRowEntry{ID: idOf(r.ID), HasIssued: r.HasIssued, Total: r.Total, Advance: r.Advance}
	}
	return out, nil
}

func (j *jobs) invoiceNumbersByEntry(ctx context.Context, companyOID primitive.ObjectID, entryIDs []primitive.ObjectID) (map[primitive.ObjectID]string, error) {
	out := map[primitive.ObjectID]string{}
	if len(entryIDs) == 0 {
		return out, nil
	}
	cur, err := j.db.Collection(colInvoices).Find(ctx,
		bson.M{"company_id": companyOID, "entries": bson.M{"$in": entryIDs}},
		options.Find().SetProjection(bson.M{"invoiceId": 1, "entries": 1}))
	if err != nil {
		return nil, fmt.Errorf("mapping invoices to entries: %w", err)
	}
	defer cur.Close(ctx)
	var rows []struct {
		InvoiceID string               `bson:"invoiceId"`
		Entries   []primitive.ObjectID `bson:"entries"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	for _, inv := range rows {
		for _, eid := range inv.Entries {
			out[eid] = inv.InvoiceID
		}
	}
	return out, nil
}

func chan_(c *alertChannelDoc) store.JobAlertChannel {
	if c == nil {
		return store.JobAlertChannel{}
	}
	return store.JobAlertChannel{
		Status: c.Status, StatusAt: c.StatusAt, Error: c.Error, SentAt: c.SentAt,
		Count: c.Count, RowIDs: c.RowIDs, Signature: c.Signature,
	}
}

func (d jobFull) toStore(persons map[primitive.ObjectID]string, clients map[primitive.ObjectID]store.JobClient,
	quotations map[primitive.ObjectID]string, entries map[primitive.ObjectID]store.JobRowEntry,
	invoiceByEntry map[primitive.ObjectID]string) store.Job {

	job := store.Job{
		ID: idOf(d.ID), ChallanNumber: d.ChallanNumber, ReceivedDate: d.ReceivedDate,
		Total: d.Total, Advance: d.Advance, Queue: d.Queue, Progress: d.Progress,
		QueueOrder: d.QueueOrder, Unlocked: d.Unlocked, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt, Version: d.Version,
	}
	if d.ClientID != nil {
		if c, ok := clients[*d.ClientID]; ok {
			job.Client = &c
		}
	}
	if d.EmployeeID != nil {
		job.Employee = &store.JobPerson{ID: idOf(*d.EmployeeID), Name: persons[*d.EmployeeID]}
	}
	if d.VendorID != nil {
		job.Vendor = &store.JobPerson{ID: idOf(*d.VendorID), Name: persons[*d.VendorID]}
	}
	// created channel: alerts.created only. done channel: alerts.done, falling back to the
	// legacy top-level fields (Helpers/JobAlertState.readChannel).
	if d.Alerts != nil {
		job.CreatedAlert = chan_(d.Alerts.Created)
		if d.Alerts.Done != nil {
			job.DoneAlert = chan_(d.Alerts.Done)
		}
	}
	if d.Alerts == nil || d.Alerts.Done == nil {
		job.DoneAlert = store.JobAlertChannel{
			Status: d.AlertStatus, StatusAt: d.AlertStatusAt, Error: d.AlertError, SentAt: d.AlertSentAt,
			Count: d.AlertCount, RowIDs: d.AlertedRowIDs, Signature: d.AlertSignature,
		}
	}

	job.Rows = make([]store.JobRow, 0, len(d.Rows))
	for _, r := range d.Rows {
		row := store.JobRow{
			ID: idOf(r.ID), RowID: r.RowID, Material: r.Material, Description: r.Description,
			Qty: r.Qty, HasDimensions: r.HasDimensions, Length: r.Length, Width: r.Width, Rate: r.Rate,
			Cgst: r.Cgst, Sgst: r.Sgst, Igst: r.Igst, Discount: r.Discount, Charges: r.Charges,
			Queue: r.Queue, Progress: r.Progress, QueueOrder: r.QueueOrder,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		}
		if r.EmployeeID != nil {
			row.Employee = &store.JobPerson{ID: idOf(*r.EmployeeID), Name: persons[*r.EmployeeID]}
		}
		if r.QuotationID != nil {
			row.Quotation = &store.JobRowQuotation{ID: idOf(*r.QuotationID), QuotationNumber: quotations[*r.QuotationID]}
		}
		if r.EntryID != nil {
			if e, ok := entries[*r.EntryID]; ok {
				row.Entry = &e
				row.EntryIssued = e.HasIssued
			}
			if num, ok := invoiceByEntry[*r.EntryID]; ok {
				row.InvoiceNumber = num
			}
		}
		job.Rows = append(job.Rows, row)
	}
	return job
}

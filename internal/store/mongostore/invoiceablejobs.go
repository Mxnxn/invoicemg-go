package mongostore

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

func (j *jobs) InvoiceableJobs(ctx context.Context, companyID, clientID store.ID) ([]store.InvoiceableJob, error) {
	companyOID, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	clientOID, err := objectID(clientID)
	if err != nil {
		return nil, store.ErrBadID
	}

	cur, err := j.db.Collection(colJobs).Find(ctx,
		bson.M{"client_id": clientOID, "company_id": companyOID},
		options.Find().
			SetSort(bson.D{{Key: "createdAt", Value: -1}}).
			SetLimit(200).
			SetProjection(bson.M{"challanNumber": 1, "receivedDate": 1, "total": 1, "queue": 1, "createdAt": 1,
				"rows.entry_id": 1, "rows.queue": 1, "rows.igst": 1}))
	if err != nil {
		return nil, fmt.Errorf("invoiceable jobs: %w", err)
	}
	var docs []struct {
		ID            primitive.ObjectID `bson:"_id"`
		ChallanNumber string             `bson:"challanNumber"`
		ReceivedDate  string             `bson:"receivedDate"`
		Total         float64            `bson:"total"`
		Queue         string             `bson:"queue"`
		CreatedAt     time.Time          `bson:"createdAt"`
		Rows          []struct {
			EntryID *primitive.ObjectID `bson:"entry_id"`
			Queue   string              `bson:"queue"`
			Igst    float64             `bson:"igst"`
		} `bson:"rows"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}

	// Batch the entry state (#6): amount + has_issued for every converted row.
	entryIDs := []primitive.ObjectID{}
	for _, d := range docs {
		for _, r := range d.Rows {
			if r.EntryID != nil {
				entryIDs = append(entryIDs, *r.EntryID)
			}
		}
	}
	type entryState struct {
		amount    float64
		hasIssued bool
	}
	entries := map[primitive.ObjectID]entryState{}
	if len(entryIDs) > 0 {
		ec, err := j.db.Collection(colEntries).Find(ctx, bson.M{"_id": bson.M{"$in": entryIDs}},
			options.Find().SetProjection(bson.M{"amount": 1, "has_issued": 1}))
		if err != nil {
			return nil, fmt.Errorf("invoiceable entries: %w", err)
		}
		var edocs []struct {
			ID        primitive.ObjectID `bson:"_id"`
			Amount    float64            `bson:"amount"`
			HasIssued bool               `bson:"has_issued"`
		}
		if err := ec.All(ctx, &edocs); err != nil {
			return nil, err
		}
		for _, e := range edocs {
			entries[e.ID] = entryState{amount: e.Amount, hasIssued: e.HasIssued}
		}
	}

	out := make([]store.InvoiceableJob, 0, len(docs))
	for _, d := range docs {
		date := d.ReceivedDate
		if date == "" {
			date = d.CreatedAt.Format("2006-01-02")
		}
		job := store.InvoiceableJob{ID: idOf(d.ID), ChallanNumber: d.ChallanNumber, ReceivedDate: date,
			Queue: d.Queue, Total: d.Total, Rows: []store.InvoiceableRow{}}
		for _, r := range d.Rows {
			row := store.InvoiceableRow{Queue: r.Queue, Igst: r.Igst}
			if r.EntryID != nil {
				row.EntryID = idOf(*r.EntryID)
				row.HasEntry = true
				if e, ok := entries[*r.EntryID]; ok {
					row.Amount = e.amount
					row.HasIssued = e.hasIssued
				}
			}
			job.Rows = append(job.Rows, row)
		}
		out = append(out, job)
	}
	return out, nil
}

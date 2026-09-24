package mongostore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

func (i *invoices) History(ctx context.Context, companyID, invoiceID store.ID) (store.InvoiceHistory, bool, error) {
	var out store.InvoiceHistory
	companyOID, err := objectID(companyID)
	if err != nil {
		return out, false, err
	}
	invOID, err := objectID(invoiceID)
	if err != nil {
		return out, false, nil
	}

	var invDoc struct {
		InvoiceID string               `bson:"invoiceId"`
		Entries   []primitive.ObjectID `bson:"entries"`
	}
	err = i.db.Collection(colInvoices).FindOne(ctx, bson.M{"_id": invOID, "company_id": companyOID},
		options.FindOne().SetProjection(bson.M{"invoiceId": 1, "entries": 1})).Decode(&invDoc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return out, false, nil
	}
	if err != nil {
		return out, false, fmt.Errorf("invoice history invoice: %w", err)
	}
	out.InvoiceNumber = invDoc.InvoiceID

	entries, err := i.entriesByID(ctx, invDoc.Entries)
	if err != nil {
		return out, false, err
	}
	for _, eid := range invDoc.Entries { // preserve the invoice's own entry order
		e, ok := entries[eid]
		if !ok {
			continue
		}
		out.Entries = append(out.Entries, store.InvoiceHistoryEntry{
			EntryID: e.ID, Date: e.Date, Material: e.Material, Description: e.Description,
			Qty: e.Qty, Rate: e.Rate, Amount: e.Amount, Advance: e.Advance,
			Cgst: e.Cgst, Sgst: e.Sgst, Igst: e.Igst, Discount: e.Discount, Charges: e.Charges,
			CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
		})
	}
	if len(invDoc.Entries) == 0 {
		return out, true, nil
	}

	// Jobs whose rows reference these entries.
	jc, err := i.db.Collection(colJobs).Find(ctx,
		bson.M{"company_id": companyOID, "rows.entry_id": bson.M{"$in": invDoc.Entries}},
		options.Find().SetProjection(bson.M{"challanNumber": 1}))
	if err != nil {
		return out, false, fmt.Errorf("invoice history jobs: %w", err)
	}
	var jobDocs []struct {
		ID            primitive.ObjectID `bson:"_id"`
		ChallanNumber string             `bson:"challanNumber"`
	}
	if err := jc.All(ctx, &jobDocs); err != nil {
		return out, false, err
	}
	jobOIDs := make([]primitive.ObjectID, 0, len(jobDocs))
	for _, j := range jobDocs {
		out.Jobs = append(out.Jobs, store.InvoiceHistoryJob{JobID: idOf(j.ID), ChallanNumber: j.ChallanNumber})
		jobOIDs = append(jobOIDs, j.ID)
	}
	if len(jobOIDs) == 0 {
		return out, true, nil
	}

	tc, err := i.db.Collection(colJobHistories).Find(ctx, bson.M{"job_id": bson.M{"$in": jobOIDs}},
		options.Find().
			SetProjection(bson.M{"job_id": 1, "action": 1, "detail": 1, "actorName": 1, "createdAt": 1}).
			SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(200))
	if err != nil {
		return out, false, fmt.Errorf("invoice history trail: %w", err)
	}
	var trailDocs []struct {
		JobID     primitive.ObjectID `bson:"job_id"`
		Action    string             `bson:"action"`
		Detail    string             `bson:"detail"`
		ActorName string             `bson:"actorName"`
		CreatedAt time.Time          `bson:"createdAt"`
	}
	if err := tc.All(ctx, &trailDocs); err != nil {
		return out, false, err
	}
	for _, d := range trailDocs {
		out.Trail = append(out.Trail, store.InvoiceHistoryTrail{
			At: d.CreatedAt, Action: d.Action, Detail: d.Detail, ActorName: d.ActorName, JobID: idOf(d.JobID),
		})
	}
	return out, true, nil
}

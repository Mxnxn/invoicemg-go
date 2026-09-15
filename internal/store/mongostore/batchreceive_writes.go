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

func (b *batchReceives) OpenJobs(ctx context.Context, uid, companyID, clientID store.ID) ([]store.BatchOpenJob, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return nil, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	clientOID, err := objectID(clientID)
	if err != nil {
		return nil, store.ErrBadID
	}
	filter := bson.M{"uid": uidOID, "company_id": companyOID, "client_id": clientOID,
		"$expr": bson.M{"$lt": bson.A{"$advance", "$total"}}}
	cur, err := b.db.Collection(colJobs).Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "receivedDate", Value: 1}, {Key: "createdAt", Value: 1}}).
			SetProjection(bson.M{"challanNumber": 1, "receivedDate": 1, "total": 1, "advance": 1, "rows": 1}))
	if err != nil {
		return nil, fmt.Errorf("open jobs: %w", err)
	}
	var docs []struct {
		ID            primitive.ObjectID `bson:"_id"`
		ChallanNumber string             `bson:"challanNumber"`
		ReceivedDate  string             `bson:"receivedDate"`
		Total         float64            `bson:"total"`
		Advance       float64            `bson:"advance"`
		Rows          []bson.Raw         `bson:"rows"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]store.BatchOpenJob, 0, len(docs))
	for _, d := range docs {
		out = append(out, store.BatchOpenJob{
			ID: idOf(d.ID), ChallanNumber: d.ChallanNumber, ReceivedDate: d.ReceivedDate,
			Total: d.Total, Advance: d.Advance, Remaining: brRound2(d.Total - d.Advance), EntryCount: len(d.Rows),
		})
	}
	return out, nil
}

// storedEntryAllocM is the persisted entryAllocations subdoc; entry_id is kept for reversal.
type storedEntryAllocM struct {
	EntryID   primitive.ObjectID  `bson:"entry_id"`
	InvoiceID *primitive.ObjectID `bson:"invoice_id"`
	Amount    float64             `bson:"amount"`
}

func (b *batchReceives) Create(ctx context.Context, uid, companyID store.ID, in store.BatchReceiveWrite) (store.BatchReceive, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return store.BatchReceive{}, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.BatchReceive{}, err
	}
	clientOID, err := objectID(in.ClientID)
	if err != nil {
		return store.BatchReceive{}, store.ErrBadID
	}

	allocs := bson.A{}
	entryAllocs := bson.A{}

	apply := func(jobID primitive.ObjectID, amount float64) error {
		if _, err := b.db.Collection(colJobs).UpdateByID(ctx, jobID, bson.M{"$inc": bson.M{"advance": amount}}); err != nil {
			return fmt.Errorf("apply job advance: %w", err)
		}
		ea, err := b.applyDownstream(ctx, jobID, amount, companyOID)
		if err != nil {
			return err
		}
		allocs = append(allocs, bson.M{"job_id": jobID, "amount": amount})
		for _, e := range ea {
			entryAllocs = append(entryAllocs, bson.M{"entry_id": e.EntryID, "invoice_id": e.InvoiceID, "amount": e.Amount})
		}
		return nil
	}

	if in.Mode == "auto" {
		cur, err := b.db.Collection(colJobs).Find(ctx,
			bson.M{"uid": uidOID, "company_id": companyOID, "client_id": clientOID, "$expr": bson.M{"$lt": bson.A{"$advance", "$total"}}},
			options.Find().SetSort(bson.D{{Key: "receivedDate", Value: 1}, {Key: "createdAt", Value: 1}}).
				SetProjection(bson.M{"total": 1, "advance": 1}))
		if err != nil {
			return store.BatchReceive{}, fmt.Errorf("auto jobs: %w", err)
		}
		var jobs []struct {
			ID      primitive.ObjectID `bson:"_id"`
			Total   float64            `bson:"total"`
			Advance float64            `bson:"advance"`
		}
		if err := cur.All(ctx, &jobs); err != nil {
			return store.BatchReceive{}, err
		}
		remaining := in.Amount
		for _, j := range jobs {
			if remaining <= 0 {
				break
			}
			gap := brRound2(j.Total - j.Advance)
			if gap <= 0 {
				continue
			}
			applied := gap
			if remaining < gap {
				applied = remaining
			}
			if err := apply(j.ID, applied); err != nil {
				return store.BatchReceive{}, err
			}
			remaining = brRound2(remaining - applied)
		}
	} else {
		for _, a := range in.Allocations {
			if a.JobID == "" || a.Amount <= 0 {
				continue
			}
			jobOID, err := objectID(a.JobID)
			if err != nil {
				continue
			}
			var exists struct {
				ID primitive.ObjectID `bson:"_id"`
			}
			err = b.db.Collection(colJobs).FindOne(ctx,
				bson.M{"_id": jobOID, "uid": uidOID, "company_id": companyOID, "client_id": clientOID}).Decode(&exists)
			if errors.Is(err, mongo.ErrNoDocuments) {
				continue
			}
			if err != nil {
				return store.BatchReceive{}, fmt.Errorf("manual job lookup: %w", err)
			}
			if err := apply(jobOID, a.Amount); err != nil {
				return store.BatchReceive{}, err
			}
		}
	}

	now := time.Now().UTC()
	doc := bson.M{
		"client": clientOID, "uid": uidOID, "company_id": companyOID, "date": in.Date, "amount": in.Amount,
		"note": in.Note, "mode": in.Mode, "allocations": allocs, "entryAllocations": entryAllocs,
		"createdAt": now, "updatedAt": now, "__v": 0,
	}
	if in.BankID != "" {
		if bankOID, err := objectID(in.BankID); err == nil {
			doc["bank_id"] = bankOID
		}
	}
	res, err := b.db.Collection(colBatchReceives).InsertOne(ctx, doc)
	if err != nil {
		return store.BatchReceive{}, fmt.Errorf("insert batch receive: %w", err)
	}
	oid, _ := res.InsertedID.(primitive.ObjectID)
	return b.getOne(ctx, uid, companyID, idOf(oid))
}

// applyDownstream mirrors applyJobPaymentDownstream: spread a job payment over its rows' linked
// entries (advance up, total down) and bump each invoiced entry's invoice amount.
func (b *batchReceives) applyDownstream(ctx context.Context, jobID primitive.ObjectID, applied float64, companyOID primitive.ObjectID) ([]storedEntryAllocM, error) {
	var job struct {
		Rows []jobRowFull `bson:"rows"`
	}
	if err := b.db.Collection(colJobs).FindOne(ctx, bson.M{"_id": jobID},
		options.FindOne().SetProjection(bson.M{"rows": 1})).Decode(&job); err != nil {
		return nil, fmt.Errorf("job rows: %w", err)
	}
	remaining := applied
	var out []storedEntryAllocM
	for _, row := range job.Rows {
		if remaining <= 0 {
			break
		}
		if row.EntryID == nil {
			continue
		}
		var entry struct {
			Total  float64             `bson:"total"`
			Issued *primitive.ObjectID `bson:"issued"`
		}
		err := b.db.Collection(colEntries).FindOne(ctx, bson.M{"_id": *row.EntryID, "company_id": companyOID}).Decode(&entry)
		if errors.Is(err, mongo.ErrNoDocuments) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("entry lookup: %w", err)
		}
		if entry.Total <= 0 {
			continue
		}
		applied2 := entry.Total
		if remaining < applied2 {
			applied2 = remaining
		}
		applied2 = brRound2(applied2)
		if applied2 <= 0 {
			continue
		}
		if _, err := b.db.Collection(colEntries).UpdateByID(ctx, *row.EntryID,
			bson.M{"$inc": bson.M{"advance": applied2, "total": -applied2}}); err != nil {
			return nil, fmt.Errorf("apply entry payment: %w", err)
		}
		if entry.Issued != nil {
			if _, err := b.db.Collection(colInvoices).UpdateByID(ctx, *entry.Issued,
				bson.M{"$inc": bson.M{"amount": applied2}}); err != nil {
				return nil, fmt.Errorf("apply invoice payment: %w", err)
			}
		}
		out = append(out, storedEntryAllocM{EntryID: *row.EntryID, InvoiceID: entry.Issued, Amount: applied2})
		remaining = brRound2(remaining - applied2)
	}
	return out, nil
}

func (b *batchReceives) Delete(ctx context.Context, uid, companyID, batchID store.ID) (bool, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return false, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return false, err
	}
	batchOID, err := objectID(batchID)
	if err != nil {
		return false, nil
	}
	var doc struct {
		Allocations []brAlloc           `bson:"allocations"`
		EntryAllocs []storedEntryAllocM `bson:"entryAllocations"`
	}
	err = b.db.Collection(colBatchReceives).FindOne(ctx, bson.M{"_id": batchOID, "uid": uidOID, "company_id": companyOID}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("looking up batch receive: %w", err)
	}
	for _, a := range doc.Allocations {
		if a.JobID == nil {
			continue
		}
		var job struct {
			Advance float64 `bson:"advance"`
		}
		if err := b.db.Collection(colJobs).FindOne(ctx, bson.M{"_id": *a.JobID, "company_id": companyOID}).Decode(&job); err != nil {
			continue
		}
		if _, err := b.db.Collection(colJobs).UpdateByID(ctx, *a.JobID,
			bson.M{"$set": bson.M{"advance": brRound2(maxF(0, job.Advance-a.Amount))}}); err != nil {
			return false, fmt.Errorf("reverse job advance: %w", err)
		}
	}
	for _, a := range doc.EntryAllocs {
		if !a.EntryID.IsZero() {
			var entry struct {
				Advance float64 `bson:"advance"`
				Total   float64 `bson:"total"`
			}
			if err := b.db.Collection(colEntries).FindOne(ctx, bson.M{"_id": a.EntryID, "company_id": companyOID}).Decode(&entry); err == nil {
				if _, err := b.db.Collection(colEntries).UpdateByID(ctx, a.EntryID, bson.M{"$set": bson.M{
					"advance": brRound2(maxF(0, entry.Advance-a.Amount)), "total": brRound2(entry.Total + a.Amount),
				}}); err != nil {
					return false, fmt.Errorf("reverse entry payment: %w", err)
				}
			}
		}
		if a.InvoiceID != nil {
			var invoice struct {
				Amount float64 `bson:"amount"`
			}
			if err := b.db.Collection(colInvoices).FindOne(ctx, bson.M{"_id": *a.InvoiceID, "company_id": companyOID}).Decode(&invoice); err == nil {
				if _, err := b.db.Collection(colInvoices).UpdateByID(ctx, *a.InvoiceID,
					bson.M{"$set": bson.M{"amount": brRound2(maxF(0, invoice.Amount-a.Amount))}}); err != nil {
					return false, fmt.Errorf("reverse invoice payment: %w", err)
				}
			}
		}
	}
	if _, err := b.db.Collection(colBatchReceives).DeleteOne(ctx, bson.M{"_id": batchOID}); err != nil {
		return false, fmt.Errorf("delete batch receive: %w", err)
	}
	return true, nil
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// getOne resolves one batch receive the way List does (client/bank populated, destinations).
func (b *batchReceives) getOne(ctx context.Context, uid, companyID, batchID store.ID) (store.BatchReceive, error) {
	list, err := b.List(ctx, uid, companyID, "")
	if err != nil {
		return store.BatchReceive{}, err
	}
	for _, br := range list {
		if br.ID == batchID {
			return br, nil
		}
	}
	return store.BatchReceive{}, fmt.Errorf("batch receive %s not found after create", batchID)
}

// CreateSimple is /client/batchUpdate: a plain logged receipt, no allocation, after confirming
// the client belongs to the caller (own company or a legacy null-company row).
func (b *batchReceives) CreateSimple(ctx context.Context, uid, companyID, clientID store.ID, amount float64, date, note string) (store.BatchReceive, bool, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return store.BatchReceive{}, false, nil
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.BatchReceive{}, false, nil
	}
	clientOID, err := objectID(clientID)
	if err != nil {
		return store.BatchReceive{}, false, nil
	}
	cnt, err := b.db.Collection(colClients).CountDocuments(ctx, bson.M{
		"_id": clientOID, "uid": uidOID,
		"$or": bson.A{bson.M{"company_id": companyOID}, bson.M{"company_id": nil}},
	})
	if err != nil {
		return store.BatchReceive{}, false, fmt.Errorf("verify client: %w", err)
	}
	if cnt == 0 {
		return store.BatchReceive{}, false, nil
	}

	now := time.Now().UTC()
	doc := bson.M{
		"client": clientOID, "uid": uidOID, "company_id": companyOID,
		"date": date, "amount": amount, "note": note, "createdAt": now, "updatedAt": now, "__v": 0,
	}
	res, err := b.db.Collection(colBatchReceives).InsertOne(ctx, doc)
	if err != nil {
		return store.BatchReceive{}, false, fmt.Errorf("insert batch receive: %w", err)
	}
	return store.BatchReceive{
		ID: idOf(res.InsertedID.(primitive.ObjectID)), UID: uid, CompanyID: companyID, ClientID: clientID,
		Date: date, Amount: amount, Note: note, CreatedAt: now, UpdatedAt: now,
	}, true, nil
}

func (b *batchReceives) UpdateSimple(ctx context.Context, companyID, batchID store.ID, amount *float64, note, date *string) (store.BatchReceive, bool, error) {
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.BatchReceive{}, false, nil
	}
	batchOID, err := objectID(batchID)
	if err != nil {
		return store.BatchReceive{}, false, nil
	}
	set := bson.M{"updatedAt": time.Now().UTC()}
	if amount != nil {
		set["amount"] = *amount
	}
	if note != nil {
		set["note"] = *note
	}
	if date != nil {
		set["date"] = *date
	}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var doc struct {
		ID        primitive.ObjectID  `bson:"_id"`
		UID       *primitive.ObjectID `bson:"uid"`
		CompanyID *primitive.ObjectID `bson:"company_id"`
		ClientID  *primitive.ObjectID `bson:"client"`
		Date      string              `bson:"date"`
		Amount    float64             `bson:"amount"`
		Note      string              `bson:"note"`
		Mode      string              `bson:"mode"`
		CreatedAt time.Time           `bson:"createdAt"`
		UpdatedAt time.Time           `bson:"updatedAt"`
		Version   int                 `bson:"__v"`
	}
	err = b.db.Collection(colBatchReceives).FindOneAndUpdate(ctx,
		bson.M{"_id": batchOID, "company_id": companyOID}, bson.M{"$set": set}, opts).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.BatchReceive{}, false, nil
	}
	if err != nil {
		return store.BatchReceive{}, false, fmt.Errorf("update batch receive: %w", err)
	}
	out := store.BatchReceive{ID: idOf(doc.ID), Date: doc.Date, Amount: doc.Amount, Note: doc.Note, Mode: doc.Mode, CreatedAt: doc.CreatedAt, UpdatedAt: doc.UpdatedAt, Version: doc.Version}
	if doc.UID != nil {
		out.UID = idOf(*doc.UID)
	}
	if doc.CompanyID != nil {
		out.CompanyID = idOf(*doc.CompanyID)
	}
	if doc.ClientID != nil {
		out.ClientID = idOf(*doc.ClientID)
	}
	return out, true, nil
}

func (b *batchReceives) DeleteSimple(ctx context.Context, companyID, batchID store.ID) (bool, error) {
	companyOID, err := objectID(companyID)
	if err != nil {
		return false, nil
	}
	batchOID, err := objectID(batchID)
	if err != nil {
		return false, nil
	}
	res, err := b.db.Collection(colBatchReceives).DeleteOne(ctx, bson.M{"_id": batchOID, "company_id": companyOID})
	if err != nil {
		return false, fmt.Errorf("delete batch receive: %w", err)
	}
	return res.DeletedCount > 0, nil
}

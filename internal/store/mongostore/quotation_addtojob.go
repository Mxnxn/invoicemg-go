package mongostore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/mxnxn/invoicemg-go/internal/docnumber"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

// AddRowToJob is routes/Quotation.js /row/add-to-job on Mongo: convert a quotation row into a
// job row, appending to the job already built from this quotation or creating a new one, then
// stamp the row's job_id and log a System note + history entry.
func (q *quotations) AddRowToJob(ctx context.Context, uid, companyID, quotationID, rowID store.ID) (store.QuotationRowToJobResult, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return store.QuotationRowToJobResult{}, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.QuotationRowToJobResult{}, err
	}
	quotationOID, err := objectID(quotationID)
	if err != nil {
		return store.QuotationRowToJobResult{Status: store.QRJQuotationNotFound}, nil
	}
	rowOID, err := objectID(rowID)
	if err != nil {
		return store.QuotationRowToJobResult{Status: store.QRJRowNotFound}, nil
	}

	var quote struct {
		ClientID        *primitive.ObjectID `bson:"client_id"`
		QuotationNumber string              `bson:"quotationNumber"`
		Rows            []quotationRowDoc   `bson:"rows"`
	}
	err = q.db.Collection(colQuotations).FindOne(ctx,
		bson.M{"_id": quotationOID, "uid": uidOID, "company_id": companyOID}).Decode(&quote)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.QuotationRowToJobResult{Status: store.QRJQuotationNotFound}, nil
	}
	if err != nil {
		return store.QuotationRowToJobResult{}, err
	}

	var row *quotationRowDoc
	for i := range quote.Rows {
		if quote.Rows[i].ID == rowOID {
			row = &quote.Rows[i]
			break
		}
	}
	if row == nil {
		return store.QuotationRowToJobResult{Status: store.QRJRowNotFound}, nil
	}
	if row.JobID != nil {
		return store.QuotationRowToJobResult{Status: store.QRJAlreadyAdded}, nil
	}

	rowTotal := store.JobRowGrossTotal(store.JobRowPricing{
		Length: row.Length, Width: row.Width, Qty: row.Qty, Rate: row.Rate,
		Cgst: row.Cgst, Sgst: row.Sgst, Discount: row.Discount, Charges: row.Charges,
	})

	newRow := func(rowID string) bson.M {
		return bson.M{
			"_id": primitive.NewObjectID(), "rowId": rowID,
			"material": row.Material, "description": row.Description, "length": row.Length, "width": row.Width,
			"qty": row.Qty, "rate": row.Rate, "cgst": row.Cgst, "sgst": row.Sgst, "igst": row.Igst,
			"discount": row.Discount, "charges": row.Charges, "queue": "Created", "progress": "Assign",
			"quotation_id": quotationOID,
		}
	}

	var jobOID primitive.ObjectID
	isNew := false

	var existing struct {
		ID            primitive.ObjectID `bson:"_id"`
		ChallanNumber string             `bson:"challanNumber"`
		Rows          []bson.Raw         `bson:"rows"`
	}
	err = q.db.Collection(colJobs).FindOne(ctx,
		bson.M{"uid": uidOID, "company_id": companyOID, "rows.quotation_id": quotationOID}).Decode(&existing)
	switch {
	case err == nil:
		jobOID = existing.ID
		rowID := fmt.Sprintf("%s-%06d", existing.ChallanNumber, len(existing.Rows)+1)
		if _, err := q.db.Collection(colJobs).UpdateByID(ctx, jobOID, bson.M{
			"$push": bson.M{"rows": newRow(rowID)},
			"$inc":  bson.M{"total": rowTotal},
			"$set":  bson.M{"updatedAt": time.Now().UTC()},
		}); err != nil {
			return store.QuotationRowToJobResult{}, fmt.Errorf("append job row: %w", err)
		}
	case errors.Is(err, mongo.ErrNoDocuments):
		isNew = true
		challans, cerr := (&jobs{db: q.db}).ChallanNumbers(ctx, uid, companyID)
		if cerr != nil {
			return store.QuotationRowToJobResult{}, cerr
		}
		challan := docnumber.Next(challans, "", time.Now().UTC())
		now := time.Now().UTC()
		doc := bson.M{
			"uid": uidOID, "company_id": companyOID, "challanNumber": challan,
			"rows": bson.A{newRow(challan + "-000001")}, "total": rowTotal,
			"queue": "Created", "progress": "Unassigned", "createdAt": now, "updatedAt": now, "__v": 0,
		}
		if quote.ClientID != nil {
			doc["client_id"] = *quote.ClientID
		}
		res, ierr := q.db.Collection(colJobs).InsertOne(ctx, doc)
		if isDuplicate(ierr) {
			return store.QuotationRowToJobResult{Status: store.QRJDupChallan}, nil
		}
		if ierr != nil {
			return store.QuotationRowToJobResult{}, fmt.Errorf("insert job: %w", ierr)
		}
		jobOID = res.InsertedID.(primitive.ObjectID)
	default:
		return store.QuotationRowToJobResult{}, err
	}

	if _, err := q.db.Collection(colQuotations).UpdateOne(ctx,
		bson.M{"_id": quotationOID, "rows._id": rowOID},
		bson.M{"$set": bson.M{"rows.$.job_id": jobOID, "rows.$.updatedAt": time.Now().UTC()}}); err != nil {
		return store.QuotationRowToJobResult{}, fmt.Errorf("stamp row job_id: %w", err)
	}

	detail := fmt.Sprintf("Row added from Quote number %s", quote.QuotationNumber)
	action := "Updated"
	if isNew {
		detail = fmt.Sprintf("Created using Quote number %s", quote.QuotationNumber)
		action = "Created"
	}
	now := time.Now().UTC()
	if _, err := q.db.Collection(colJobNotes).InsertOne(ctx, bson.M{
		"job_id": jobOID, "uid": uidOID, "company_id": companyOID,
		"authorType": "system", "authorId": uidOID, "authorName": "System", "text": detail,
		"createdAt": now, "updatedAt": now, "__v": 0,
	}); err != nil {
		return store.QuotationRowToJobResult{}, fmt.Errorf("log note: %w", err)
	}
	if _, err := q.db.Collection(colJobHistories).InsertOne(ctx, bson.M{
		"job_id": jobOID, "uid": uidOID, "company_id": companyOID,
		"actorType": "system", "actorId": uidOID, "actorName": "System", "action": action, "detail": detail,
		"createdAt": now, "__v": 0,
	}); err != nil {
		return store.QuotationRowToJobResult{}, fmt.Errorf("log history: %w", err)
	}

	quoteOut, _, err := q.Get(ctx, uid, companyID, quotationID)
	if err != nil {
		return store.QuotationRowToJobResult{}, err
	}
	list, err := (&jobs{db: q.db}).List(ctx, uid, companyID, "")
	if err != nil {
		return store.QuotationRowToJobResult{}, err
	}
	var job store.Job
	for _, jb := range list {
		if string(jb.ID) == jobOID.Hex() {
			job = jb
			break
		}
	}
	return store.QuotationRowToJobResult{Status: store.QRJOk, IsNew: isNew, Quotation: quoteOut, Job: job}, nil
}

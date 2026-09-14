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

const (
	colClients    = "clients"
	colJobReviews = "jobreviews"
)

type alerts struct{ db *mongo.Database }

func (s *Store) Alerts() store.Alerts { return &alerts{db: s.db} }

// jobRowDoc mirrors the JobRow subdocument in Model/Job.js. HasDimensions is a pointer so an
// absent field stays absent (nil), which jobmath reads as by-dimension - decoding it as a
// plain bool would turn every pre-existing row into a by-quantity one and mis-price the job.
type jobRowDoc struct {
	ID            primitive.ObjectID `bson:"_id"`
	RowID         string             `bson:"rowId"`
	Description   string             `bson:"description"`
	Material      string             `bson:"material"`
	Qty           float64            `bson:"qty"`
	HasDimensions *bool              `bson:"hasDimensions"`
	Length        string             `bson:"length"`
	Width         string             `bson:"width"`
	Rate          float64            `bson:"rate"`
	Cgst          float64            `bson:"cgst"`
	Sgst          float64            `bson:"sgst"`
	Igst          float64            `bson:"igst"`
	Discount      float64            `bson:"discount"`
	Charges       float64            `bson:"charges"`
	Queue         string             `bson:"queue"`
	Progress      string             `bson:"progress"`
}

// jobDoc is the subset of Model/Job.js the public alert page needs. client_id/company_id/uid
// are pointers because company_id is nullable on a job and the others may be absent on very
// old data - a plain ObjectID would decode a missing value as the zero id.
type jobDoc struct {
	ID            primitive.ObjectID  `bson:"_id"`
	ChallanNumber string              `bson:"challanNumber"`
	ReceivedDate  string              `bson:"receivedDate"`
	Queue         string              `bson:"queue"`
	ClientID      *primitive.ObjectID `bson:"client_id"`
	CompanyID     *primitive.ObjectID `bson:"company_id"`
	UID           *primitive.ObjectID `bson:"uid"`
	CreatedAt     *time.Time          `bson:"createdAt"`
	Rows          []jobRowDoc         `bson:"rows"`
}

func (d jobDoc) toStore() store.AlertJob {
	job := store.AlertJob{
		ID:            idOf(d.ID),
		ChallanNumber: d.ChallanNumber,
		ReceivedDate:  d.ReceivedDate,
		Queue:         d.Queue,
		CreatedAt:     d.CreatedAt,
	}
	if d.ClientID != nil {
		job.ClientID = idOf(*d.ClientID)
	}
	if d.CompanyID != nil {
		job.CompanyID = idOf(*d.CompanyID)
	}
	if d.UID != nil {
		job.UID = idOf(*d.UID)
	}
	// make, not nil: an empty rows array must marshal to [], the same shape Node sends, and a
	// nil slice would become null. (See httpx envelope #22.)
	job.Rows = make([]store.AlertRow, 0, len(d.Rows))
	for _, r := range d.Rows {
		job.Rows = append(job.Rows, store.AlertRow{
			ID:            idOf(r.ID),
			RowID:         r.RowID,
			Description:   r.Description,
			Material:      r.Material,
			Qty:           r.Qty,
			HasDimensions: r.HasDimensions,
			Length:        r.Length,
			Width:         r.Width,
			Rate:          r.Rate,
			Cgst:          r.Cgst,
			Sgst:          r.Sgst,
			Igst:          r.Igst,
			Discount:      r.Discount,
			Charges:       r.Charges,
			Queue:         r.Queue,
			Progress:      r.Progress,
		})
	}
	return job
}

func (a *alerts) Job(ctx context.Context, id store.ID) (store.AlertJob, error) {
	oid, err := objectID(id)
	if err != nil {
		return store.AlertJob{}, store.ErrBadID
	}
	var doc jobDoc
	err = a.db.Collection(colJobs).FindOne(ctx, bson.M{"_id": oid}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.AlertJob{}, store.ErrNotFound
	}
	if err != nil {
		return store.AlertJob{}, fmt.Errorf("looking up job: %w", err)
	}
	return doc.toStore(), nil
}

func (a *alerts) Client(ctx context.Context, id store.ID) (store.AlertClient, error) {
	oid, err := objectID(id)
	if err != nil {
		// A missing or malformed client id is "no client", not a fault: Node reads it through
		// optional chaining and renders blank. ErrNotFound, never ErrBadID.
		return store.AlertClient{}, store.ErrNotFound
	}
	var doc struct {
		ClientName string `bson:"clientName"`
		ClientFirm string `bson:"clientFirm"`
	}
	err = a.db.Collection(colClients).FindOne(ctx, bson.M{"_id": oid},
		options.FindOne().SetProjection(bson.M{"clientName": 1, "clientFirm": 1})).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.AlertClient{}, store.ErrNotFound
	}
	if err != nil {
		return store.AlertClient{}, fmt.Errorf("looking up client: %w", err)
	}
	return store.AlertClient{Name: doc.ClientName, Firm: doc.ClientFirm}, nil
}

func (a *alerts) Company(ctx context.Context, id store.ID) (store.AlertCompany, error) {
	oid, err := objectID(id)
	if err != nil {
		// company_id is nullable on a job, so an empty id is the ordinary case, not a fault.
		return store.AlertCompany{}, store.ErrNotFound
	}
	var doc struct {
		Name    string `bson:"name"`
		Firm    string `bson:"firm"`
		Phone   string `bson:"phone"`
		URL     string `bson:"url"`
		Address string `bson:"address"`
		Gst     string `bson:"gst"`
	}
	err = a.db.Collection(colCompanies).FindOne(ctx, bson.M{"_id": oid},
		options.FindOne().SetProjection(bson.M{"name": 1, "firm": 1, "phone": 1, "url": 1, "address": 1, "gst": 1})).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.AlertCompany{}, store.ErrNotFound
	}
	if err != nil {
		return store.AlertCompany{}, fmt.Errorf("looking up company: %w", err)
	}
	return store.AlertCompany{Name: doc.Name, Firm: doc.Firm, Phone: doc.Phone, URL: doc.URL, Address: doc.Address, Gst: doc.Gst}, nil
}

func (a *alerts) Review(ctx context.Context, jobID store.ID) (store.AlertReview, error) {
	oid, err := objectID(jobID)
	if err != nil {
		// The handler has already guarded the id shape; anything unparseable here just has no
		// review. Not-reviewed, not a fault.
		return store.AlertReview{}, store.ErrNotFound
	}
	// Projection mirrors Node's .select(["scores","comment","createdAt"]): _id is kept, __v is
	// dropped (inclusion projection), which is why the response carries no __v (#21).
	var doc struct {
		ID     primitive.ObjectID `bson:"_id"`
		Scores struct {
			Quality       int `bson:"quality"`
			Speed         int `bson:"speed"`
			Communication int `bson:"communication"`
			Satisfaction  int `bson:"satisfaction"`
			Overall       int `bson:"overall"`
		} `bson:"scores"`
		Comment   string    `bson:"comment"`
		CreatedAt time.Time `bson:"createdAt"`
	}
	err = a.db.Collection(colJobReviews).FindOne(ctx, bson.M{"job_id": oid},
		options.FindOne().SetProjection(bson.M{"scores": 1, "comment": 1, "createdAt": 1})).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.AlertReview{}, store.ErrNotFound
	}
	if err != nil {
		return store.AlertReview{}, fmt.Errorf("looking up review: %w", err)
	}
	return store.AlertReview{
		ID: idOf(doc.ID),
		Scores: store.ReviewScores{
			Quality:       doc.Scores.Quality,
			Speed:         doc.Scores.Speed,
			Communication: doc.Scores.Communication,
			Satisfaction:  doc.Scores.Satisfaction,
			Overall:       doc.Scores.Overall,
		},
		Comment:   doc.Comment,
		CreatedAt: doc.CreatedAt,
	}, nil
}

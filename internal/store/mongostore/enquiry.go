package mongostore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/mxnxn/invoicemg-go/internal/store"
	"github.com/mxnxn/invoicemg-go/internal/textcase"
)

const colEnquiries = "enquiries"

type enquiries struct{ db *mongo.Database }

func (s *Store) Enquiries() store.Enquiries { return &enquiries{db: s.db} }

func capLen(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func (e *enquiries) Create(ctx context.Context, in store.EnquiryWrite) (store.ID, error) {
	note := in.Note
	if len(note) > 2000 {
		note = note[:2000]
	}
	now := time.Now().UTC()
	doc := bson.M{
		"name": capLen(textcase.TitleCase(in.Name), 120), "email": strings.ToLower(in.Email), "phone": in.Phone,
		"companyName": capLen(textcase.TitleCase(in.CompanyName), 160), "note": textcase.SentenceCase(note),
		"source": in.Source, "userAgent": in.UserAgent, "handled": false, "createdAt": now, "updatedAt": now, "__v": 0,
	}
	res, err := e.db.Collection(colEnquiries).InsertOne(ctx, doc)
	if err != nil {
		return "", fmt.Errorf("insert enquiry: %w", err)
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		return idOf(oid), nil
	}
	return "", nil
}

func (e *enquiries) List(ctx context.Context) ([]store.Enquiry, error) {
	cur, err := e.db.Collection(colEnquiries).Find(ctx, bson.M{},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(200))
	if err != nil {
		return nil, fmt.Errorf("list enquiries: %w", err)
	}
	var docs []struct {
		ID          primitive.ObjectID `bson:"_id"`
		Name        string             `bson:"name"`
		Email       string             `bson:"email"`
		Phone       string             `bson:"phone"`
		CompanyName string             `bson:"companyName"`
		Note        string             `bson:"note"`
		Source      string             `bson:"source"`
		UserAgent   string             `bson:"userAgent"`
		Handled     bool               `bson:"handled"`
		CreatedAt   time.Time          `bson:"createdAt"`
		UpdatedAt   time.Time          `bson:"updatedAt"`
		Version     int                `bson:"__v"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]store.Enquiry, 0, len(docs))
	for _, d := range docs {
		out = append(out, store.Enquiry{
			ID: idOf(d.ID), Name: d.Name, Email: d.Email, Phone: d.Phone, CompanyName: d.CompanyName,
			Note: d.Note, Source: d.Source, UserAgent: d.UserAgent, Handled: d.Handled,
			CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt, Version: d.Version,
		})
	}
	return out, nil
}

func (e *enquiries) SetHandled(ctx context.Context, id store.ID, handled bool) error {
	oid, err := objectID(id)
	if err != nil {
		return nil
	}
	if _, err := e.db.Collection(colEnquiries).UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$set": bson.M{"handled": handled, "updatedAt": time.Now().UTC()}}); err != nil {
		return fmt.Errorf("set enquiry handled: %w", err)
	}
	return nil
}

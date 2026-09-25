package mongostore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

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

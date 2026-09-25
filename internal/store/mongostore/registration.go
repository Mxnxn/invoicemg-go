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
	colRegistrationTokens = "registrationtokens"
	colPasswordResets     = "passwordresetrequests"
)

type registrationTokens struct{ db *mongo.Database }

func (s *Store) RegistrationTokens() store.RegistrationTokens { return &registrationTokens{db: s.db} }

type regTokenDoc struct {
	ID        primitive.ObjectID  `bson:"_id"`
	Token     string              `bson:"token"`
	ExpiresAt time.Time           `bson:"expiresAt"`
	UsedAt    *time.Time          `bson:"usedAt"`
	UsedBy    *primitive.ObjectID `bson:"usedBy"`
	CreatedAt time.Time           `bson:"createdAt"`
}

func (d regTokenDoc) toStore() store.RegistrationToken {
	t := store.RegistrationToken{ID: idOf(d.ID), Token: d.Token, ExpiresAt: d.ExpiresAt, UsedAt: d.UsedAt, CreatedAt: d.CreatedAt}
	if d.UsedBy != nil {
		t.UsedBy = idOf(*d.UsedBy)
	}
	return t
}

func (r *registrationTokens) FindByToken(ctx context.Context, token string) (store.RegistrationToken, bool, error) {
	var d regTokenDoc
	err := r.db.Collection(colRegistrationTokens).FindOne(ctx, bson.M{"token": token}).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.RegistrationToken{}, false, nil
	}
	if err != nil {
		return store.RegistrationToken{}, false, fmt.Errorf("find registration token: %w", err)
	}
	return d.toStore(), true, nil
}

func (r *registrationTokens) MarkUsed(ctx context.Context, id, uid store.ID) error {
	oid, err := objectID(id)
	if err != nil {
		return err
	}
	usedBy := optionalOID(uid)
	_, err = r.db.Collection(colRegistrationTokens).UpdateOne(ctx, bson.M{"_id": oid},
		bson.M{"$set": bson.M{"usedAt": time.Now().UTC(), "usedBy": usedBy, "updatedAt": time.Now().UTC()}})
	return err
}

func (r *registrationTokens) Create(ctx context.Context, token string, expiresAt time.Time) (store.RegistrationToken, error) {
	now := time.Now().UTC()
	doc := bson.M{"token": token, "expiresAt": expiresAt, "usedAt": nil, "usedBy": nil, "createdAt": now, "updatedAt": now, "__v": 0}
	res, err := r.db.Collection(colRegistrationTokens).InsertOne(ctx, doc)
	if err != nil {
		return store.RegistrationToken{}, fmt.Errorf("create registration token: %w", err)
	}
	t := store.RegistrationToken{Token: token, ExpiresAt: expiresAt, CreatedAt: now}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		t.ID = idOf(oid)
	}
	return t, nil
}

func (r *registrationTokens) List(ctx context.Context) ([]store.RegistrationToken, error) {
	cur, err := r.db.Collection(colRegistrationTokens).Find(ctx, bson.M{},
		options.Find().SetSort(bson.D{{Key: "_id", Value: -1}}).SetLimit(25))
	if err != nil {
		return nil, fmt.Errorf("list registration tokens: %w", err)
	}
	var docs []regTokenDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]store.RegistrationToken, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.toStore())
	}
	return out, nil
}

type passwordResetRequests struct{ db *mongo.Database }

func (s *Store) PasswordResetRequests() store.PasswordResetRequests {
	return &passwordResetRequests{db: s.db}
}

type prrDoc struct {
	ID         primitive.ObjectID  `bson:"_id"`
	Email      string              `bson:"email"`
	UID        *primitive.ObjectID `bson:"uid"`
	Status     string              `bson:"status"`
	Note       string              `bson:"note"`
	ResolvedAt *time.Time          `bson:"resolvedAt"`
	CreatedAt  time.Time           `bson:"createdAt"`
}

func (d prrDoc) toStore() store.PasswordResetRequest {
	p := store.PasswordResetRequest{ID: idOf(d.ID), Email: d.Email, Status: d.Status, Note: d.Note, ResolvedAt: d.ResolvedAt, CreatedAt: d.CreatedAt}
	if d.UID != nil {
		p.UID = idOf(*d.UID)
	}
	return p
}

func (p *passwordResetRequests) Create(ctx context.Context, email string, uid store.ID) error {
	now := time.Now().UTC()
	doc := bson.M{"email": email, "uid": optionalOID(uid), "status": "pending", "note": "", "resolvedAt": nil, "createdAt": now, "updatedAt": now, "__v": 0}
	if _, err := p.db.Collection(colPasswordResets).InsertOne(ctx, doc); err != nil {
		return fmt.Errorf("create password reset request: %w", err)
	}
	return nil
}

func (p *passwordResetRequests) HasPending(ctx context.Context, email string) (bool, error) {
	cnt, err := p.db.Collection(colPasswordResets).CountDocuments(ctx, bson.M{"email": email, "status": "pending"})
	if err != nil {
		return false, fmt.Errorf("password reset pending: %w", err)
	}
	return cnt > 0, nil
}

func (p *passwordResetRequests) ListPending(ctx context.Context) ([]store.PasswordResetRequest, error) {
	cur, err := p.db.Collection(colPasswordResets).Find(ctx, bson.M{"status": "pending"},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(50))
	if err != nil {
		return nil, fmt.Errorf("list password reset requests: %w", err)
	}
	var docs []prrDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]store.PasswordResetRequest, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.toStore())
	}
	return out, nil
}

func (p *passwordResetRequests) Get(ctx context.Context, id store.ID) (store.PasswordResetRequest, bool, error) {
	oid, err := objectID(id)
	if err != nil {
		return store.PasswordResetRequest{}, false, nil
	}
	var d prrDoc
	err = p.db.Collection(colPasswordResets).FindOne(ctx, bson.M{"_id": oid}).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.PasswordResetRequest{}, false, nil
	}
	if err != nil {
		return store.PasswordResetRequest{}, false, fmt.Errorf("get password reset request: %w", err)
	}
	return d.toStore(), true, nil
}

func (p *passwordResetRequests) Resolve(ctx context.Context, id store.ID, status string) error {
	oid, err := objectID(id)
	if err != nil {
		return nil
	}
	_, err = p.db.Collection(colPasswordResets).UpdateOne(ctx, bson.M{"_id": oid},
		bson.M{"$set": bson.M{"status": status, "resolvedAt": time.Now().UTC(), "updatedAt": time.Now().UTC()}})
	return err
}

package mongostore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

const colUsers = "users"

type usersStore struct{ db *mongo.Database }

func (s *Store) Users() store.Users { return &usersStore{db: s.db} }

type userDoc struct {
	ID           primitive.ObjectID `bson:"_id"`
	Email        string             `bson:"email"`
	Password     string             `bson:"password"`
	Name         string             `bson:"name"`
	Firm         string             `bson:"firm"`
	Role         string             `bson:"role"`
	CompanyLimit int                `bson:"companyLimit"`
	ActiveUntil  *time.Time         `bson:"activeUntil"`
	TotpEnabled  bool               `bson:"totpEnabled"`
	TotpSecret   string             `bson:"totpSecret"`
}

func (u *usersStore) FindByEmail(ctx context.Context, email string) (store.User, error) {
	var doc userDoc
	err := u.db.Collection(colUsers).FindOne(ctx, bson.M{"email": email}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.User{}, store.ErrNotFound
	}
	if err != nil {
		return store.User{}, fmt.Errorf("looking up user: %w", err)
	}
	return doc.toStore(), nil
}

func (d userDoc) toStore() store.User {
	return store.User{
		ID:           idOf(d.ID),
		Email:        d.Email,
		PasswordHash: d.Password,
		Name:         d.Name,
		Firm:         d.Firm,
		Role:         d.Role,
		CompanyLimit: d.CompanyLimit,
		ActiveUntil:  d.ActiveUntil,
		TotpEnabled:  d.TotpEnabled,
		TotpSecret:   d.TotpSecret,
	}
}

func (u *usersStore) FindByID(ctx context.Context, uid store.ID) (store.User, error) {
	oid, err := objectID(uid)
	if err != nil {
		return store.User{}, store.ErrNotFound
	}
	var doc userDoc
	err = u.db.Collection(colUsers).FindOne(ctx, bson.M{"_id": oid}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.User{}, store.ErrNotFound
	}
	if err != nil {
		return store.User{}, fmt.Errorf("looking up user: %w", err)
	}
	return doc.toStore(), nil
}

func (u *usersStore) UpdateProfile(ctx context.Context, uid store.ID, email, name string) (store.User, bool, error) {
	oid, err := objectID(uid)
	if err != nil {
		return store.User{}, false, nil
	}
	res, err := u.db.Collection(colUsers).UpdateByID(ctx, oid, bson.M{"$set": bson.M{"email": email, "name": name}})
	if err != nil {
		return store.User{}, false, fmt.Errorf("updating user profile: %w", err)
	}
	if res.MatchedCount == 0 {
		return store.User{}, false, nil
	}
	user, err := u.FindByID(ctx, uid)
	if err != nil {
		return store.User{}, false, err
	}
	return user, true, nil
}

func (u *usersStore) CreateSession(ctx context.Context, s store.NewSession) (store.Session, error) {
	uid, err := objectID(s.UID)
	if err != nil {
		return store.Session{}, err
	}

	perms := s.Permissions
	if perms == nil {
		perms = []string{}
	}

	doc := bson.M{
		"_id":         primitive.NewObjectID(),
		"token":       s.Token,
		"uid":         uid,
		"role":        s.Role,
		"permissions": perms,
		"is_active":   true,
		"remembered":  s.Remembered,
		"expiresAt":   s.ExpiresAt,
		"createdAt":   time.Now().UTC(),
		"updatedAt":   time.Now().UTC(),
	}
	if s.PersonID != "" {
		personID, err := objectID(s.PersonID)
		if err == nil {
			doc["person_id"] = personID
		}
	}

	if _, err := u.db.Collection(colUserSessions).InsertOne(ctx, doc); err != nil {
		return store.Session{}, fmt.Errorf("creating session: %w", err)
	}

	return store.Session{
		SessionID:   idOf(doc["_id"].(primitive.ObjectID)),
		Token:       s.Token,
		UID:         s.UID,
		Role:        s.Role,
		Permissions: perms,
		IsActive:    true,
		ExpiresAt:   s.ExpiresAt,
	}, nil
}

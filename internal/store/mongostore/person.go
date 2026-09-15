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

const colPersons = "people"

type people struct{ db *mongo.Database }

func (s *Store) People() store.People { return &people{db: s.db} }

// personDoc is the /person/list projection of Model/Person.js. NotifyPo* are pointers so an
// unset flag decodes to nil (absent), never false.
type personDoc struct {
	ID                primitive.ObjectID `bson:"_id"`
	Name              string             `bson:"name"`
	Type              string             `bson:"type"`
	Email             string             `bson:"email"`
	Phone             string             `bson:"phone"`
	Firm              string             `bson:"firm"`
	Address           string             `bson:"address"`
	Gst               string             `bson:"gst"`
	OpeningBalance    float64            `bson:"openingBalance"`
	IsActive          bool               `bson:"is_active"`
	Permissions       []string           `bson:"permissions"`
	NotifyPoCreated   *bool              `bson:"notifyPoCreated"`
	NotifyPoUpdated   *bool              `bson:"notifyPoUpdated"`
	NotifyPoConfirmed *bool              `bson:"notifyPoConfirmed"`
	CreatedAt         time.Time          `bson:"createdAt"`
}

func (d personDoc) toStore() store.Person {
	perms := d.Permissions
	if perms == nil {
		perms = []string{}
	}
	return store.Person{
		ID: idOf(d.ID), Name: d.Name, Type: d.Type, Email: d.Email, Phone: d.Phone,
		Firm: d.Firm, Address: d.Address, Gst: d.Gst, OpeningBalance: d.OpeningBalance,
		IsActive: d.IsActive, Permissions: perms,
		NotifyPoCreated: d.NotifyPoCreated, NotifyPoUpdated: d.NotifyPoUpdated,
		NotifyPoConfirmed: d.NotifyPoConfirmed, CreatedAt: d.CreatedAt,
	}
}

func (p *people) List(ctx context.Context, uid store.ID, personType string) ([]store.Person, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return nil, err
	}
	filter := bson.M{"uid": uidOID}
	if personType != "" {
		filter["type"] = personType
	}
	proj := bson.M{
		"name": 1, "type": 1, "email": 1, "phone": 1, "firm": 1, "address": 1, "gst": 1,
		"openingBalance": 1, "is_active": 1, "permissions": 1,
		"notifyPoCreated": 1, "notifyPoUpdated": 1, "notifyPoConfirmed": 1, "createdAt": 1,
	}
	cur, err := p.db.Collection(colPersons).Find(ctx, filter, options.Find().SetProjection(proj))
	if err != nil {
		return nil, fmt.Errorf("listing people: %w", err)
	}
	defer cur.Close(ctx)

	var docs []personDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("reading people: %w", err)
	}
	out := make([]store.Person, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.toStore())
	}
	return out, nil
}

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

func (p *people) Create(ctx context.Context, uid store.ID, in store.PersonWrite) (store.Person, bool, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return store.Person{}, false, err
	}
	perms := in.Permissions
	if perms == nil {
		perms = []string{}
	}
	now := time.Now().UTC()
	doc := bson.M{
		"uid": uidOID, "name": in.Name, "type": in.Type, "email": nullable(in.Email),
		"phone": in.Phone, "firm": in.Firm, "address": in.Address, "gst": in.Gst,
		"is_active": true, "permissions": perms, "createdAt": now, "updatedAt": now, "__v": 0,
	}
	if in.PasswordHash != nil {
		doc["password"] = *in.PasswordHash
	}
	res, err := p.db.Collection(colPersons).InsertOne(ctx, doc)
	if isDuplicate(err) {
		return store.Person{}, true, nil
	}
	if err != nil {
		return store.Person{}, false, fmt.Errorf("insert person: %w", err)
	}
	out := personDoc{Name: in.Name, Type: in.Type, Email: strOrEmpty(in.Email), Phone: in.Phone,
		Firm: in.Firm, Address: in.Address, Gst: in.Gst, IsActive: true, Permissions: perms, CreatedAt: now}.toStore()
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		out.ID = idOf(oid)
	}
	return out, false, nil
}

func (p *people) Update(ctx context.Context, uid, personID store.ID, patch store.PersonPatch) (store.Person, bool, bool, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return store.Person{}, false, false, err
	}
	personOID, err := objectID(personID)
	if err != nil {
		return store.Person{}, false, false, nil // malformed id is just "not found"
	}
	set := bson.M{}
	if patch.Name != nil {
		set["name"] = *patch.Name
	}
	if patch.Type != nil {
		set["type"] = *patch.Type
	}
	if patch.EmailSet {
		set["email"] = nullable(patch.Email)
	}
	if patch.Phone != nil {
		set["phone"] = *patch.Phone
	}
	if patch.Firm != nil {
		set["firm"] = *patch.Firm
	}
	if patch.Address != nil {
		set["address"] = *patch.Address
	}
	if patch.Gst != nil {
		set["gst"] = *patch.Gst
	}
	if patch.IsActive != nil {
		set["is_active"] = *patch.IsActive
	}
	if patch.Permissions != nil {
		perms := *patch.Permissions
		if perms == nil {
			perms = []string{}
		}
		set["permissions"] = perms
	}
	if patch.PasswordHash != nil {
		set["password"] = *patch.PasswordHash
	}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	update := bson.M{}
	if len(set) > 0 {
		update["$set"] = set
	} else {
		// Nothing to change: Node still returns the (unchanged) doc. A no-op $set keeps that.
		update["$set"] = bson.M{"updatedAt": time.Now().UTC()}
	}
	var doc personDoc
	err = p.db.Collection(colPersons).FindOneAndUpdate(ctx, bson.M{"_id": personOID, "uid": uidOID}, update, opts).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return store.Person{}, false, false, nil
	}
	if isDuplicate(err) {
		return store.Person{}, true, false, nil
	}
	if err != nil {
		return store.Person{}, false, false, fmt.Errorf("update person: %w", err)
	}
	return doc.toStore(), false, true, nil
}

func (p *people) Delete(ctx context.Context, uid, personID store.ID) (bool, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return false, err
	}
	personOID, err := objectID(personID)
	if err != nil {
		return false, nil
	}
	res, err := p.db.Collection(colPersons).DeleteOne(ctx, bson.M{"_id": personOID, "uid": uidOID})
	if err != nil {
		return false, fmt.Errorf("delete person: %w", err)
	}
	return res.DeletedCount > 0, nil
}

// nullable maps a nil/empty email pointer to a BSON null, matching Node's `email || null`.
func nullable(s *string) any {
	if s == nil || *s == "" {
		return nil
	}
	return *s
}

func strOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// FindEmployeeByEmail loads an Employee's credentials by exact email for the portal login.
func (p *people) FindEmployeeByEmail(ctx context.Context, email string) (store.EmployeeAuth, bool, error) {
	var doc struct {
		ID          primitive.ObjectID `bson:"_id"`
		UID         primitive.ObjectID `bson:"uid"`
		Name        string             `bson:"name"`
		Email       string             `bson:"email"`
		Password    string             `bson:"password"`
		IsActive    bool               `bson:"is_active"`
		Permissions []string           `bson:"permissions"`
	}
	err := p.db.Collection(colPersons).FindOne(ctx, bson.M{"email": email, "type": "Employee"}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.EmployeeAuth{}, false, nil
	}
	if err != nil {
		return store.EmployeeAuth{}, false, fmt.Errorf("employee login lookup: %w", err)
	}
	return store.EmployeeAuth{
		ID: idOf(doc.ID), UID: idOf(doc.UID), Name: doc.Name, Email: doc.Email,
		PasswordHash: doc.Password, IsActive: doc.IsActive, Permissions: doc.Permissions,
	}, true, nil
}

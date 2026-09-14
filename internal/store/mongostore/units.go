package mongostore

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

const colUnits = "units"

// defaultUnits is Helpers/UnitSeed.js's list, in its order. Exactly the set
// Helpers/InventoryMath's UNIT_RULES has a rule for - seeding a unit the inventory report
// cannot count would be a trap.
var defaultUnits = []string{"SQ. Ft", "SQ. In", "Qty", "Piece", "mm", "in", "cm", "Feet"}

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

// normaliseKey is Helpers/InventoryMath.js's normaliseKey: lowercased, punctuation replaced
// with spaces, collapsed, trimmed. DIGITS ARE KEPT - "300gsm" and "400gsm" are different
// products and must not collapse onto one line.
//
// This is the value the unique index is built on, so it has to agree with Node character for
// character. A Go version that also stripped digits would let a duplicate through the index
// that Node would have refused, and the collection would then hold a pair the index says
// cannot exist.
func normaliseKey(name string) string {
	lowered := strings.ToLower(name)
	spaced := nonAlphanumeric.ReplaceAllString(lowered, " ")
	return strings.TrimSpace(spaced)
}

type units struct{ db *mongo.Database }

func (s *Store) Units() store.Units { return &units{db: s.db} }

// unitDoc mirrors Model/Unit.js.
type unitDoc struct {
	ID        primitive.ObjectID  `bson:"_id"`
	UID       primitive.ObjectID  `bson:"uid"`
	CompanyID *primitive.ObjectID `bson:"company_id"`
	Name      string              `bson:"name"`
	Key       string              `bson:"key"`
	CreatedAt time.Time           `bson:"createdAt"`
	UpdatedAt time.Time           `bson:"updatedAt"`
	Version   int                 `bson:"__v"`
}

func (d unitDoc) toStore() store.Unit {
	u := store.Unit{
		ID:        idOf(d.ID),
		UID:       idOf(d.UID),
		Name:      d.Name,
		Key:       d.Key,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
		Version:   d.Version,
	}
	if d.CompanyID != nil {
		u.CompanyID = idOf(*d.CompanyID)
	}
	return u
}

// isDuplicate recognises Mongo's E11000. The unique index on (company_id, key) is the actual
// guarantee - an application-level "does this exist?" check is racy, because two requests can
// both read "no" before either writes.
func isDuplicate(err error) bool {
	var we mongo.WriteException
	if errors.As(err, &we) {
		for _, e := range we.WriteErrors {
			if e.Code == 11000 {
				return true
			}
		}
	}
	var bwe mongo.BulkWriteException
	if errors.As(err, &bwe) {
		for _, e := range bwe.WriteErrors {
			if e.Code == 11000 {
				return true
			}
		}
	}
	return false
}

func (u *units) List(ctx context.Context, companyID store.ID) ([]store.Unit, error) {
	oid, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	cur, err := u.db.Collection(colUnits).Find(ctx, bson.M{"company_id": oid},
		options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("listing units: %w", err)
	}
	defer cur.Close(ctx)

	var docs []unitDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("reading units: %w", err)
	}
	out := make([]store.Unit, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.toStore())
	}
	return out, nil
}

func (u *units) Create(ctx context.Context, uid, companyID store.ID, name string) (store.Unit, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return store.Unit{}, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.Unit{}, err
	}

	name = strings.TrimSpace(name)
	// `key` is maintained by a pre("validate") hook on the Mongoose model, not by callers. Go
	// has no hooks, so it is set here - in the STORE, not the handler, because a second
	// handler that forgot would create exactly the duplicate the index exists to prevent.
	now := time.Now().UTC()
	doc := unitDoc{
		ID:        primitive.NewObjectID(),
		UID:       uidOID,
		CompanyID: &companyOID,
		Name:      name,
		Key:       normaliseKey(name),
		CreatedAt: now,
		UpdatedAt: now,
		Version:   0,
	}

	if _, err := u.db.Collection(colUnits).InsertOne(ctx, doc); err != nil {
		if isDuplicate(err) {
			return store.Unit{}, store.ErrDuplicate
		}
		return store.Unit{}, fmt.Errorf("creating unit: %w", err)
	}
	return doc.toStore(), nil
}

func (u *units) Rename(ctx context.Context, unitID, companyID store.ID, name string) (store.Unit, error) {
	unitOID, err := objectID(unitID)
	if err != nil {
		// NOT ErrNotFound: Mongoose throws a CastError here, which the Node route's catch
		// block turns into a 500. See store.ErrBadID for why that bug is reproduced rather
		// than improved on.
		return store.Unit{}, store.ErrBadID
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.Unit{}, err
	}

	name = strings.TrimSpace(name)
	// Returns the document AFTER the update, because the route sends the updated unit back.
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	res := u.db.Collection(colUnits).FindOneAndUpdate(ctx,
		bson.M{"_id": unitOID, "company_id": companyOID},
		bson.M{"$set": bson.M{"name": name, "key": normaliseKey(name), "updatedAt": time.Now().UTC()}},
		opts)

	var doc unitDoc
	err = res.Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.Unit{}, store.ErrNotFound
	}
	if err != nil {
		if isDuplicate(err) {
			return store.Unit{}, store.ErrDuplicate
		}
		return store.Unit{}, fmt.Errorf("renaming unit: %w", err)
	}
	return doc.toStore(), nil
}

func (u *units) Delete(ctx context.Context, unitID, companyID store.ID) error {
	unitOID, err := objectID(unitID)
	if err != nil {
		return store.ErrBadID
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return err
	}
	res, err := u.db.Collection(colUnits).DeleteOne(ctx, bson.M{"_id": unitOID, "company_id": companyOID})
	if err != nil {
		return fmt.Errorf("deleting unit: %w", err)
	}
	if res.DeletedCount == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (u *units) SeedDefaults(ctx context.Context, uid, companyID store.ID) error {
	existing, err := u.List(ctx, companyID)
	if err != nil {
		return err
	}
	have := map[string]bool{}
	for _, unit := range existing {
		// Matched on the NORMALISED name, not the stored key: a row written before the key
		// field existed would have an empty key, and matching on that would re-seed a unit the
		// company already has.
		have[normaliseKey(unit.Name)] = true
	}

	uidOID, err := objectID(uid)
	if err != nil {
		return err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	var missing []interface{}
	for _, name := range defaultUnits {
		if have[normaliseKey(name)] {
			continue
		}
		missing = append(missing, unitDoc{
			ID:        primitive.NewObjectID(),
			UID:       uidOID,
			CompanyID: &companyOID,
			Name:      name,
			Key:       normaliseKey(name),
			CreatedAt: now,
			UpdatedAt: now,
		})
	}
	if len(missing) == 0 {
		return nil
	}

	if _, err := u.db.Collection(colUnits).InsertMany(ctx, missing); err != nil {
		// Two requests seeding at once is the expected race, not a fault: the index refuses
		// the loser's duplicates and the company ends up with exactly one of each, which is
		// the point of seeding being idempotent.
		if isDuplicate(err) {
			return nil
		}
		return fmt.Errorf("seeding units: %w", err)
	}
	return nil
}

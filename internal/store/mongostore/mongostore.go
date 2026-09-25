// Package mongostore implements store.Store against the MongoDB the Node API already owns.
//
// It is deliberately the only package that imports the mongo driver. Everything it exposes is
// a plain Go type from internal/store, so replacing it with a Postgres implementation at
// cutover touches no handler.
//
// Collection names are Mongoose's: it lowercases and pluralises a model name, so
// mongoose.model("Job") lives in "jobs" and CompanySession in "companysessions". Getting one
// of these wrong produces an empty result rather than an error, which is why they are named
// once here as constants rather than spelled out at each call site.
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
	colUserSessions    = "usersessions"
	colCompanySessions = "companysessions"
	colCompanies       = "companies"
	colJobs            = "jobs"
	colSheets          = "sheets"
)

// doneStage is the last queue stage. A card that has reached it is finished; anything else is
// still work. Spelled the same way Model/Job.js spells it - a typo here silently reports every
// card as open.
const doneStage = "Done"

type Store struct {
	client *mongo.Client
	db     *mongo.Database
}

func Open(ctx context.Context, uri, dbName string, timeout time.Duration) (*Store, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri).SetServerSelectionTimeout(timeout))
	if err != nil {
		return nil, fmt.Errorf("connecting to mongo: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("pinging mongo: %w", err)
	}
	return &Store{client: client, db: client.Database(dbName)}, nil
}

func (s *Store) Ping(ctx context.Context) error  { return s.client.Ping(ctx, nil) }
func (s *Store) Close(ctx context.Context) error { return s.client.Disconnect(ctx) }

func (s *Store) Sessions() store.Sessions { return &sessions{db: s.db} }
func (s *Store) Days() store.Days         { return &days{db: s.db} }

// objectID turns a store.ID back into what Mongo indexes on.
//
// Every _id in this database is an ObjectID, and querying one with a plain string matches
// NOTHING - no error, just an empty result, which is the single easiest way to write a Go
// handler that quietly returns nothing where Node returned rows.
func objectID(id store.ID) (primitive.ObjectID, error) {
	oid, err := primitive.ObjectIDFromHex(string(id))
	if err != nil {
		return primitive.NilObjectID, fmt.Errorf("%q is not an object id: %w", id, err)
	}
	return oid, nil
}

func idOf(v primitive.ObjectID) store.ID { return store.ID(v.Hex()) }

// objectIDs converts a set of ids to ObjectIDs, failing on the first malformed one. Used by the
// cross-company shared reads, whose scope is a list of the owner's company ids.
func objectIDs(ids []store.ID) ([]primitive.ObjectID, error) {
	out := make([]primitive.ObjectID, 0, len(ids))
	for _, id := range ids {
		oid, err := objectID(id)
		if err != nil {
			return nil, err
		}
		out = append(out, oid)
	}
	return out, nil
}

// ---------------------------------------------------------------------------------------

type sessions struct{ db *mongo.Database }

// userSessionDoc mirrors Model/UserSession.js.
type userSessionDoc struct {
	ID          primitive.ObjectID  `bson:"_id"`
	Token       string              `bson:"token"`
	UID         primitive.ObjectID  `bson:"uid"`
	Role        string              `bson:"role"`
	PersonID    *primitive.ObjectID `bson:"person_id"`
	Permissions []string            `bson:"permissions"`
	IsActive    bool                `bson:"is_active"`
	ExpiresAt   *time.Time          `bson:"expiresAt"`
}

func (s *sessions) FindByToken(ctx context.Context, token string) (store.Session, error) {
	var doc userSessionDoc
	err := s.db.Collection(colUserSessions).FindOne(ctx, bson.M{"token": token}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.Session{}, store.ErrNotFound
	}
	if err != nil {
		return store.Session{}, fmt.Errorf("looking up session: %w", err)
	}

	sess := store.Session{
		UID:         idOf(doc.UID),
		Role:        doc.Role,
		Permissions: doc.Permissions,
		SessionID:   idOf(doc.ID),
		Token:       doc.Token,
		IsActive:    doc.IsActive,
		ExpiresAt:   doc.ExpiresAt,
	}
	if doc.PersonID != nil {
		sess.PersonID = idOf(*doc.PersonID)
	}
	return sess, nil
}

func (s *sessions) Deactivate(ctx context.Context, sessionID store.ID) error {
	oid, err := objectID(sessionID)
	if err != nil {
		return err
	}
	_, err = s.db.Collection(colUserSessions).UpdateOne(ctx,
		bson.M{"_id": oid}, bson.M{"$set": bson.M{"is_active": false}})
	if err != nil {
		return fmt.Errorf("retiring session: %w", err)
	}
	return nil
}

func (s *sessions) ResolveCompany(ctx context.Context, token, tabID string, uid store.ID) (store.ID, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return "", err
	}

	// An existing binding for this tab wins.
	if tabID != "" {
		var binding struct {
			CompanyID primitive.ObjectID `bson:"company_id"`
		}
		err := s.db.Collection(colCompanySessions).
			FindOne(ctx, bson.M{"token": token, "tab_id": tabID}).Decode(&binding)
		if err == nil {
			return idOf(binding.CompanyID), nil
		}
		if !errors.Is(err, mongo.ErrNoDocuments) {
			return "", fmt.Errorf("looking up tab binding: %w", err)
		}
	}

	// Otherwise the user's default company, or any company they own.
	var company struct {
		ID primitive.ObjectID `bson:"_id"`
	}
	err = s.db.Collection(colCompanies).
		FindOne(ctx, bson.M{"uid": uidOID, "is_default": true}).Decode(&company)
	if errors.Is(err, mongo.ErrNoDocuments) {
		err = s.db.Collection(colCompanies).FindOne(ctx, bson.M{"uid": uidOID}).Decode(&company)
	}
	if errors.Is(err, mongo.ErrNoDocuments) {
		// No company at all. The Node helper returns null here and lets the route decide.
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("looking up default company: %w", err)
	}

	// Persist the fallback, or a reload puts the tab on a different company than it was on.
	if tabID != "" {
		_, err = s.db.Collection(colCompanySessions).UpdateOne(ctx,
			bson.M{"token": token, "tab_id": tabID},
			bson.M{"$set": bson.M{"token": token, "tab_id": tabID, "uid": uidOID, "company_id": company.ID}},
			options.Update().SetUpsert(true))
		if err != nil {
			return "", fmt.Errorf("persisting tab binding: %w", err)
		}
	}
	return idOf(company.ID), nil
}

// ---------------------------------------------------------------------------------------

type days struct{ db *mongo.Database }

func (d *days) CountsByDate(ctx context.Context, companyID store.ID) ([]store.DayCount, error) {
	oid, err := objectID(companyID)
	if err != nil {
		return nil, err
	}

	// One grouped query rather than one per date. $ifNull guards a job written before `rows`
	// existed: $size of a missing field is an error, not zero, and it would fail the whole
	// aggregate rather than skip the document.
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.D{{Key: "company_id", Value: oid}}}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$receivedDate"},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "cards", Value: bson.D{{Key: "$sum", Value: bson.D{
				{Key: "$size", Value: bson.D{{Key: "$ifNull", Value: bson.A{"$rows", bson.A{}}}}},
			}}}},
		}}},
	}

	cur, err := d.db.Collection(colJobs).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("counting jobs by date: %w", err)
	}
	defer cur.Close(ctx)

	var rows []struct {
		Date  string `bson:"_id"`
		Count int    `bson:"count"`
		Cards int    `bson:"cards"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("reading job counts: %w", err)
	}

	out := make([]store.DayCount, 0, len(rows))
	for _, r := range rows {
		out = append(out, store.DayCount{Date: r.Date, Jobs: r.Count, Cards: r.Cards})
	}
	return out, nil
}

func (d *days) SheetDates(ctx context.Context, companyID store.ID) (map[string]store.ID, error) {
	oid, err := objectID(companyID)
	if err != nil {
		return nil, err
	}

	cur, err := d.db.Collection(colSheets).Find(ctx, bson.M{"company_id": oid},
		options.Find().SetProjection(bson.M{"date": 1}))
	if err != nil {
		return nil, fmt.Errorf("listing sheets: %w", err)
	}
	defer cur.Close(ctx)

	var docs []struct {
		ID   primitive.ObjectID `bson:"_id"`
		Date string             `bson:"date"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("reading sheets: %w", err)
	}

	out := make(map[string]store.ID, len(docs))
	for _, doc := range docs {
		// First sheet wins the id: there is nothing to choose between two sheets for one day,
		// and the id is only a fallback link target now.
		if _, seen := out[doc.Date]; !seen {
			out[doc.Date] = idOf(doc.ID)
		}
	}
	return out, nil
}

func (d *days) OpenJobs(ctx context.Context, companyID store.ID, limit int) ([]store.OpenJob, error) {
	oid, err := objectID(companyID)
	if err != nil {
		return nil, err
	}

	// $elemMatch on the rows array, so a job with every card Done never leaves the database.
	// Filtering in Go instead would pull the whole jobs collection across the wire.
	filter := bson.M{
		"company_id": oid,
		"rows":       bson.M{"$elemMatch": bson.M{"queue": bson.M{"$ne": doneStage}}},
	}
	// Oldest first: the job-id sitting open the longest is the one worth looking at, which is
	// the opposite of how every other list here is sorted.
	// No explicit tiebreak, on purpose: Node sorts the same way (routes/Sheet.js), so relying
	// on Mongo's natural order for equal receivedDates keeps this list identical to the live
	// service. The sqlstore adds `id ASC` to reproduce that order once Postgres, which has no
	// natural order, is the tenant - see internal/store/store.go (#19).
	opts := options.Find().SetSort(bson.D{{Key: "receivedDate", Value: 1}}).SetLimit(int64(limit))

	cur, err := d.db.Collection(colJobs).Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("listing open jobs: %w", err)
	}
	defer cur.Close(ctx)

	var docs []struct {
		ID            primitive.ObjectID  `bson:"_id"`
		ChallanNumber string              `bson:"challanNumber"`
		ReceivedDate  string              `bson:"receivedDate"`
		Total         float64             `bson:"total"`
		ClientID      *primitive.ObjectID `bson:"client_id"`
		Rows          []struct {
			Queue string `bson:"queue"`
		} `bson:"rows"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("reading open jobs: %w", err)
	}

	// The client names, in one query rather than one per job. Node uses .populate(), which is
	// exactly this - a second round trip - but writing it out makes the cost visible.
	clientIDs := make([]primitive.ObjectID, 0, len(docs))
	for _, doc := range docs {
		if doc.ClientID != nil {
			clientIDs = append(clientIDs, *doc.ClientID)
		}
	}
	names, err := d.clientNames(ctx, clientIDs)
	if err != nil {
		return nil, err
	}

	out := make([]store.OpenJob, 0, len(docs))
	for _, doc := range docs {
		open := 0
		for _, row := range doc.Rows {
			if row.Queue != doneStage {
				open++
			}
		}
		job := store.OpenJob{
			ID:            idOf(doc.ID),
			ChallanNumber: doc.ChallanNumber,
			ReceivedDate:  doc.ReceivedDate,
			Total:         doc.Total,
			Cards:         len(doc.Rows),
			OpenCards:     open,
		}
		if doc.ClientID != nil {
			if c, ok := names[*doc.ClientID]; ok {
				job.HasClient = true
				job.ClientName = c.name
				job.ClientPhone = c.phone
			}
		}
		out = append(out, job)
	}
	return out, nil
}

type clientName struct{ name, phone string }

func (d *days) clientNames(ctx context.Context, ids []primitive.ObjectID) (map[primitive.ObjectID]clientName, error) {
	out := map[primitive.ObjectID]clientName{}
	if len(ids) == 0 {
		return out, nil
	}

	cur, err := d.db.Collection("clients").Find(ctx, bson.M{"_id": bson.M{"$in": ids}},
		options.Find().SetProjection(bson.M{"clientName": 1, "clientFirm": 1, "clientPhone": 1}))
	if err != nil {
		return nil, fmt.Errorf("looking up clients: %w", err)
	}
	defer cur.Close(ctx)

	var docs []struct {
		ID    primitive.ObjectID `bson:"_id"`
		Name  string             `bson:"clientName"`
		Firm  string             `bson:"clientFirm"`
		Phone string             `bson:"clientPhone"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("reading clients: %w", err)
	}

	for _, doc := range docs {
		// Firm first, falling back to the person - the same choice routes/Sheet.js makes.
		name := doc.Firm
		if name == "" {
			name = doc.Name
		}
		out[doc.ID] = clientName{name: name, phone: doc.Phone}
	}
	return out, nil
}

func (s *sessions) BindCompany(ctx context.Context, token, tabID string, uid, companyID store.ID) error {
	uidOID, err := objectID(uid)
	if err != nil {
		return err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return err
	}
	_, err = s.db.Collection(colCompanySessions).UpdateOne(ctx,
		bson.M{"token": token, "tab_id": tabID},
		bson.M{"$set": bson.M{"token": token, "tab_id": tabID, "uid": uidOID, "company_id": companyOID}},
		options.Update().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("binding tab to company: %w", err)
	}
	return nil
}

// DeactivateOthers retires every other session of uid (all but keepToken), matching Node's
// UserSession.updateMany({uid, token:{$ne}}, {is_active:false}) - it does not filter on the
// current active flag, so re-running it is idempotent.
func (s *sessions) DeactivateOthers(ctx context.Context, uid store.ID, keepToken string) error {
	oid, err := objectID(uid)
	if err != nil {
		return err
	}
	_, err = s.db.Collection(colUserSessions).UpdateMany(ctx,
		bson.M{"uid": oid, "token": bson.M{"$ne": keepToken}},
		bson.M{"$set": bson.M{"is_active": false}})
	if err != nil {
		return fmt.Errorf("deactivate other sessions: %w", err)
	}
	return nil
}

func (s *sessions) LastLoginAt(ctx context.Context, uid store.ID) (*time.Time, error) {
	oid, err := objectID(uid)
	if err != nil {
		return nil, nil
	}
	var doc struct {
		ID        primitive.ObjectID `bson:"_id"`
		CreatedAt *time.Time         `bson:"createdAt"`
	}
	err = s.db.Collection(colUserSessions).FindOne(ctx, bson.M{"uid": oid},
		options.FindOne().SetSort(bson.D{{Key: "_id", Value: -1}})).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("last login: %w", err)
	}
	if doc.CreatedAt != nil {
		return doc.CreatedAt, nil
	}
	t := doc.ID.Timestamp()
	return &t, nil
}

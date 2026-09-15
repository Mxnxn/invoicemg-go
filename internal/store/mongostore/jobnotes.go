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
	colJobNotes     = "jobnotes"
	colJobHistories = "jobhistories"
)

type jobNotes struct{ db *mongo.Database }

func (s *Store) JobNotes() store.JobNotes { return &jobNotes{db: s.db} }

func (n *jobNotes) actorName(ctx context.Context, actor store.NoteActor) string {
	if actor.Role == "admin" {
		if oid, err := objectID(actor.UID); err == nil {
			var u struct {
				Name string `bson:"name"`
			}
			if n.db.Collection(colUsers).FindOne(ctx, bson.M{"_id": oid}).Decode(&u) == nil && u.Name != "" {
				return u.Name
			}
		}
		return "Admin"
	}
	if oid, err := objectID(actor.PersonID); err == nil {
		var p struct {
			Name string `bson:"name"`
		}
		if n.db.Collection(colPersons).FindOne(ctx, bson.M{"_id": oid}).Decode(&p) == nil && p.Name != "" {
			return p.Name
		}
	}
	return "Unknown"
}

func (n *jobNotes) logHistory(ctx context.Context, uid, companyID, jobID primitive.ObjectID, actor store.NoteActor, name, action, detail string) error {
	actorID, _ := objectID(actor.ActorID())
	_, err := n.db.Collection(colJobHistories).InsertOne(ctx, bson.M{
		"job_id": jobID, "uid": uid, "company_id": companyID, "actorType": actor.Role,
		"actorId": actorID, "actorName": name, "action": action, "detail": detail,
		"createdAt": time.Now().UTC(), "__v": 0,
	})
	return err
}

type jobNoteDoc struct {
	ID         primitive.ObjectID `bson:"_id"`
	JobID      primitive.ObjectID `bson:"job_id"`
	AuthorType string             `bson:"authorType"`
	AuthorID   primitive.ObjectID `bson:"authorId"`
	AuthorName string             `bson:"authorName"`
	Text       string             `bson:"text"`
	CreatedAt  time.Time          `bson:"createdAt"`
	UpdatedAt  time.Time          `bson:"updatedAt"`
	Version    int                `bson:"__v"`
}

func (d jobNoteDoc) toStore() store.JobNote {
	return store.JobNote{
		ID: idOf(d.ID), JobID: idOf(d.JobID), AuthorType: d.AuthorType, AuthorID: idOf(d.AuthorID),
		AuthorName: d.AuthorName, Text: d.Text, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt, Version: d.Version,
	}
}

func (n *jobNotes) NotesList(ctx context.Context, uid, companyID, jobID store.ID) ([]store.JobNote, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return nil, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	jobOID, err := objectID(jobID)
	if err != nil {
		return []store.JobNote{}, nil
	}
	cur, err := n.db.Collection(colJobNotes).Find(ctx, bson.M{"job_id": jobOID, "uid": uidOID, "company_id": companyOID},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("notes list: %w", err)
	}
	var docs []jobNoteDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]store.JobNote, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.toStore())
	}
	return out, nil
}

func (n *jobNotes) NoteCreate(ctx context.Context, uid, companyID, jobID store.ID, actor store.NoteActor, text string) (store.JobNote, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return store.JobNote{}, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.JobNote{}, err
	}
	jobOID, err := objectID(jobID)
	if err != nil {
		return store.JobNote{}, store.ErrBadID
	}
	name := n.actorName(ctx, actor)
	actorID, _ := objectID(actor.ActorID())
	now := time.Now().UTC()
	doc := jobNoteDoc{
		ID: primitive.NewObjectID(), JobID: jobOID, AuthorType: actor.Role, AuthorID: actorID,
		AuthorName: name, Text: text, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := n.db.Collection(colJobNotes).InsertOne(ctx, bson.M{
		"_id": doc.ID, "job_id": jobOID, "uid": uidOID, "company_id": companyOID,
		"authorType": actor.Role, "authorId": actorID, "authorName": name, "text": text,
		"createdAt": now, "updatedAt": now, "__v": 0,
	}); err != nil {
		return store.JobNote{}, fmt.Errorf("note create: %w", err)
	}
	detail := text
	if len(detail) > 80 {
		detail = detail[:80]
	}
	if err := n.logHistory(ctx, uidOID, companyOID, jobOID, actor, name, "Note added", detail); err != nil {
		return store.JobNote{}, fmt.Errorf("log history: %w", err)
	}
	return doc.toStore(), nil
}

func (n *jobNotes) NoteUpdate(ctx context.Context, uid, companyID, noteID store.ID, actor store.NoteActor, text string) (store.JobNote, bool, bool, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return store.JobNote{}, false, false, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return store.JobNote{}, false, false, err
	}
	noteOID, err := objectID(noteID)
	if err != nil {
		return store.JobNote{}, false, false, nil
	}
	var doc jobNoteDoc
	err = n.db.Collection(colJobNotes).FindOne(ctx, bson.M{"_id": noteOID, "uid": uidOID, "company_id": companyOID}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.JobNote{}, false, false, nil
	}
	if err != nil {
		return store.JobNote{}, false, false, fmt.Errorf("note lookup: %w", err)
	}
	if !store.CanEditNote(idOf(doc.AuthorID), actor.ActorID(), doc.CreatedAt, time.Now().UTC()) {
		return store.JobNote{}, true, true, nil
	}
	now := time.Now().UTC()
	if _, err := n.db.Collection(colJobNotes).UpdateByID(ctx, noteOID, bson.M{"$set": bson.M{"text": text, "updatedAt": now}}); err != nil {
		return store.JobNote{}, false, false, fmt.Errorf("note update: %w", err)
	}
	doc.Text, doc.UpdatedAt = text, now
	name := n.actorName(ctx, actor)
	detail := text
	if len(detail) > 80 {
		detail = detail[:80]
	}
	if err := n.logHistory(ctx, uidOID, companyOID, doc.JobID, actor, name, "Note edited", detail); err != nil {
		return store.JobNote{}, false, false, fmt.Errorf("log history: %w", err)
	}
	return doc.toStore(), true, false, nil
}

func (n *jobNotes) HistoryList(ctx context.Context, uid, companyID, jobID store.ID) ([]store.JobHistoryRow, error) {
	uidOID, err := objectID(uid)
	if err != nil {
		return nil, err
	}
	companyOID, err := objectID(companyID)
	if err != nil {
		return nil, err
	}
	jobOID, err := objectID(jobID)
	if err != nil {
		return []store.JobHistoryRow{}, nil
	}
	cur, err := n.db.Collection(colJobHistories).Find(ctx, bson.M{"job_id": jobOID, "uid": uidOID, "company_id": companyOID},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("history list: %w", err)
	}
	var docs []struct {
		ID        primitive.ObjectID `bson:"_id"`
		JobID     primitive.ObjectID `bson:"job_id"`
		ActorType string             `bson:"actorType"`
		ActorID   primitive.ObjectID `bson:"actorId"`
		ActorName string             `bson:"actorName"`
		Action    string             `bson:"action"`
		Detail    string             `bson:"detail"`
		CreatedAt time.Time          `bson:"createdAt"`
		Version   int                `bson:"__v"`
	}
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]store.JobHistoryRow, 0, len(docs))
	for _, d := range docs {
		out = append(out, store.JobHistoryRow{
			ID: idOf(d.ID), JobID: idOf(d.JobID), ActorType: d.ActorType, ActorID: idOf(d.ActorID),
			ActorName: d.ActorName, Action: d.Action, Detail: d.Detail, CreatedAt: d.CreatedAt, Version: d.Version,
		})
	}
	return out, nil
}

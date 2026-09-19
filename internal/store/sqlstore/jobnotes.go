package sqlstore

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type jobNotes struct{ pool *pgxpool.Pool }

func (s *Store) JobNotes() store.JobNotes { return &jobNotes{pool: s.pool} }

// actorName resolves the display name of the acting actor: an admin's user name, else the
// person's name (Helpers/Lifecycle actorName). Falls back the way Node does.
func (n *jobNotes) actorName(ctx context.Context, actor store.NoteActor) string {
	if actor.Role == "admin" {
		var name string
		if err := n.pool.QueryRow(ctx, `SELECT name FROM users WHERE id=$1`, string(actor.UID)).Scan(&name); err == nil && name != "" {
			return name
		}
		return "Admin"
	}
	var name string
	if err := n.pool.QueryRow(ctx, `SELECT name FROM persons WHERE id=$1`, string(actor.PersonID)).Scan(&name); err == nil && name != "" {
		return name
	}
	return "Unknown"
}

func (n *jobNotes) logHistory(ctx context.Context, uid, companyID, jobID store.ID, actor store.NoteActor, name, action, detail string) error {
	_, err := n.pool.Exec(ctx, `
		INSERT INTO job_history (job_id, uid, company_id, actor_type, actor_id, actor_name, action, detail)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		string(jobID), string(uid), string(companyID), actor.Role, string(actor.ActorID()), name, action, detail)
	return err
}

func (n *jobNotes) NotesList(ctx context.Context, uid, companyID, jobID store.ID) ([]store.JobNote, error) {
	rows, err := n.pool.Query(ctx, `
		SELECT id, job_id, author_type, author_id, author_name, text, created_at, updated_at
		  FROM job_notes WHERE job_id=$1 AND uid=$2 AND company_id=$3
		 ORDER BY created_at DESC, id DESC`, string(jobID), string(uid), string(companyID))
	if err != nil {
		return nil, fmt.Errorf("notes list: %w", err)
	}
	defer rows.Close()
	out := make([]store.JobNote, 0)
	for rows.Next() {
		note, err := scanNote(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, note)
	}
	return out, rows.Err()
}

func (n *jobNotes) NoteCreate(ctx context.Context, uid, companyID, jobID store.ID, actor store.NoteActor, text string) (store.JobNote, error) {
	name := n.actorName(ctx, actor)
	var note store.JobNote
	err := n.pool.QueryRow(ctx, `
		INSERT INTO job_notes (job_id, uid, company_id, author_type, author_id, author_name, text)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, job_id, author_type, author_id, author_name, text, created_at, updated_at`,
		string(jobID), string(uid), string(companyID), actor.Role, string(actor.ActorID()), name, text).
		Scan(&note.ID, &note.JobID, &note.AuthorType, &note.AuthorID, &note.AuthorName, &note.Text, &note.CreatedAt, &note.UpdatedAt)
	if err != nil {
		return store.JobNote{}, fmt.Errorf("note create: %w", err)
	}
	detail := text
	if len(detail) > 80 {
		detail = detail[:80]
	}
	if err := n.logHistory(ctx, uid, companyID, jobID, actor, name, "Note added", detail); err != nil {
		return store.JobNote{}, fmt.Errorf("log history: %w", err)
	}
	return note, nil
}

func (n *jobNotes) NoteUpdate(ctx context.Context, uid, companyID, noteID store.ID, actor store.NoteActor, text string) (store.JobNote, bool, bool, error) {
	var authorID store.ID
	var createdAt time.Time
	var jobID store.ID
	err := n.pool.QueryRow(ctx, `SELECT author_id, created_at, job_id FROM job_notes WHERE id=$1 AND uid=$2 AND company_id=$3`,
		string(noteID), string(uid), string(companyID)).Scan(&authorID, &createdAt, &jobID)
	if noRows(err) {
		return store.JobNote{}, false, false, nil
	}
	if err != nil {
		return store.JobNote{}, false, false, fmt.Errorf("note lookup: %w", err)
	}
	if !store.CanEditNote(authorID, actor.ActorID(), createdAt, time.Now().UTC()) {
		return store.JobNote{}, true, true, nil
	}
	var note store.JobNote
	err = n.pool.QueryRow(ctx, `
		UPDATE job_notes SET text=$2, updated_at=now() WHERE id=$1
		RETURNING id, job_id, author_type, author_id, author_name, text, created_at, updated_at`,
		string(noteID), text).
		Scan(&note.ID, &note.JobID, &note.AuthorType, &note.AuthorID, &note.AuthorName, &note.Text, &note.CreatedAt, &note.UpdatedAt)
	if err != nil {
		return store.JobNote{}, false, false, fmt.Errorf("note update: %w", err)
	}
	name := n.actorName(ctx, actor)
	detail := text
	if len(detail) > 80 {
		detail = detail[:80]
	}
	if err := n.logHistory(ctx, uid, companyID, jobID, actor, name, "Note edited", detail); err != nil {
		return store.JobNote{}, false, false, fmt.Errorf("log history: %w", err)
	}
	return note, true, false, nil
}

func (n *jobNotes) HistoryList(ctx context.Context, uid, companyID, jobID store.ID) ([]store.JobHistoryRow, error) {
	rows, err := n.pool.Query(ctx, `
		SELECT id, job_id, actor_type, actor_id, actor_name, action, detail, from_stage, to_stage, row_key, created_at
		  FROM job_history WHERE job_id=$1 AND uid=$2 AND company_id=$3
		 ORDER BY created_at DESC, id DESC`, string(jobID), string(uid), string(companyID))
	if err != nil {
		return nil, fmt.Errorf("history list: %w", err)
	}
	defer rows.Close()
	out := make([]store.JobHistoryRow, 0)
	for rows.Next() {
		var h store.JobHistoryRow
		if err := rows.Scan(&h.ID, &h.JobID, &h.ActorType, &h.ActorID, &h.ActorName, &h.Action, &h.Detail,
			&h.FromStage, &h.ToStage, &h.RowKey, &h.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func scanNote(row interface{ Scan(...any) error }) (store.JobNote, error) {
	var note store.JobNote
	err := row.Scan(&note.ID, &note.JobID, &note.AuthorType, &note.AuthorID, &note.AuthorName, &note.Text, &note.CreatedAt, &note.UpdatedAt)
	return note, err
}

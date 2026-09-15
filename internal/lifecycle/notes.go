package lifecycle

import (
	"net/http"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

// actorOf builds the note/history actor from the session: an admin acts as their user, an
// employee as their person (Helpers/Lifecycle actorFromAuth).
func actorOf(sess store.Session) store.NoteActor {
	return store.NoteActor{Role: sess.Role, UID: sess.UID, PersonID: sess.PersonID}
}

func noteDTO(n store.JobNote, canEdit bool) map[string]any {
	return map[string]any{
		"_id": string(n.ID), "job_id": string(n.JobID), "authorType": n.AuthorType,
		"authorId": string(n.AuthorID), "authorName": n.AuthorName, "text": n.Text,
		"createdAt": httpx.NewTime(n.CreatedAt), "updatedAt": httpx.NewTime(n.UpdatedAt),
		"__v": n.Version, "canEdit": canEdit,
	}
}

// NotesList is POST /lifecycle/notes/list: a job's notes, each flagged canEdit for the actor.
func (h *Handler) NotesList(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	jobID := form.String("job_id")
	if jobID == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	notes, err := h.notes.NotesList(r.Context(), sess.UID, sess.CompanyID, store.ID(jobID))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	actorID := actorOf(sess).ActorID()
	nowT := time.Now().UTC()
	out := make([]map[string]any, 0, len(notes))
	for _, n := range notes {
		out = append(out, noteDTO(n, store.CanEditNote(n.AuthorID, actorID, n.CreatedAt, nowT)))
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: out})
}

// NoteCreate is POST /lifecycle/notes/create.
func (h *Handler) NoteCreate(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	jobID := form.String("job_id")
	text := form.String("text")
	if jobID == "" || text == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	note, err := h.notes.NoteCreate(r.Context(), sess.UID, sess.CompanyID, store.ID(jobID), actorOf(sess), text)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	// A note just authored is always editable.
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Note added.", Data: noteDTO(note, true)})
}

// NoteUpdate is POST /lifecycle/notes/update: edit within the author's 24h window.
func (h *Handler) NoteUpdate(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	noteID := form.String("note_id")
	text := form.String("text")
	if noteID == "" || text == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	note, found, forbidden, err := h.notes.NoteUpdate(r.Context(), sess.UID, sess.CompanyID, store.ID(noteID), actorOf(sess), text)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Note not found.", Status: httpx.False()})
		return
	}
	if forbidden {
		httpx.Write(w, httpx.Envelope{Code: 403, Message: "This note can no longer be edited (more than 24 hours old, or you're not its author).", Status: httpx.False()})
		return
	}
	canEdit := store.CanEditNote(note.AuthorID, actorOf(sess).ActorID(), note.CreatedAt, time.Now().UTC())
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Note updated.", Data: noteDTO(note, canEdit)})
}

// HistoryList is POST /lifecycle/history/list: a job's audit trail, newest first.
func (h *Handler) HistoryList(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	jobID := form.String("job_id")
	if jobID == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	rows, err := h.notes.HistoryList(r.Context(), sess.UID, sess.CompanyID, store.ID(jobID))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, hrow := range rows {
		out = append(out, map[string]any{
			"_id": string(hrow.ID), "job_id": string(hrow.JobID), "actorType": hrow.ActorType,
			"actorId": string(hrow.ActorID), "actorName": hrow.ActorName, "action": hrow.Action,
			"detail": hrow.Detail, "createdAt": httpx.NewTime(hrow.CreatedAt), "__v": hrow.Version,
		})
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: out})
}

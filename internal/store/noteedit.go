package store

import "time"

// NoteEditWindow is Helpers/NoteEditWindow: a note is editable only by its author, within 24h.
const NoteEditWindow = 24 * time.Hour

// CanEditNote ports canEditNote: the actor must be the author, and the note under 24h old.
func CanEditNote(authorID, actorID ID, createdAt, now time.Time) bool {
	if authorID != actorID {
		return false
	}
	return now.Sub(createdAt) < NoteEditWindow
}

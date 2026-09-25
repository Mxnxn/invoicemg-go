package purchaseorder

import (
	"net/http"
	"strings"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

func toNoteDTO(n store.PONote) noteDTO {
	return noteDTO{
		ID: string(n.ID), POID: string(n.POID), AuthorType: n.AuthorType, AuthorID: strPtr(n.AuthorID),
		AuthorName: n.AuthorName, Text: n.Text, CreatedAt: httpx.NewTime(n.CreatedAt), UpdatedAt: httpx.NewTime(n.UpdatedAt),
	}
}

// Approve is POST /purchase-order/approve (requireFeature purchase_orders_approve).
func (h *Handler) Approve(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	poID := form.String("po_id")
	if poID == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	po, status, err := h.store.Approve(r.Context(), sess.UID, sess.CompanyID, store.ID(poID), actorOf(r))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	switch status {
	case store.POActionNotFound:
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Purchase order not found.", Status: httpx.False()})
	case store.POActionConverted:
		httpx.Write(w, httpx.Envelope{Code: 409, Message: "This purchase order has already become a purchase invoice.", Status: httpx.False()})
	case store.POActionNoRows:
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Add at least one row before approving.", Status: httpx.False()})
	case store.POActionAlreadyApproved:
		httpx.Write(w, httpx.Envelope{Code: 409, Message: "Already approved.", Status: httpx.False()})
	default:
		httpx.Write(w, httpx.Envelope{Code: 200, Message: "Purchase order approved.", Status: httpx.True(), Data: toPODTO(po)})
	}
}

// Revoke is POST /purchase-order/revoke (requireFeature purchase_orders_approve).
func (h *Handler) Revoke(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	poID := form.String("po_id")
	if poID == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	po, status, err := h.store.Revoke(r.Context(), sess.UID, sess.CompanyID, store.ID(poID), actorOf(r))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	switch status {
	case store.POActionNotFound:
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Purchase order not found.", Status: httpx.False()})
	case store.POActionNotApproved:
		httpx.Write(w, httpx.Envelope{Code: 409, Message: "This purchase order is not approved.", Status: httpx.False()})
	default:
		httpx.Write(w, httpx.Envelope{Code: 200, Message: "Approval withdrawn.", Status: httpx.True(), Data: toPODTO(po)})
	}
}

// Note is POST /purchase-order/note (requireCreate purchase_orders).
func (h *Handler) Note(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	poID := form.String("po_id")
	text := form.String("text")
	if poID == "" || strings.TrimSpace(text) == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Write something first.", Status: httpx.False()})
		return
	}
	note, found, err := h.store.AddNote(r.Context(), sess.UID, sess.CompanyID, store.ID(poID), actorOf(r), text)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Purchase order not found.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Note added.", Status: httpx.True(), Data: toNoteDTO(note)})
}

// NoteEdit is POST /purchase-order/note/edit (requireCreate purchase_orders).
func (h *Handler) NoteEdit(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	noteID := form.String("note_id")
	text := form.String("text")
	if noteID == "" || strings.TrimSpace(text) == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	note, found, forbidden, err := h.store.EditNote(r.Context(), sess.CompanyID, store.ID(noteID), actorOf(r), text)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Note not found.", Status: httpx.False()})
		return
	}
	if forbidden {
		httpx.Write(w, httpx.Envelope{Code: 403, Message: "A note can only be edited by its author, within 24 hours.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Note updated.", Status: httpx.True(), Data: toNoteDTO(note)})
}

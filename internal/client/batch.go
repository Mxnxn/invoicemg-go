package client

import (
	"net/http"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

// BatchUpdate is POST /client/batchUpdate: log a plain client receipt (no entry allocation - the
// auto-apply is disabled in Node). Requires uid, client_id and amount (422 "Invalid request"
// status:true), and 404s when the client is not the caller's. Date defaults to today.
func (h *Handler) BatchUpdate(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	if !form.Has("uid") || !form.Has("client_id") || !form.Has("amount") {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request", Status: httpx.True()})
		return
	}
	date := form.String("date")
	if date == "" {
		date = time.Now().UTC().Format("2006-01-02")
	}
	b, found, err := h.batches.CreateSimple(r.Context(), sess.UID, sess.CompanyID,
		store.ID(form.String("client_id")), form.Float("amount", 0), date, form.String("note"))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Client not found.", Status: httpx.False()})
		return
	}
	data := map[string]any{
		"_id": string(b.ID), "client": idOrNil(b.ClientID), "uid": string(b.UID),
		"company_id": idOrNil(b.CompanyID), "date": b.Date, "amount": b.Amount, "note": b.Note,
		"createdAt": httpx.NewTime(b.CreatedAt), "updatedAt": httpx.NewTime(b.UpdatedAt), "__v": b.Version,
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "ok", Status: httpx.True(), Data: data})
}

// BatchReceiveUpdate is POST /client/batchReceiveUpdate: patch a receipt's amount/note/date by
// id (only the fields sent), company-scoped. 422 without batch_id, 404 on a miss.
func (h *Handler) BatchReceiveUpdate(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	if !form.Has("batch_id") {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request", Status: httpx.False()})
		return
	}
	var amount *float64
	var note, date *string
	if form.Present("amount") {
		v := form.Float("amount", 0)
		amount = &v
	}
	if form.Present("note") {
		v := form.String("note")
		note = &v
	}
	if form.Present("date") {
		v := form.String("date")
		date = &v
	}
	b, found, err := h.batches.UpdateSimple(r.Context(), sess.CompanyID, store.ID(form.String("batch_id")), amount, note, date)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Batch receive not found.", Status: httpx.False()})
		return
	}
	data := map[string]any{
		"_id": string(b.ID), "client": idOrNil(b.ClientID), "uid": string(b.UID),
		"company_id": idOrNil(b.CompanyID), "date": b.Date, "amount": b.Amount, "note": b.Note,
		"createdAt": httpx.NewTime(b.CreatedAt), "updatedAt": httpx.NewTime(b.UpdatedAt), "__v": b.Version,
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Batch receive updated.", Data: data})
}

// BatchReceiveDelete is POST /client/batchReceiveDelete: a plain delete of a receipt by id with
// no allocation reversal (Node's fill logic is disabled), company-scoped. 422/404.
func (h *Handler) BatchReceiveDelete(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	if !form.Has("batch_id") {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request", Status: httpx.False()})
		return
	}
	found, err := h.batches.DeleteSimple(r.Context(), sess.CompanyID, store.ID(form.String("batch_id")))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Batch receive not found.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Batch receive deleted.", Status: httpx.True()})
}

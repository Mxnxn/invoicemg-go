package batchreceive

import (
	"net/http"
	"strconv"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

func invalid(w http.ResponseWriter, msg string) {
	httpx.Write(w, httpx.Envelope{Code: 422, Message: msg, Status: httpx.False()})
}

func brDTO(b store.BatchReceive) map[string]any {
	dests := make([]map[string]any, 0, len(b.Destinations))
	for _, d := range b.Destinations {
		dests = append(dests, map[string]any{"kind": d.Kind, "id": d.ID, "label": d.Label, "amount": d.Amount})
	}
	row := map[string]any{
		"_id": string(b.ID), "uid": string(b.UID), "date": b.Date, "amount": b.Amount,
		"note": b.Note, "mode": b.Mode, "destinations": dests,
		"bank_id": bankObj(b.BankID, b.BankName), "createdAt": httpx.NewTime(b.CreatedAt), "__v": b.Version,
	}
	if b.ClientID != "" {
		row["client"] = map[string]any{"_id": string(b.ClientID), "clientName": b.ClientName, "clientFirm": b.ClientFirm, "clientPhone": b.ClientPhone}
	} else {
		row["client"] = nil
	}
	return row
}

// OpenJobs is POST /batch-receive/lookups/open-jobs: a client's unpaid jobs, oldest first.
func (h *Handler) OpenJobs(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	clientID := form.String("client_id")
	if clientID == "" {
		invalid(w, "Invalid request.")
		return
	}
	jobs, err := h.store.OpenJobs(r.Context(), sess.UID, sess.CompanyID, store.ID(clientID))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out := make([]map[string]any, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, map[string]any{
			"_id": string(j.ID), "challanNumber": j.ChallanNumber, "receivedDate": j.ReceivedDate,
			"total": j.Total, "advance": j.Advance, "remaining": j.Remaining, "entryCount": j.EntryCount,
		})
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: out})
}

// Create is POST /batch-receive/create: record a client lump payment, auto- or manually-allocated.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)

	if !form.Has("client_id") || !form.Has("amount") || !form.Has("mode") {
		invalid(w, "Invalid request.")
		return
	}
	amount, _ := strconv.ParseFloat(form.String("amount"), 64)
	if !(amount > 0) {
		invalid(w, "Amount must be greater than 0.")
		return
	}
	mode := form.String("mode")
	if mode != "auto" && mode != "manual" {
		invalid(w, "mode must be 'auto' or 'manual'.")
		return
	}

	in := store.BatchReceiveWrite{
		ClientID: store.ID(form.String("client_id")), Amount: amount, Mode: mode,
		Note: form.String("note"), BankID: store.ID(form.String("bank_id")), Date: form.String("date"),
	}
	if mode == "manual" {
		var incoming []struct {
			JobID  string  `json:"job_id"`
			Amount float64 `json:"amount"`
		}
		if err := form.JSONOr("allocations", "[]", &incoming); err != nil {
			invalid(w, "allocations must be a JSON array.")
			return
		}
		if len(incoming) == 0 {
			invalid(w, "Manual mode needs at least one job allocation.")
			return
		}
		var allocated float64
		for _, a := range incoming {
			allocated += a.Amount
			in.Allocations = append(in.Allocations, store.BatchAllocInput{JobID: store.ID(a.JobID), Amount: a.Amount})
		}
		if allocated > amount+0.01 {
			invalid(w, "Allocated amount exceeds the batch amount.")
			return
		}
	}

	saved, err := h.store.Create(r.Context(), sess.UID, sess.CompanyID, in)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Batch receive created.", Data: brDTO(saved)})
}

// Delete is POST /batch-receive/delete: reverse and remove a recorded receipt.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	id := form.String("batch_id")
	if id == "" {
		invalid(w, "Invalid request.")
		return
	}
	found, err := h.store.Delete(r.Context(), sess.UID, sess.CompanyID, store.ID(id))
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

// Package batchreceive serves POST /batch-receive/list from routes/BatchReceive.js - client
// lump payments, newest first, with client/bank populated and each transfer's destinations
// (which jobs/invoices it settled) resolved. Behind the batch_receive feature.
package batchreceive

import (
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type Handler struct{ store store.BatchReceives }

func New(s store.BatchReceives) *Handler { return &Handler{store: s} }

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	list, err := h.store.List(r.Context(), sess.UID, sess.CompanyID, store.ID(form.String("client_id")))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, b := range list {
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
		out = append(out, row)
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: out})
}

// bankObj renders the populated bank {_id,name}, or null when unset.
func bankObj(id store.ID, name string) any {
	if id == "" {
		return nil
	}
	return map[string]any{"_id": string(id), "name": name}
}

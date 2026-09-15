package client

import (
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/entry"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

// Get is POST /client/get: one company-scoped client with its entries populated (issued invoice
// number, quotation number) and its batch receipts attached as batchUpdates. company_id is
// server-derived, so a caller who knows an id cannot read another company's client. 404 covers
// both "no such id" and "belongs to another company" so the response cannot probe which exist.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	clientID := form.String("client_id")
	if clientID == "" {
		httpx.Invalid(w, "")
		return
	}
	d, found, err := h.store.Get(r.Context(), sess.CompanyID, store.ID(clientID))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Client not found.", Status: httpx.False()})
		return
	}

	entries := make([]map[string]any, 0, len(d.Entries))
	for _, v := range d.Entries {
		m := entry.EntryJSON(v.Entry)
		if v.IssuedID != "" {
			m["issued"] = map[string]any{"_id": string(v.IssuedID), "invoiceId": v.IssuedInvoiceID}
		}
		if v.QuotationID != "" {
			m["quotation_id"] = map[string]any{"_id": string(v.QuotationID), "quotationNumber": v.QuotationNumber}
		} else {
			m["quotation_id"] = nil
		}
		entries = append(entries, m)
	}

	batchUpdates, err := h.batchUpdates(r, sess, store.ID(clientID))
	if err != nil {
		httpx.Internal(w, err)
		return
	}

	data := map[string]any{
		"_id":           string(d.ID),
		"uid":           idOrNil(d.UID),
		"company_id":    idOrNil(d.CompanyID),
		"client_id":     d.LegacyID,
		"clientName":    d.ClientName,
		"clientFirm":    d.ClientFirm,
		"clientPhone":   d.ClientPhone,
		"clientGST":     d.ClientGST,
		"clientAddress": d.ClientAddress,
		"entries":       entries,
		"batchUpdates":  batchUpdates,
		"__v":           0,
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: data})
}

// batchUpdates is the client's batch receipts newest-first, the shape /batch-receive/list uses
// (a superset of Node's raw BatchReceive.find here - the extra populated names are harmless).
func (h *Handler) batchUpdates(r *http.Request, sess store.Session, clientID store.ID) ([]map[string]any, error) {
	list, err := h.batches.List(r.Context(), sess.UID, sess.CompanyID, clientID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(list))
	for _, b := range list {
		dests := make([]map[string]any, 0, len(b.Destinations))
		for _, dst := range b.Destinations {
			dests = append(dests, map[string]any{"kind": dst.Kind, "id": dst.ID, "label": dst.Label, "amount": dst.Amount})
		}
		row := map[string]any{
			"_id": string(b.ID), "uid": string(b.UID), "date": b.Date, "amount": b.Amount,
			"note": b.Note, "mode": b.Mode, "destinations": dests,
			"createdAt": httpx.NewTime(b.CreatedAt), "__v": b.Version,
		}
		if b.BankID != "" {
			row["bank_id"] = map[string]any{"_id": string(b.BankID), "name": b.BankName}
		} else {
			row["bank_id"] = nil
		}
		if b.ClientID != "" {
			row["client"] = map[string]any{"_id": string(b.ClientID), "clientName": b.ClientName, "clientFirm": b.ClientFirm, "clientPhone": b.ClientPhone}
		} else {
			row["client"] = nil
		}
		out = append(out, row)
	}
	return out, nil
}

func idOrNil(id store.ID) any {
	if id == "" {
		return nil
	}
	return string(id)
}

package invoice

import (
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/docnumber"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

// now is injectable so the FY the next-number helper picks is deterministic in tests.
var now = func() time.Time { return time.Now().UTC() }

func invalid(w http.ResponseWriter, msg string) {
	httpx.Write(w, httpx.Envelope{Code: 422, Message: msg, Status: httpx.False()})
}

// NextNumber is POST /invoice/next-invoice-number.
func (h *Handler) NextNumber(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	numbers, err := h.invoices.Numbers(r.Context(), sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	next := docnumber.Next(numbers, "INV-", now())
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: map[string]any{"invoiceNumber": next}})
}

// EntriesJobs is POST /invoice/entries-jobs: map each entry id to its job's challan number.
func (h *Handler) EntriesJobs(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	var ids []string
	if err := form.JSONOr("entry_ids", "[]", &ids); err != nil {
		invalid(w, "entry_ids must be a JSON array.")
		return
	}
	if len(ids) == 0 {
		httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: map[string]any{}})
		return
	}
	entryIDs := make([]store.ID, len(ids))
	for i, s := range ids {
		entryIDs[i] = store.ID(s)
	}
	labels, err := h.invoices.EntryJobLabels(r.Context(), sess.CompanyID, entryIDs)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out := map[string]any{}
	for k, v := range labels {
		out[k] = v
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: out})
}

// GetReceived is POST /invoice/getReceived: payments recorded against an invoice, bank populated.
func (h *Handler) GetReceived(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	invoiceID := form.String("invoice_id")
	if invoiceID == "" {
		invalid(w, "Invalid request.")
		return
	}
	list, err := h.invoices.Received(r.Context(), sess.CompanyID, store.ID(invoiceID))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, rc := range list {
		row := map[string]any{
			"_id": string(rc.ID), "date": rc.Date, "amount": rc.Amount, "note": rc.Note,
			"invoice_id": string(rc.InvoiceID), "createdAt": httpx.NewTime(rc.CreatedAt), "__v": rc.Version,
		}
		if rc.BankID != "" {
			row["bank_id"] = map[string]any{"_id": string(rc.BankID), "name": rc.BankName}
		} else {
			row["bank_id"] = nil
		}
		out = append(out, row)
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Data: out, Status: httpx.True()})
}

// Remove is POST /invoice/remove: delete an invoice and un-issue its entries.
func (h *Handler) Remove(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	invoiceID := form.String("invoice_id")
	if invoiceID == "" {
		invalid(w, "Invalid request.")
		return
	}
	found, err := h.invoices.Remove(r.Context(), sess.CompanyID, store.ID(invoiceID))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Invoice not found.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Delete Successful.", Status: httpx.True()})
}

// Save is POST /invoice/save: issue (or re-issue) an invoice from selected entries.
func (h *Handler) Save(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)

	var entryIDs []string
	_ = form.JSONOr("entry_ids", "[]", &entryIDs)
	date := form.String("date")
	clientID := form.String("client_id")
	invNo := form.String("invNo")
	if len(entryIDs) == 0 || date == "" || clientID == "" || invNo == "" {
		invalid(w, "Invalid request.")
		return
	}
	ids := make([]store.ID, len(entryIDs))
	for i, s := range entryIDs {
		ids[i] = store.ID(s)
	}
	id, err := h.invoices.Save(r.Context(), sess.UID, sess.CompanyID, store.InvoiceSaveInput{
		Date: date, ClientID: store.ID(clientID), InvNo: invNo, EntryIDs: ids,
	})
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Saved Successful!", Data: string(id)})
}

// Paid is POST /invoice/paid: record a payment against an invoice.
func (h *Handler) Paid(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)

	invoiceID := form.String("invoice_id")
	if invoiceID == "" {
		invalid(w, "Invalid request.")
		return
	}
	received, _ := strconv.ParseFloat(form.String("receivedAmount"), 64)
	if !(received > 0) {
		invalid(w, "Amount must be greater than 0.")
		return
	}
	mode := "auto"
	if form.String("mode") == "manual" {
		mode = "manual"
	}
	in := store.InvoicePaidInput{
		InvoiceID: store.ID(invoiceID), ReceivedAmount: received, Mode: mode,
		Date: form.String("date"), BankID: store.ID(form.String("bank_id")), Note: form.String("note"),
	}
	if mode == "manual" {
		var incoming []struct {
			EntryID string  `json:"entry_id"`
			Amount  float64 `json:"amount"`
		}
		if err := form.JSONOr("allocations", "[]", &incoming); err != nil {
			invalid(w, "allocations must be a JSON array.")
			return
		}
		if len(incoming) == 0 {
			invalid(w, "Manual mode needs at least one allocation.")
			return
		}
		var allocated float64
		for _, a := range incoming {
			allocated += a.Amount
			in.Allocations = append(in.Allocations, store.InvoicePaidAlloc{EntryID: store.ID(a.EntryID), Amount: a.Amount})
		}
		if math.Abs(allocated-received) > 0.01 {
			invalid(w, "Allocate the full amount before saving.")
			return
		}
	}
	found, err := h.invoices.Paid(r.Context(), sess.CompanyID, in)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Invoice not found.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Paid Successful.", Status: httpx.True()})
}

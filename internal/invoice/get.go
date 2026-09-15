package invoice

import (
	"encoding/json"
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/entry"
	"github.com/mxnxn/invoicemg-go/internal/entrymath"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

// Get is POST /invoice/get: every invoice in the company as its raw document with entries and
// client populated, and uid replaced by the issuer's billing profile. Message is Node's
// "Successfully retreived!" (misspelling preserved), with no status field.
//
// Node reads the letterhead fields (account_no/ifsc/…) off the User; this port keeps the
// letterhead on the Company (as /invoice/getAll already does), so the uid profile is assembled
// from the active company plus the user's own email and name.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)

	list, err := h.invoices.List(ctx, sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	user, _ := h.users.FindByID(ctx, sess.UID)
	var company store.Company
	if sess.CompanyID != "" {
		company, _ = h.companies.Active(ctx, sess.CompanyID, sess.UID)
	}
	profile := map[string]any{
		"email": user.Email, "name": user.Name, "firm": company.Firm,
		"account_no": company.AccountNo, "ifsc": company.Ifsc, "phone": company.Phone,
		"address": company.Address, "bank_name": company.BankName, "url": company.URL,
		"gst": company.Gst,
	}

	out := make([]map[string]any, 0, len(list))
	for _, inv := range list {
		entries := make([]map[string]any, 0, len(inv.Entries))
		for _, e := range inv.Entries {
			entries = append(entries, entry.EntryJSON(e))
		}
		doc := map[string]any{
			"_id":         string(inv.ID),
			"invoiceId":   inv.InvoiceID,
			"amount":      inv.Amount,
			"totalAmount": inv.TotalAmount,
			"date":        inv.Date,
			"entries":     entries,
			"uid":         profile,
			"company_id":  string(sess.CompanyID),
			"createdAt":   httpx.NewTime(inv.CreatedAt),
			"__v":         0,
		}
		if inv.Client != nil {
			doc["client"] = map[string]any{
				"_id": string(inv.Client.ID), "uid": string(inv.Client.UID),
				"clientName": inv.Client.ClientName, "clientFirm": inv.Client.ClientFirm,
				"clientPhone": inv.Client.ClientPhone, "clientGST": inv.Client.ClientGST,
				"clientAddress": inv.Client.ClientAddress,
			}
		} else {
			doc["client"] = nil
		}
		out = append(out, doc)
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Successfully retreived!", Data: out})
}

// GetClientInvoices is POST /invoice/getClientInvoices: one client's invoices with per-invoice
// taxed/untaxed/received/due figures, plus totals and the client's received-payment history.
// The tax figure is Node's flat *1.18, not the per-entry rate, and due uses RoundOffWithAmount -
// both reproduced exactly. entries/client are emitted as ids (Node does not populate here).
func (h *Handler) GetClientInvoices(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)
	form, _ := httpx.ReadForm(r)
	cid := form.String("cid")
	if cid == "" {
		httpx.Invalid(w, "")
		return
	}

	list, err := h.invoices.List(ctx, sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	received, err := h.invoices.ReceivedByClient(ctx, sess.CompanyID, store.ID(cid))
	if err != nil {
		httpx.Internal(w, err)
		return
	}

	var totalDue, totalReceived float64
	var clientName, clientFirm string
	out := make([]map[string]any, 0)
	for _, inv := range list {
		if inv.Client == nil || string(inv.Client.ID) != cid {
			continue
		}
		if clientName == "" && clientFirm == "" {
			clientName, clientFirm = inv.Client.ClientName, inv.Client.ClientFirm
		}
		var nonTaxedValue, entryReceived, taxedValue float64
		entryIDs := make([]string, 0, len(inv.Entries))
		for _, e := range inv.Entries {
			if e.Total == 0 {
				entryReceived += e.Advance * 1.18
			} else {
				entryReceived += e.Advance
			}
			nonTaxedValue += e.Amount
			taxedValue += e.Amount * 1.18
			entryIDs = append(entryIDs, string(e.ID))
		}
		due := entrymath.RoundOffWithAmount(taxedValue) - entrymath.RoundOffWithAmount(inv.Amount)
		totalDue += due
		totalReceived += inv.Amount

		row := map[string]any{
			"_id": string(inv.ID), "invoiceId": inv.InvoiceID, "amount": inv.Amount,
			"totalAmount": inv.TotalAmount, "date": inv.Date, "company_id": string(sess.CompanyID),
			"client": cid, "entries": entryIDs, "createdAt": httpx.NewTime(inv.CreatedAt), "__v": 0,
			"nonTaxedValue": nonTaxedValue, "entryReceived": entryReceived,
			"taxedValue": taxedValue, "due": due,
		}
		out = append(out, row)
	}

	history := make([]map[string]any, 0, len(received))
	for _, rc := range received {
		hrow := map[string]any{
			"_id": string(rc.ID), "date": rc.Date, "amount": rc.Amount, "note": rc.Note,
			"client": cid, "createdAt": httpx.NewTime(rc.CreatedAt), "__v": rc.Version,
		}
		if rc.InvoiceID != "" {
			hrow["invoice_id"] = string(rc.InvoiceID)
		} else {
			hrow["invoice_id"] = nil
		}
		if rc.BankID != "" {
			hrow["bank_id"] = map[string]any{"_id": string(rc.BankID), "name": rc.BankName}
		} else {
			hrow["bank_id"] = nil
		}
		history = append(history, hrow)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = jsonEncode(w, map[string]any{
		"code": 200, "data": out, "totalDue": totalDue, "totalReceived": totalReceived,
		"clientName": clientName, "clientFirm": clientFirm, "status": true, "receivedHistory": history,
	})
}

func jsonEncode(w http.ResponseWriter, v any) error { return json.NewEncoder(w).Encode(v) }

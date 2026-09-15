package supplierpayment

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

func invalid(w http.ResponseWriter, msg string) {
	httpx.Write(w, httpx.Envelope{Code: 422, Message: msg, Status: httpx.False()})
}

// money is JS Number.toFixed(2) for the rupee-amount refusal messages.
func money(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

// round2 is Helpers/SupplierDues.roundMoney, for the handler-side allocatedTotal checks.
func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}

func spDTO(p store.SupplierPayment) map[string]any {
	dests := make([]map[string]any, 0, len(p.Destinations))
	for _, d := range p.Destinations {
		dests = append(dests, map[string]any{"kind": d.Kind, "id": d.ID, "label": d.Label, "amount": d.Amount})
	}
	row := map[string]any{
		"_id": string(p.ID), "uid": string(p.UID), "date": p.Date, "amount": p.Amount,
		"note": p.Note, "mode": p.Mode, "destinations": dests,
		"bank_id": bankObj(p.BankID, p.BankName), "createdAt": httpx.NewTime(p.CreatedAt), "__v": p.Version,
	}
	if p.SupplierID != "" {
		row["supplier_id"] = map[string]any{"_id": string(p.SupplierID), "name": p.SupplierName, "firm": p.SupplierFirm, "phone": p.SupplierPhone}
	} else {
		row["supplier_id"] = nil
	}
	return row
}

// OpenInvoices is POST /supplier-payment/lookups/open-invoices: a supplier's unpaid bills.
func (h *Handler) OpenInvoices(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	supplierID := form.String("supplier_id")
	if supplierID == "" {
		invalid(w, "Invalid request.")
		return
	}
	list, err := h.store.OpenInvoices(r.Context(), sess.UID, sess.CompanyID, store.ID(supplierID))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, v := range list {
		out = append(out, map[string]any{
			"_id": string(v.ID), "invoiceNumber": v.InvoiceNumber, "date": v.Date,
			"total": v.Total, "amount": v.Amount, "due": v.Due,
		})
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: out})
}

// Create is POST /supplier-payment/create: record a payment to a supplier.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)

	if !form.Has("supplier_id") || !form.Has("amount") || !form.Has("mode") {
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

	in := store.SupplierPaymentWrite{
		SupplierID: store.ID(form.String("supplier_id")), Amount: amount, Mode: mode,
		Note: form.String("note"), BankID: store.ID(form.String("bank_id")), Date: form.String("date"),
	}
	if mode == "manual" {
		var incoming []struct {
			PurchaseInvoiceID string  `json:"purchase_invoice_id"`
			Amount            float64 `json:"amount"`
		}
		if err := form.JSONOr("allocations", "[]", &incoming); err != nil {
			invalid(w, "allocations must be a JSON array.")
			return
		}
		if len(incoming) == 0 {
			invalid(w, "Manual mode needs at least one invoice allocation.")
			return
		}
		var allocated float64
		for _, a := range incoming {
			allocated += a.Amount
			in.Allocations = append(in.Allocations, store.SupplierAllocInput{InvoiceID: store.ID(a.PurchaseInvoiceID), Amount: a.Amount})
		}
		allocated = round2(allocated)
		if allocated > amount+0.01 {
			invalid(w, "Allocated amount exceeds the payment amount.")
			return
		}
		if allocated < amount-0.01 {
			invalid(w, fmt.Sprintf("₹%s of this payment is still unallocated. Allocate the full amount before saving.", money(round2(amount-allocated))))
			return
		}
	}

	saved, res, err := h.store.Create(r.Context(), sess.UID, sess.CompanyID, in)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	switch res.Status {
	case store.SupplierPayAutoUnallocated:
		invalid(w, fmt.Sprintf("₹%s of this payment can't be allocated - that's more than this supplier currently has outstanding. Reduce the amount, or record the extra once their next invoice arrives.", money(res.Remaining)))
	case store.SupplierPayInvoiceNotFound:
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "One of the selected invoices no longer exists.", Status: httpx.False()})
	case store.SupplierPayOverInvoice:
		invalid(w, fmt.Sprintf("₹%s is more than the ₹%s outstanding on that invoice.", money(res.Applied), money(res.Due)))
	default:
		httpx.Write(w, httpx.Envelope{Code: 200, Message: "Payment recorded.", Data: spDTO(saved)})
	}
}

// Delete is POST /supplier-payment/delete: reverse and remove a recorded payment.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	id := form.String("payment_id")
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
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Payment not found.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Payment deleted.", Status: httpx.True()})
}

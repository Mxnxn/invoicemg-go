// Package supplierpayment serves POST /supplier-payment/list from routes/SupplierPayment.js -
// payments to suppliers, newest first, supplier/bank populated with purchase-invoice
// destinations. Behind the purchase_invoices feature.
package supplierpayment

import (
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type Handler struct{ store store.SupplierPayments }

func New(s store.SupplierPayments) *Handler { return &Handler{store: s} }

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	list, err := h.store.List(r.Context(), sess.UID, sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, p := range list {
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
		out = append(out, row)
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: out})
}

func bankObj(id store.ID, name string) any {
	if id == "" {
		return nil
	}
	return map[string]any{"_id": string(id), "name": name}
}

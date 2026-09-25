package purchaseorder

import (
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/pochanges"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

// Share is POST /purchase-order/share and Confirm is /confirm (purchase_orders_send). The actual
// WhatsApp send is MOCKED in this port (no Meta call): the routes still enforce the gating and
// advance the send state so the buttons behave, but the template pipeline is not wired.
func (h *Handler) Share(w http.ResponseWriter, r *http.Request)   { h.performSend(w, r, "sent") }
func (h *Handler) Confirm(w http.ResponseWriter, r *http.Request) { h.performSend(w, r, "confirm") }

func (h *Handler) performSend(w http.ResponseWriter, r *http.Request, kind string) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	poID := form.String("po_id")
	if poID == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	po, _, _, found, err := h.store.Detail(r.Context(), sess.UID, sess.CompanyID, store.ID(poID))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Purchase order not found.", Status: httpx.False()})
		return
	}
	state := sendStateOf(po)
	if kind == "sent" && !state.CanShare {
		msg := "The supplier already has this version of the order."
		if po.Approval.State != "approved" {
			msg = "Approve this purchase order before sending it."
		}
		httpx.Write(w, httpx.Envelope{Code: 409, Message: msg, Status: httpx.False()})
		return
	}
	if kind == "confirm" && !state.CanConfirm {
		httpx.Write(w, httpx.Envelope{Code: 409, Message: "Approve this purchase order before confirming it.", Status: httpx.False()})
		return
	}
	if po.Supplier == nil || po.Supplier.Phone == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "This supplier has no phone number.", Status: httpx.False()})
		return
	}

	// (Meta send would happen here; mocked as success.)
	fp := pochanges.SendFingerprint(toFingerprintPO(po.SupplierID, po.Date, po.Total, po.Rows))
	updated, ok, err := h.store.RecordSend(r.Context(), sess.UID, sess.CompanyID, store.ID(poID), kind, fp)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !ok {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Purchase order not found.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Sent.", Status: httpx.True(), Data: toPODTO(updated)})
}

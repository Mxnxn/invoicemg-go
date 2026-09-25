package purchaseorder

import (
	"net/http"
	"strconv"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

// poActorID is the per-person key for dismissals: the person id for an employee, else the uid.
func poActorID(sess store.Session) store.ID {
	if sess.Role == "employee" {
		return sess.PersonID
	}
	return sess.UID
}

type pendingItem struct {
	ID           string  `json:"_id"`
	PoNumber     string  `json:"poNumber"`
	SupplierName string  `json:"supplierName"`
	Total        float64 `json:"total"`
	Date         string  `json:"date"`
	AgeDays      int     `json:"ageDays"`
}

// PendingApprovals is POST /purchase-order/pending-approvals (purchase_orders_approve): the bell.
func (h *Handler) PendingApprovals(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	pending, visible, err := h.store.PendingApprovals(r.Context(), sess.UID, sess.CompanyID, poActorID(sess))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	now := poClock().UnixMilli()
	items := make([]pendingItem, 0, 8)
	for i, po := range visible {
		if i >= 8 {
			break
		}
		name := "Unnamed supplier"
		if po.Supplier != nil {
			if po.Supplier.Firm != "" {
				name = po.Supplier.Firm
			} else if po.Supplier.Name != "" {
				name = po.Supplier.Name
			}
		}
		age := int((now - po.CreatedAt.UnixMilli()) / 86400000)
		if age < 0 {
			age = 0
		}
		items = append(items, pendingItem{
			ID: string(po.ID), PoNumber: po.PoNumber, SupplierName: name, Total: po.Total, Date: po.Date, AgeDays: age,
		})
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Status: httpx.True(), Data: map[string]any{
		"count": len(visible), "totalPending": len(pending), "items": items,
	}})
}

// ApprovalsDismiss is POST /purchase-order/approvals/dismiss (purchase_orders_approve).
func (h *Handler) ApprovalsDismiss(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	all := form.String("all") == "true"
	poID := form.String("po_id")
	if !all && poID == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	targets, freshVisible, freshTotal, err := h.store.DismissApprovals(r.Context(), sess.UID, sess.CompanyID, poActorID(sess), sess.Role, all, store.ID(poID))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if targets == 0 {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Nothing to dismiss.", Status: httpx.False()})
		return
	}
	msg := "Marked as read."
	if targets != 1 {
		msg = strconv.Itoa(targets) + " marked as read."
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: msg, Status: httpx.True(), Data: map[string]any{
		"count": freshVisible, "totalPending": freshTotal,
	}})
}

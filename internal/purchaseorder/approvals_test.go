package purchaseorder

import (
	"net/url"
	"testing"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

func TestPendingApprovals_CountsAndItems(t *testing.T) {
	poClock = func() time.Time { return time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC) }
	defer func() { poClock = func() time.Time { return time.Now().UTC() } }()
	created := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC) // 5 days old
	pending := make([]store.PurchaseOrder, 10)
	visible := make([]store.PurchaseOrder, 10)
	for i := range pending {
		po := store.PurchaseOrder{ID: store.ID("p" + string(rune('0'+i))), PoNumber: "PO-x", Total: 100, Date: "2026-09-10", CreatedAt: created, Supplier: &store.POSupplier{Firm: "Acme"}}
		pending[i] = po
		visible[i] = po
	}
	s := &stubPO{pending: pending, visible: visible}
	body := runForm(s, (*Handler).PendingApprovals, url.Values{})
	data := body["data"].(map[string]any)
	if data["count"] != float64(10) || data["totalPending"] != float64(10) {
		t.Errorf("counts = %v/%v, want 10/10", data["count"], data["totalPending"])
	}
	items := data["items"].([]any)
	if len(items) != 8 {
		t.Fatalf("items = %d, want 8 (capped)", len(items))
	}
	it := items[0].(map[string]any)
	if it["supplierName"] != "Acme" || it["ageDays"] != float64(5) {
		t.Errorf("item0 = %v, want Acme/age5", it)
	}
}

func TestDismiss_Validation(t *testing.T) {
	// neither all nor po_id -> 422
	if body := runForm(&stubPO{}, (*Handler).ApprovalsDismiss, url.Values{}); body["code"] != float64(422) {
		t.Errorf("no args -> %v, want 422", body["code"])
	}
	// nothing to dismiss -> 404
	if body := runForm(&stubPO{dismissTargets: 0}, (*Handler).ApprovalsDismiss, url.Values{"all": {"true"}}); body["code"] != float64(404) {
		t.Errorf("no targets -> %v, want 404", body["code"])
	}
}

func TestDismiss_Success(t *testing.T) {
	s := &stubPO{dismissTargets: 3, dismissVisible: 2, dismissTotal: 5}
	body := runForm(s, (*Handler).ApprovalsDismiss, url.Values{"all": {"true"}})
	if body["code"] != float64(200) || body["message"] != "3 marked as read." {
		t.Errorf("got code=%v msg=%v", body["code"], body["message"])
	}
	data := body["data"].(map[string]any)
	if data["count"] != float64(2) || data["totalPending"] != float64(5) {
		t.Errorf("fresh counts = %v/%v, want 2/5", data["count"], data["totalPending"])
	}
	if !s.gotDismissAll {
		t.Error("all flag not passed")
	}
	// single target -> singular message
	s2 := &stubPO{dismissTargets: 1, dismissVisible: 0, dismissTotal: 4}
	if body := runForm(s2, (*Handler).ApprovalsDismiss, url.Values{"po_id": {"p1"}}); body["message"] != "Marked as read." {
		t.Errorf("single -> %v, want singular", body["message"])
	}
	if s2.gotDismissID != "p1" {
		t.Errorf("po_id not passed: %q", s2.gotDismissID)
	}
}

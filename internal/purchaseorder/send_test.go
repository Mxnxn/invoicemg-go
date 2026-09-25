package purchaseorder

import (
	"net/url"
	"testing"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

func approvedPO() store.PurchaseOrder {
	return store.PurchaseOrder{ID: "p1", Approval: store.POApproval{State: "approved"}, Supplier: &store.POSupplier{Phone: "+91123"}, Rows: []store.PORow{{Material: "x", Qty: 1, Rate: 1}}}
}

func TestShare_GatingAndSuccess(t *testing.T) {
	// not approved -> 409
	draft := store.PurchaseOrder{ID: "p1", Approval: store.POApproval{State: "draft"}, Supplier: &store.POSupplier{Phone: "+91"}}
	if b := runForm(&stubPO{detailFound: true, detail: draft}, (*Handler).Share, url.Values{"po_id": {"p1"}}); b["code"] != float64(409) {
		t.Errorf("unapproved share -> %v, want 409", b["code"])
	}
	// approved, no phone -> 422
	noPhone := approvedPO()
	noPhone.Supplier = &store.POSupplier{}
	if b := runForm(&stubPO{detailFound: true, detail: noPhone}, (*Handler).Share, url.Values{"po_id": {"p1"}}); b["code"] != float64(422) {
		t.Errorf("no phone -> %v, want 422", b["code"])
	}
	// approved, phone, not yet sent -> 200 and records kind "sent"
	s := &stubPO{detailFound: true, detail: approvedPO(), recordSendFound: true, actionPO: approvedPO()}
	if b := runForm(s, (*Handler).Share, url.Values{"po_id": {"p1"}}); b["code"] != float64(200) || s.gotSendKind != "sent" {
		t.Errorf("share -> code=%v kind=%q", b["code"], s.gotSendKind)
	}
}

func TestConfirm(t *testing.T) {
	s := &stubPO{detailFound: true, detail: approvedPO(), recordSendFound: true, actionPO: approvedPO()}
	if b := runForm(s, (*Handler).Confirm, url.Values{"po_id": {"p1"}}); b["code"] != float64(200) || s.gotSendKind != "confirm" {
		t.Errorf("confirm -> code=%v kind=%q", b["code"], s.gotSendKind)
	}
}

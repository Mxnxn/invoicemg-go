package purchaseorder

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

func callPublic(s store.PurchaseOrders, supplierID, poID string) map[string]any {
	r := httptest.NewRequest("GET", "/po-public/"+supplierID+"/"+poID, nil)
	r.SetPathValue("supplier_id", supplierID)
	r.SetPathValue("po_id", poID)
	rec := httptest.NewRecorder()
	New(s).PublicView(rec, r)
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	return body
}

func TestPublicView_NotFound(t *testing.T) {
	if body := callPublic(&stubPO{publicFound: false}, "s1", "p1"); body["code"] != float64(404) {
		t.Errorf("not found -> %v, want 404", body["code"])
	}
}

func TestPublicView_Closed(t *testing.T) {
	body := callPublic(&stubPO{publicFound: true, public: store.PublicPO{Closed: true}}, "s1", "p1")
	data := body["data"].(map[string]any)
	if data["closed"] != true {
		t.Errorf("closed link -> %v, want {closed:true}", data)
	}
	if _, hasOrder := data["order"]; hasOrder {
		t.Error("a spent link must not carry the order")
	}
}

func TestPublicView_Open(t *testing.T) {
	s := &stubPO{publicFound: true, public: store.PublicPO{
		PoNumber: "PO-1", Date: "2026-09-01", Total: 236, SupplierName: "Acme", SupplierFirm: "Acme LLP",
		Rows:    []store.PORow{{Material: "Steel", Qty: 2, Rate: 100, Gst: 18}},
		Company: store.PublicPOCompany{Firm: "Us LLP", Gst: "GST9"},
	}}
	data := callPublic(s, "s1", "p1")["data"].(map[string]any)
	if data["closed"] != false {
		t.Fatalf("open link closed=%v", data["closed"])
	}
	order := data["order"].(map[string]any)
	if order["poNumber"] != "PO-1" || order["total"] != float64(236) {
		t.Errorf("order = %v", order)
	}
	sup := order["supplier"].(map[string]any)
	if sup["firm"] != "Acme LLP" {
		t.Errorf("supplier = %v (name+firm only)", sup)
	}
	if _, leaked := sup["phone"]; leaked {
		t.Error("public supplier must not expose phone")
	}
	if len(order["rows"].([]any)) != 1 {
		t.Errorf("rows = %v", order["rows"])
	}
	co := data["company"].(map[string]any)
	if co["firm"] != "Us LLP" || co["exportTemplate"] == nil {
		t.Errorf("company = %v", co)
	}
}

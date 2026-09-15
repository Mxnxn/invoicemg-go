package purchaseinvoice

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type stubPI struct{ list []store.PurchaseInvoice }

func (s *stubPI) List(_ context.Context, _, _ store.ID) ([]store.PurchaseInvoice, error) {
	return s.list, nil
}

func serve(t *testing.T, s store.PurchaseInvoices) map[string]any {
	t.Helper()
	r := httptest.NewRequest("POST", "/", nil)
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	New(s).List(rec, r)
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("not json: %v (%s)", err, rec.Body.String())
	}
	return out
}

func TestList_PopulatedSupplierAndRows(t *testing.T) {
	now := time.Date(2026, 9, 8, 5, 0, 0, 0, time.UTC)
	s := &stubPI{list: []store.PurchaseInvoice{{
		ID: "pi1", UID: "u1", CompanyID: "co1", InvoiceNumber: "MP-4471", Date: "2026-09-08",
		Total: 11800, Amount: 5000,
		Supplier:  &store.PurchaseSupplier{ID: "s1", Name: "Metro Papers", Firm: "Metro Pvt", Phone: "999"},
		Rows:      []store.PurchaseInvoiceRow{{ID: "r1", Material: "Art Paper", Qty: 100, Rate: 100, Unit: "Sheet"}},
		CreatedAt: now, UpdatedAt: now,
	}}}
	body := serve(t, s)
	inv := body["data"].([]any)[0].(map[string]any)
	if inv["invoiceNumber"] != "MP-4471" || inv["total"] != float64(11800) || inv["amount"] != float64(5000) {
		t.Errorf("top fields wrong: %v", inv)
	}
	sup, ok := inv["supplier_id"].(map[string]any)
	if !ok || sup["name"] != "Metro Papers" || sup["_id"] != "s1" {
		t.Errorf("supplier not populated: %v", inv["supplier_id"])
	}
	rows := inv["rows"].([]any)
	row := rows[0].(map[string]any)
	if row["material"] != "Art Paper" || row["unit"] != "Sheet" {
		t.Errorf("row wrong: %v", row)
	}
	// purchase rows default hasDimensions:false when unset
	if row["hasDimensions"] != false {
		t.Errorf("purchase row hasDimensions should default false, got %v", row["hasDimensions"])
	}
}

func TestList_NullSupplier(t *testing.T) {
	s := &stubPI{list: []store.PurchaseInvoice{{ID: "pi1", InvoiceNumber: "X", Supplier: nil, Rows: nil}}}
	inv := serve(t, s)["data"].([]any)[0].(map[string]any)
	if v, ok := inv["supplier_id"]; !ok || v != nil {
		t.Errorf("supplier_id must be present and null, got ok=%v v=%v", ok, v)
	}
	if _, ok := inv["rows"].([]any); !ok {
		t.Errorf("rows should be [], got %v", inv["rows"])
	}
}

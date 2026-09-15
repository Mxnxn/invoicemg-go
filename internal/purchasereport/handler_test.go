package purchasereport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"strings"
	"testing"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type stub struct {
	dues      store.SupplierDuesData
	statement store.SupplierStatementData
}

func (s *stub) SupplierDues(_ context.Context, _, _ store.ID) (store.SupplierDuesData, error) {
	return s.dues, nil
}
func (s *stub) SupplierStatement(_ context.Context, _, _, _ store.ID) (store.SupplierStatementData, error) {
	return s.statement, nil
}

func do(t *testing.T, s store.PurchaseReport, fn func(*Handler) func(http.ResponseWriter, *http.Request), form map[string]string) map[string]any {
	t.Helper()
	vals := neturl.Values{}
	for k, v := range form {
		vals.Set(k, v)
	}
	r := httptest.NewRequest("POST", "/", strings.NewReader(vals.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	fn(New(s))(rec, r)
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("not json: %v (%s)", err, rec.Body.String())
	}
	return out
}

func TestDues(t *testing.T) {
	s := &stub{dues: store.SupplierDuesData{
		Invoices: []store.SupplierDueInvoice{
			{SupplierID: "s1", Total: 1000, Amount: 400},
			{SupplierID: "s1", Total: 500, Amount: 500},
			{SupplierID: "s2", Total: 200, Amount: 0},
			{SupplierID: "gone", Total: 999, Amount: 0}, // supplier not in directory -> dropped
		},
		Suppliers: map[string]store.SupplierInfo{
			"s1": {ID: "s1", Name: "Metro", Firm: "MP"},
			"s2": {ID: "s2", Name: "Ink Co"},
		},
	}}
	body := do(t, s, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Dues }, map[string]string{})
	data := body["data"].(map[string]any)
	rows := data["rows"].([]any)
	if len(rows) != 2 {
		t.Fatalf("want 2 rows (gone dropped), got %d: %v", len(rows), rows)
	}
	// sorted by due desc: s1 due=600 first, s2 due=200
	first := rows[0].(map[string]any)
	if first["supplierName"] != "Metro" || first["due"] != float64(600) || first["billed"] != float64(1500) {
		t.Errorf("first row: %v", first)
	}
	totals := data["totals"].(map[string]any)
	if totals["due"] != float64(800) || totals["outstandingSuppliers"] != float64(2) {
		t.Errorf("totals: %v", totals)
	}
}

func TestSupplier(t *testing.T) {
	s := &stub{statement: store.SupplierStatementData{
		Found:    true,
		Supplier: store.SupplierInfo{Name: "Metro", Firm: "MP", GST: "G", Address: "Rd"},
		Invoices: []store.SupplierLedgerInvoice{{Date: "2026-09-10", InvoiceNumber: "PI/1", Total: 1000}},
		Payments: []store.SupplierLedgerPayment{{Date: "2026-09-12", Amount: 400, Note: "part"}},
	}}
	body := do(t, s, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Supplier }, map[string]string{"supplier_id": "s1"})
	data := body["data"].(map[string]any)
	if data["supplierName"] != "Metro" || data["currentBalance"] != float64(600) {
		t.Fatalf("supplier: %v", data)
	}
	rows := data["rows"].([]any)
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rows))
	}
	// invoice first (bill 1000, balance 1000), then payment (payment 400, balance 600)
	if rows[0].(map[string]any)["bill"] != float64(1000) || rows[1].(map[string]any)["balance"] != float64(600) {
		t.Errorf("ledger rows: %v", rows)
	}
	// missing supplier_id -> 422
	body = do(t, s, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Supplier }, map[string]string{})
	if body["code"] != float64(422) {
		t.Errorf("missing supplier_id: %v", body)
	}
	// not found -> 404
	body = do(t, &stub{statement: store.SupplierStatementData{Found: false}}, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Supplier }, map[string]string{"supplier_id": "x"})
	if body["code"] != float64(404) {
		t.Errorf("not found: %v", body)
	}
}

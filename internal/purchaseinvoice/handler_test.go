package purchaseinvoice

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"strings"
	"testing"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type stubPI struct {
	list      []store.PurchaseInvoice
	created   store.PurchaseInvoice
	updated   store.PurchaseInvoice
	updateRes store.PurchaseUpdateResult
	deleteRes store.PurchaseDeleteStatus
	gotCreate store.PurchaseInvoiceWrite
	gotUpdate store.PurchaseInvoiceUpdate
	numbers   []string
}

func (s *stubPI) List(_ context.Context, _, _ store.ID) ([]store.PurchaseInvoice, error) {
	return s.list, nil
}
func (s *stubPI) Create(_ context.Context, _, _ store.ID, in store.PurchaseInvoiceWrite) (store.PurchaseInvoice, error) {
	s.gotCreate = in
	return s.created, nil
}
func (s *stubPI) Update(_ context.Context, _, _, _ store.ID, in store.PurchaseInvoiceUpdate) (store.PurchaseInvoice, store.PurchaseUpdateResult, error) {
	s.gotUpdate = in
	return s.updated, s.updateRes, nil
}
func (s *stubPI) Delete(_ context.Context, _, _, _ store.ID) (store.PurchaseDeleteStatus, error) {
	return s.deleteRes, nil
}
func (s *stubPI) Numbers(_ context.Context, _ store.ID) ([]string, error) {
	return s.numbers, nil
}

func TestNextNumber_OnePastHighestInSeries(t *testing.T) {
	piClock = func() time.Time { return time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC) } // FY 2025-2026
	defer func() { piClock = func() time.Time { return time.Now().UTC() } }()

	s := &stubPI{numbers: []string{"MG/25-26/PINV-00004", "MG/25-26/PINV-00002", "MG/24-25/PINV-00009"}}
	r := httptest.NewRequest("POST", "/", nil)
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	New(s).NextNumber(rec, r)

	var out map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out["code"] != float64(200) {
		t.Fatalf("code = %v (%s)", out["code"], rec.Body.String())
	}
	data := out["data"].(map[string]any)
	// Highest in THIS FY's PINV series is 00004; last year's 00009 is a different series, ignored.
	if data["invoiceNumber"] != "MG/25-26/PINV-00005" {
		t.Errorf("invoiceNumber = %v, want MG/25-26/PINV-00005", data["invoiceNumber"])
	}
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

func postPI(t *testing.T, s store.PurchaseInvoices, fn func(*Handler) http.HandlerFunc, fields map[string]string) map[string]any {
	t.Helper()
	vals := neturl.Values{}
	for k, v := range fields {
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

func TestCreate_TotalAndCoercion(t *testing.T) {
	s := &stubPI{created: store.PurchaseInvoice{ID: "pi1"}}
	body := postPI(t, s, func(h *Handler) http.HandlerFunc { return h.Create }, map[string]string{
		"supplier_id": "s1", "date": "2026-09-15", "invoiceNumber": "BILL-1",
		"rows": `[{"material":"ACP","qty":"10","rate":45,"discount":50,"charges":20,"gst":18}]`,
	})
	if body["code"] != float64(200) || body["message"] != "Purchase invoice created." {
		t.Fatalf("envelope: %v", body)
	}
	// (10*45 - 50 + 20) * 1.18 = 420 * 1.18 = 495.6
	if s.gotCreate.Total < 495.59 || s.gotCreate.Total > 495.61 {
		t.Errorf("total = %v, want ~495.6", s.gotCreate.Total)
	}
	if len(s.gotCreate.Rows) != 1 || s.gotCreate.Rows[0].Qty != 10 {
		t.Errorf("rows: %+v", s.gotCreate.Rows)
	}
}

func TestCreate_Validation(t *testing.T) {
	body := postPI(t, &stubPI{}, func(h *Handler) http.HandlerFunc { return h.Create }, map[string]string{"supplier_id": "s1"})
	if body["code"] != float64(422) || body["message"] != "Invalid request." {
		t.Errorf("missing fields: %v", body)
	}
	body = postPI(t, &stubPI{}, func(h *Handler) http.HandlerFunc { return h.Create }, map[string]string{
		"supplier_id": "s1", "date": "d", "invoiceNumber": "n", "rows": "notjson",
	})
	if body["message"] != "rows must be a JSON array." {
		t.Errorf("bad rows: %v", body)
	}
	body = postPI(t, &stubPI{}, func(h *Handler) http.HandlerFunc { return h.Create }, map[string]string{
		"supplier_id": "s1", "date": "d", "invoiceNumber": "n", "rows": "[]",
	})
	if body["message"] != "A purchase invoice needs at least one row." {
		t.Errorf("empty rows: %v", body)
	}
}

func TestUpdate_PaidExceedsAndNotFound(t *testing.T) {
	// paid guard: message carries both amounts, toFixed(2).
	s := &stubPI{updateRes: store.PurchaseUpdateResult{Status: store.PurchaseUpdatePaidExceeds, AmountPaid: 1000, NewTotal: 495.6}}
	body := postPI(t, s, func(h *Handler) http.HandlerFunc { return h.Update }, map[string]string{
		"purchase_invoice_id": "pi1", "rows": `[{"material":"ACP","qty":1,"rate":1}]`,
	})
	if body["code"] != float64(422) {
		t.Fatalf("paid guard: %v", body)
	}
	msg, _ := body["message"].(string)
	if !strings.Contains(msg, "1000.00") || !strings.Contains(msg, "495.60") {
		t.Errorf("refusal message amounts: %q", msg)
	}

	body = postPI(t, &stubPI{updateRes: store.PurchaseUpdateResult{Status: store.PurchaseUpdateNotFound}},
		func(h *Handler) http.HandlerFunc { return h.Update }, map[string]string{"purchase_invoice_id": "pi9"})
	if body["code"] != float64(404) {
		t.Errorf("not found: %v", body)
	}

	body = postPI(t, &stubPI{}, func(h *Handler) http.HandlerFunc { return h.Update }, map[string]string{})
	if body["code"] != float64(422) {
		t.Errorf("missing id: %v", body)
	}
}

func TestUpdate_Success(t *testing.T) {
	s := &stubPI{updateRes: store.PurchaseUpdateResult{Status: store.PurchaseUpdateOK}, updated: store.PurchaseInvoice{ID: "pi1"}}
	body := postPI(t, s, func(h *Handler) http.HandlerFunc { return h.Update }, map[string]string{
		"purchase_invoice_id": "pi1", "date": "2026-10-01",
	})
	if body["code"] != float64(200) || body["message"] != "Purchase invoice updated." {
		t.Fatalf("envelope: %v", body)
	}
	if s.gotUpdate.Date == nil || *s.gotUpdate.Date != "2026-10-01" || s.gotUpdate.Rows != nil {
		t.Errorf("patch: date=%v rows=%v", s.gotUpdate.Date, s.gotUpdate.Rows)
	}
}

func TestDelete(t *testing.T) {
	body := postPI(t, &stubPI{deleteRes: store.PurchaseDeleteOK}, func(h *Handler) http.HandlerFunc { return h.Delete }, map[string]string{"purchase_invoice_id": "pi1"})
	if body["code"] != float64(200) || body["status"] != true || body["message"] != "Purchase invoice deleted." {
		t.Errorf("delete ok: %v", body)
	}
	body = postPI(t, &stubPI{deleteRes: store.PurchaseDeleteHasPayment}, func(h *Handler) http.HandlerFunc { return h.Delete }, map[string]string{"purchase_invoice_id": "pi1"})
	if body["code"] != float64(422) {
		t.Errorf("has payment: %v", body)
	}
	body = postPI(t, &stubPI{deleteRes: store.PurchaseDeleteNotFound}, func(h *Handler) http.HandlerFunc { return h.Delete }, map[string]string{"purchase_invoice_id": "pi1"})
	if body["code"] != float64(404) {
		t.Errorf("not found: %v", body)
	}
	body = postPI(t, &stubPI{}, func(h *Handler) http.HandlerFunc { return h.Delete }, map[string]string{})
	if body["code"] != float64(422) {
		t.Errorf("missing id: %v", body)
	}
}

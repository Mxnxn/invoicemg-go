package supplierpayment

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
	open        []store.SupplierOpenInvoice
	created     store.SupplierPayment
	createRes   store.SupplierPayResult
	deleteFound bool
	gotCreate   store.SupplierPaymentWrite
}

func (s *stub) List(context.Context, store.ID, store.ID) ([]store.SupplierPayment, error) {
	return nil, nil
}
func (s *stub) OpenInvoices(context.Context, store.ID, store.ID, store.ID) ([]store.SupplierOpenInvoice, error) {
	return s.open, nil
}
func (s *stub) Create(_ context.Context, _, _ store.ID, in store.SupplierPaymentWrite) (store.SupplierPayment, store.SupplierPayResult, error) {
	s.gotCreate = in
	return s.created, s.createRes, nil
}
func (s *stub) Delete(_ context.Context, _, _, _ store.ID) (bool, error) {
	return s.deleteFound, nil
}

func post(t *testing.T, s store.SupplierPayments, fn func(*Handler) http.HandlerFunc, fields map[string]string) map[string]any {
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

func TestOpenInvoices(t *testing.T) {
	s := &stub{open: []store.SupplierOpenInvoice{{ID: "pi1", InvoiceNumber: "BILL-1", Total: 1000, Amount: 400, Due: 600}}}
	body := post(t, s, func(h *Handler) http.HandlerFunc { return h.OpenInvoices }, map[string]string{"supplier_id": "s1"})
	row := body["data"].([]any)[0].(map[string]any)
	if row["due"] != float64(600) || row["invoiceNumber"] != "BILL-1" {
		t.Errorf("row: %v", row)
	}
	body = post(t, &stub{}, func(h *Handler) http.HandlerFunc { return h.OpenInvoices }, map[string]string{})
	if body["code"] != float64(422) {
		t.Errorf("missing supplier_id: %v", body)
	}
}

func TestCreate_ValidationBranches(t *testing.T) {
	body := post(t, &stub{}, func(h *Handler) http.HandlerFunc { return h.Create }, map[string]string{"supplier_id": "s1", "amount": "500"})
	if body["message"] != "Invalid request." {
		t.Errorf("missing mode: %v", body)
	}
	body = post(t, &stub{}, func(h *Handler) http.HandlerFunc { return h.Create }, map[string]string{"supplier_id": "s1", "amount": "0", "mode": "auto"})
	if body["message"] != "Amount must be greater than 0." {
		t.Errorf("amount: %v", body)
	}
	body = post(t, &stub{}, func(h *Handler) http.HandlerFunc { return h.Create }, map[string]string{"supplier_id": "s1", "amount": "5", "mode": "x"})
	if body["message"] != "mode must be 'auto' or 'manual'." {
		t.Errorf("mode: %v", body)
	}
}

func TestCreate_ManualAllocationChecks(t *testing.T) {
	// over-allocated
	body := post(t, &stub{}, func(h *Handler) http.HandlerFunc { return h.Create }, map[string]string{
		"supplier_id": "s1", "amount": "500", "mode": "manual",
		"allocations": `[{"purchase_invoice_id":"pi1","amount":600}]`,
	})
	if body["message"] != "Allocated amount exceeds the payment amount." {
		t.Errorf("over: %v", body)
	}
	// under-allocated -> unallocated message with remainder
	body = post(t, &stub{}, func(h *Handler) http.HandlerFunc { return h.Create }, map[string]string{
		"supplier_id": "s1", "amount": "500", "mode": "manual",
		"allocations": `[{"purchase_invoice_id":"pi1","amount":300}]`,
	})
	msg, _ := body["message"].(string)
	if !strings.Contains(msg, "200.00") || !strings.Contains(msg, "still unallocated") {
		t.Errorf("under: %q", msg)
	}
	// empty
	body = post(t, &stub{}, func(h *Handler) http.HandlerFunc { return h.Create }, map[string]string{
		"supplier_id": "s1", "amount": "500", "mode": "manual", "allocations": "[]",
	})
	if body["message"] != "Manual mode needs at least one invoice allocation." {
		t.Errorf("empty: %v", body)
	}
}

func TestCreate_StoreResults(t *testing.T) {
	// auto unallocated
	body := post(t, &stub{createRes: store.SupplierPayResult{Status: store.SupplierPayAutoUnallocated, Remaining: 150.5}},
		func(h *Handler) http.HandlerFunc { return h.Create }, map[string]string{"supplier_id": "s1", "amount": "500", "mode": "auto"})
	if msg, _ := body["message"].(string); !strings.Contains(msg, "150.50") || !strings.Contains(msg, "can't be allocated") {
		t.Errorf("auto unallocated: %q", msg)
	}
	// over invoice (manual full allocation but store says exceeds due)
	body = post(t, &stub{createRes: store.SupplierPayResult{Status: store.SupplierPayOverInvoice, Applied: 500, Due: 300}},
		func(h *Handler) http.HandlerFunc { return h.Create }, map[string]string{
			"supplier_id": "s1", "amount": "500", "mode": "manual", "allocations": `[{"purchase_invoice_id":"pi1","amount":500}]`,
		})
	if msg, _ := body["message"].(string); !strings.Contains(msg, "500.00") || !strings.Contains(msg, "300.00") {
		t.Errorf("over invoice: %q", msg)
	}
	// invoice not found -> 404
	body = post(t, &stub{createRes: store.SupplierPayResult{Status: store.SupplierPayInvoiceNotFound}},
		func(h *Handler) http.HandlerFunc { return h.Create }, map[string]string{
			"supplier_id": "s1", "amount": "500", "mode": "manual", "allocations": `[{"purchase_invoice_id":"pi9","amount":500}]`,
		})
	if body["code"] != float64(404) {
		t.Errorf("not found: %v", body)
	}
	// ok
	s := &stub{created: store.SupplierPayment{ID: "sp1"}, createRes: store.SupplierPayResult{Status: store.SupplierPayOK}}
	body = post(t, s, func(h *Handler) http.HandlerFunc { return h.Create }, map[string]string{"supplier_id": "s1", "amount": "500", "mode": "auto"})
	if body["code"] != float64(200) || body["message"] != "Payment recorded." {
		t.Errorf("ok: %v", body)
	}
	if s.gotCreate.Mode != "auto" || s.gotCreate.Amount != 500 {
		t.Errorf("passed: %+v", s.gotCreate)
	}
}

func TestDelete(t *testing.T) {
	body := post(t, &stub{deleteFound: true}, func(h *Handler) http.HandlerFunc { return h.Delete }, map[string]string{"payment_id": "sp1"})
	if body["code"] != float64(200) || body["status"] != true || body["message"] != "Payment deleted." {
		t.Errorf("delete ok: %v", body)
	}
	body = post(t, &stub{deleteFound: false}, func(h *Handler) http.HandlerFunc { return h.Delete }, map[string]string{"payment_id": "sp1"})
	if body["code"] != float64(404) {
		t.Errorf("404: %v", body)
	}
	body = post(t, &stub{}, func(h *Handler) http.HandlerFunc { return h.Delete }, map[string]string{})
	if body["code"] != float64(422) {
		t.Errorf("missing id: %v", body)
	}
}

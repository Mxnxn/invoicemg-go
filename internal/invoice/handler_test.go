package invoice

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type stubInvoices struct{ list []store.Invoice }

func (s *stubInvoices) List(_ context.Context, _ store.ID) ([]store.Invoice, error) {
	return s.list, nil
}

type stubCompanies struct{ c store.Company }

func (s *stubCompanies) List(_ context.Context, _ store.ID) ([]store.Company, error) { return nil, nil }
func (s *stubCompanies) Active(_ context.Context, _, _ store.ID) (store.Company, error) {
	return s.c, nil
}

type stubUsers struct{ u store.User }

func (s *stubUsers) FindByEmail(_ context.Context, _ string) (store.User, error) {
	return store.User{}, store.ErrNotFound
}
func (s *stubUsers) FindByID(_ context.Context, _ store.ID) (store.User, error) { return s.u, nil }
func (s *stubUsers) CreateSession(_ context.Context, _ store.NewSession) (store.Session, error) {
	return store.Session{}, nil
}

func serve(t *testing.T, inv []store.Invoice, c store.Company, u store.User) map[string]any {
	t.Helper()
	h := New(&stubInvoices{list: inv}, &stubCompanies{c: c}, &stubUsers{u: u})
	r := httptest.NewRequest("POST", "/", nil)
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	h.List(rec, r)
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("not json: %v (%s)", err, rec.Body.String())
	}
	return out
}

func TestGetAll_Shape(t *testing.T) {
	now := time.Date(2026, 9, 11, 3, 0, 0, 0, time.UTC)
	inv := []store.Invoice{{
		ID: "iv1", InvoiceID: "INV/26-27/000001", Date: "2026-09-11", Amount: 5000, TotalAmount: 11800,
		Client:    &store.InvoiceClient{ID: "c1", UID: "u1", ClientName: "Priya", ClientFirm: "Acme"},
		Entries:   []store.Entry{{ID: "e1", Material: "Vinyl", Amount: 4500, Cgst: 9, Sgst: 9}},
		CreatedAt: now,
	}}
	company := store.Company{Firm: "Manan LLP", Gst: "GST1", Phone: "999", AccountNo: "ACC", Ifsc: "IF", BankName: "HDFC", URL: "logo.png"}
	user := store.User{Name: "Owner", Email: "o@x.test"}
	body := serve(t, inv, company, user)

	if body["message"] != "Successful!" {
		t.Errorf("message = %v, want Successful!", body["message"])
	}
	if _, ok := body["status"]; ok {
		t.Error("getAll sends no status field")
	}
	row := body["data"].([]any)[0].(map[string]any)
	if row["invoiceNumber"] != "INV/26-27/000001" {
		t.Errorf("invoiceNumber = %v", row["invoiceNumber"])
	}
	// issuer letterhead merged in
	if row["firm"] != "Manan LLP" || row["gst"] != "GST1" || row["name"] != "Owner" || row["email"] != "o@x.test" {
		t.Errorf("issuer fields wrong: %v", row)
	}
	// client fields flattened
	if row["clientName"] != "Priya" || row["client_id"] != "c1" {
		t.Errorf("client fields wrong: %v", row)
	}
	// total prefers stored totalAmount; receivedAmount is invoice.amount; notTaxAmount sums entry.amount
	if row["total"] != float64(11800) {
		t.Errorf("total = %v, want 11800", row["total"])
	}
	if row["receivedAmount"] != float64(5000) {
		t.Errorf("receivedAmount = %v, want 5000", row["receivedAmount"])
	}
	if row["notTaxAmount"] != float64(4500) {
		t.Errorf("notTaxAmount = %v, want 4500", row["notTaxAmount"])
	}
	if len(row["entries"].([]any)) != 1 {
		t.Errorf("entries length wrong: %v", row["entries"])
	}
}

// With no stored totalAmount, total falls back to the summed rupee-rounded entry totals.
func TestGetAll_TotalFallback(t *testing.T) {
	inv := []store.Invoice{{
		ID: "iv1", InvoiceID: "X", TotalAmount: 0,
		Entries: []store.Entry{{Amount: 100, Cgst: 9, Sgst: 9}}, // entryTotal 118 -> RoundOffWithAmount 118
	}}
	body := serve(t, inv, store.Company{}, store.User{})
	row := body["data"].([]any)[0].(map[string]any)
	if row["total"] != float64(118) {
		t.Errorf("fallback total = %v, want 118", row["total"])
	}
	// no client -> client_id and uid null
	if v, ok := row["client_id"]; !ok || v != nil {
		t.Errorf("client_id should be null, got %v", v)
	}
}

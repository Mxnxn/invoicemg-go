package invoice

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

type stubInvoices struct {
	list             []store.Invoice
	numbers          []string
	labels           map[string]string
	received         []store.InvoiceReceivedRow
	receivedByClient []store.InvoiceReceivedRow
	savedID          store.ID
	paidFound        bool
	removeFound      bool
	gotSave          store.InvoiceSaveInput
	gotPaid          store.InvoicePaidInput
	history          store.InvoiceHistory
	historyFound     bool
}

func (s *stubInvoices) List(_ context.Context, _ store.ID) ([]store.Invoice, error) {
	return s.list, nil
}
func (s *stubInvoices) Numbers(_ context.Context, _ store.ID) ([]string, error) {
	return s.numbers, nil
}
func (s *stubInvoices) EntryJobLabels(_ context.Context, _ store.ID, _ []store.ID) (map[string]string, error) {
	return s.labels, nil
}
func (s *stubInvoices) ReceivedByClient(_ context.Context, _, _ store.ID) ([]store.InvoiceReceivedRow, error) {
	return s.receivedByClient, nil
}
func (s *stubInvoices) Received(_ context.Context, _, _ store.ID) ([]store.InvoiceReceivedRow, error) {
	return s.received, nil
}
func (s *stubInvoices) Save(_ context.Context, _, _ store.ID, in store.InvoiceSaveInput) (store.ID, error) {
	s.gotSave = in
	return s.savedID, nil
}
func (s *stubInvoices) Paid(_ context.Context, _ store.ID, in store.InvoicePaidInput) (bool, error) {
	s.gotPaid = in
	return s.paidFound, nil
}
func (s *stubInvoices) Remove(_ context.Context, _, _ store.ID) (bool, error) {
	return s.removeFound, nil
}
func (s *stubInvoices) History(_ context.Context, _, _ store.ID) (store.InvoiceHistory, bool, error) {
	return s.history, s.historyFound, nil
}

type stubCompanies struct{ c store.Company }

func (s *stubCompanies) List(_ context.Context, _ store.ID) ([]store.Company, error) { return nil, nil }
func (s *stubCompanies) Active(_ context.Context, _, _ store.ID) (store.Company, error) {
	return s.c, nil
}
func (s *stubCompanies) Count(_ context.Context, _ store.ID) (int, error) { return 0, nil }
func (s *stubCompanies) Scope(context.Context, store.ID, store.ID) ([]store.ID, bool, map[store.ID]string, error) {
	return nil, false, nil, nil
}
func (s *stubCompanies) SetReportsAcrossCompanies(context.Context, store.ID, store.ID, bool) (bool, error) {
	return false, nil
}
func (s *stubCompanies) Create(_ context.Context, _ store.ID, _ store.CompanyWrite) (store.Company, error) {
	return store.Company{}, nil
}
func (s *stubCompanies) Update(_ context.Context, _, _ store.ID, _ store.CompanyPatch) (store.Company, bool, error) {
	return store.Company{}, false, nil
}
func (s *stubCompanies) FindActive(_ context.Context, _, _ store.ID) (store.Company, bool, error) {
	return store.Company{}, false, nil
}
func (s *stubCompanies) Deactivate(_ context.Context, _, _ store.ID) (store.DeactivateResult, error) {
	return store.DeactivateOK, nil
}

type stubUsers struct{ u store.User }

// stubJobsX embeds store.Jobs so it satisfies the interface; only the methods the invoice routes
// call are overridden.
type stubJobsX struct {
	store.Jobs
	invoiceable []store.InvoiceableJob
}

func (s stubJobsX) InvoiceableJobs(_ context.Context, _, _ store.ID) ([]store.InvoiceableJob, error) {
	return s.invoiceable, nil
}

func (s *stubUsers) UpdateProfile(_ context.Context, _ store.ID, _, _ string) (store.User, bool, error) {
	return s.u, true, nil
}
func (s *stubUsers) FindByEmail(_ context.Context, _ string) (store.User, error) {
	return store.User{}, store.ErrNotFound
}
func (s *stubUsers) FindByID(_ context.Context, _ store.ID) (store.User, error) { return s.u, nil }
func (s *stubUsers) CreateSession(_ context.Context, _ store.NewSession) (store.Session, error) {
	return store.Session{}, nil
}

func serve(t *testing.T, inv []store.Invoice, c store.Company, u store.User) map[string]any {
	t.Helper()
	h := New(&stubInvoices{list: inv}, &stubCompanies{c: c}, &stubUsers{u: u}, stubClientsX{}, stubJobsX{}, "")
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

func postInv(t *testing.T, s *stubInvoices, fn func(*Handler) http.HandlerFunc, fields map[string]string) map[string]any {
	t.Helper()
	vals := neturl.Values{}
	for k, v := range fields {
		vals.Set(k, v)
	}
	r := httptest.NewRequest("POST", "/", strings.NewReader(vals.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	fn(New(s, &stubCompanies{}, &stubUsers{}, stubClientsX{}, stubJobsX{}, ""))(rec, r)
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("not json: %v (%s)", err, rec.Body.String())
	}
	return out
}

func TestNextNumber(t *testing.T) {
	s := &stubInvoices{numbers: []string{"MG/26-27/INV-00002"}}
	h := New(s, &stubCompanies{}, &stubUsers{}, stubClientsX{}, stubJobsX{}, "")
	now = func() time.Time { return time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC) }
	defer func() { now = func() time.Time { return time.Now().UTC() } }()
	r := httptest.NewRequest("POST", "/", nil)
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	h.NextNumber(rec, r)
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["data"].(map[string]any)["invoiceNumber"] != "MG/26-27/INV-00003" {
		t.Errorf("next: %v", body["data"])
	}
}

func TestEntriesJobs(t *testing.T) {
	s := &stubInvoices{labels: map[string]string{"e1": "JOB/1"}}
	body := postInv(t, s, func(h *Handler) http.HandlerFunc { return h.EntriesJobs }, map[string]string{"entry_ids": `["e1"]`})
	if body["data"].(map[string]any)["e1"] != "JOB/1" {
		t.Errorf("labels: %v", body["data"])
	}
	// empty ids -> empty object, still 200
	body = postInv(t, &stubInvoices{}, func(h *Handler) http.HandlerFunc { return h.EntriesJobs }, map[string]string{"entry_ids": "[]"})
	if body["code"] != float64(200) {
		t.Errorf("empty: %v", body)
	}
}

func TestGetReceived(t *testing.T) {
	s := &stubInvoices{received: []store.InvoiceReceivedRow{{ID: "rc1", Amount: 500, BankID: "b1", BankName: "HDFC", InvoiceID: "inv1"}}}
	body := postInv(t, s, func(h *Handler) http.HandlerFunc { return h.GetReceived }, map[string]string{"invoice_id": "inv1"})
	row := body["data"].([]any)[0].(map[string]any)
	if row["amount"] != float64(500) || row["bank_id"].(map[string]any)["name"] != "HDFC" {
		t.Errorf("received row: %v", row)
	}
	body = postInv(t, &stubInvoices{}, func(h *Handler) http.HandlerFunc { return h.GetReceived }, map[string]string{})
	if body["code"] != float64(422) {
		t.Errorf("missing id: %v", body)
	}
}

func TestSave(t *testing.T) {
	s := &stubInvoices{savedID: "inv9"}
	body := postInv(t, s, func(h *Handler) http.HandlerFunc { return h.Save }, map[string]string{
		"entry_ids": `["e1","e2"]`, "date": "2026-09-15", "client_id": "c1", "invNo": "MG/26-27/INV-00001",
	})
	if body["code"] != float64(200) || body["message"] != "Saved Successful!" || body["data"] != "inv9" {
		t.Fatalf("save: %v", body)
	}
	if len(s.gotSave.EntryIDs) != 2 || s.gotSave.InvNo != "MG/26-27/INV-00001" {
		t.Errorf("save input: %+v", s.gotSave)
	}
	// missing fields
	body = postInv(t, &stubInvoices{}, func(h *Handler) http.HandlerFunc { return h.Save }, map[string]string{"date": "x"})
	if body["code"] != float64(422) {
		t.Errorf("validation: %v", body)
	}
}

func TestPaid(t *testing.T) {
	s := &stubInvoices{paidFound: true}
	body := postInv(t, s, func(h *Handler) http.HandlerFunc { return h.Paid }, map[string]string{"invoice_id": "inv1", "receivedAmount": "500", "mode": "auto"})
	if body["code"] != float64(200) || body["message"] != "Paid Successful." {
		t.Fatalf("paid: %v", body)
	}
	if s.gotPaid.Mode != "auto" || s.gotPaid.ReceivedAmount != 500 {
		t.Errorf("paid input: %+v", s.gotPaid)
	}
	// amount not > 0
	body = postInv(t, &stubInvoices{}, func(h *Handler) http.HandlerFunc { return h.Paid }, map[string]string{"invoice_id": "inv1", "receivedAmount": "0"})
	if body["message"] != "Amount must be greater than 0." {
		t.Errorf("amount: %v", body)
	}
	// manual under-allocated
	body = postInv(t, &stubInvoices{}, func(h *Handler) http.HandlerFunc { return h.Paid }, map[string]string{
		"invoice_id": "inv1", "receivedAmount": "500", "mode": "manual", "allocations": `[{"entry_id":"e1","amount":300}]`,
	})
	if body["message"] != "Allocate the full amount before saving." {
		t.Errorf("manual under: %v", body)
	}
	// not found
	body = postInv(t, &stubInvoices{paidFound: false}, func(h *Handler) http.HandlerFunc { return h.Paid }, map[string]string{"invoice_id": "inv9", "receivedAmount": "500", "mode": "auto"})
	if body["code"] != float64(404) {
		t.Errorf("not found: %v", body)
	}
}

func TestRemove(t *testing.T) {
	body := postInv(t, &stubInvoices{removeFound: true}, func(h *Handler) http.HandlerFunc { return h.Remove }, map[string]string{"invoice_id": "inv1"})
	if body["code"] != float64(200) || body["message"] != "Delete Successful." {
		t.Errorf("remove: %v", body)
	}
	body = postInv(t, &stubInvoices{removeFound: false}, func(h *Handler) http.HandlerFunc { return h.Remove }, map[string]string{"invoice_id": "inv1"})
	if body["code"] != float64(404) {
		t.Errorf("404: %v", body)
	}
}

func TestGet_RawShapeWithProfile(t *testing.T) {
	inv := []store.Invoice{{
		ID: "iv1", InvoiceID: "INV/1", Date: "2026-09-11", Amount: 5000, TotalAmount: 11800,
		Client:  &store.InvoiceClient{ID: "c1", UID: "u1", ClientName: "Priya", ClientFirm: "Acme"},
		Entries: []store.Entry{{ID: "e1", Material: "Vinyl", Amount: 4500}},
	}}
	h := New(&stubInvoices{list: inv},
		&stubCompanies{c: store.Company{Firm: "Manan LLP", Gst: "GST1", BankName: "HDFC"}},
		&stubUsers{u: store.User{Name: "Owner", Email: "o@x.test"}}, stubClientsX{}, stubJobsX{}, "")
	r := httptest.NewRequest("POST", "/", nil)
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	h.Get(rec, r)
	var out map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out["message"] != "Successfully retreived!" {
		t.Fatalf("message: %v", out["message"])
	}
	row := out["data"].([]any)[0].(map[string]any)
	if row["invoiceId"] != "INV/1" || row["client"].(map[string]any)["clientName"] != "Priya" {
		t.Errorf("raw shape: %v", row)
	}
	uid := row["uid"].(map[string]any)
	if uid["name"] != "Owner" || uid["firm"] != "Manan LLP" || uid["bank_name"] != "HDFC" {
		t.Errorf("uid profile: %v", uid)
	}
	if len(row["entries"].([]any)) != 1 {
		t.Errorf("entries not populated: %v", row["entries"])
	}
}

func TestGetClientInvoices(t *testing.T) {
	inv := []store.Invoice{
		{ID: "iv1", InvoiceID: "INV/1", Date: "2026-09-11", Amount: 1000,
			Client:  &store.InvoiceClient{ID: "c1", ClientName: "Priya", ClientFirm: "Acme"},
			Entries: []store.Entry{{ID: "e1", Amount: 1000, Advance: 500, Total: 680}}},
		{ID: "iv2", InvoiceID: "INV/2", Amount: 0,
			Client:  &store.InvoiceClient{ID: "c2", ClientName: "Other"},
			Entries: []store.Entry{{ID: "e9", Amount: 999}}},
	}
	s := &stubInvoices{list: inv, receivedByClient: []store.InvoiceReceivedRow{{ID: "r1", Amount: 300, Date: "2026-09-12"}}}
	h := New(s, &stubCompanies{}, &stubUsers{}, stubClientsX{}, stubJobsX{}, "")
	r := httptest.NewRequest("POST", "/", strings.NewReader("cid=c1"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	h.GetClientInvoices(rec, r)
	var out map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out["clientName"] != "Priya" || out["status"] != true {
		t.Fatalf("top-level: %v", out)
	}
	data := out["data"].([]any)
	if len(data) != 1 { // only c1's invoice
		t.Fatalf("want 1 invoice, got %d", len(data))
	}
	row := data[0].(map[string]any)
	// amount 1000 -> taxedValue 1180, nonTaxedValue 1000, entryReceived 500 (total != 0)
	if row["nonTaxedValue"] != float64(1000) || row["taxedValue"].(float64) < 1179.9 || row["entryReceived"] != float64(500) {
		t.Errorf("per-invoice math: %v", row)
	}
	if len(out["receivedHistory"].([]any)) != 1 {
		t.Errorf("receivedHistory: %v", out["receivedHistory"])
	}
	// missing cid -> 422
	r2 := httptest.NewRequest("POST", "/", nil)
	r2 = r2.WithContext(auth.WithSession(r2.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec2 := httptest.NewRecorder()
	h.GetClientInvoices(rec2, r2)
	json.Unmarshal(rec2.Body.Bytes(), &out)
	if out["code"] != float64(422) {
		t.Errorf("missing cid should 422: %v", out)
	}
}

// stubClientsX satisfies store.Clients for the export handler (only Get is exercised, and only
// the export tests drive it - the other tests never call it).
type stubClientsX struct {
	detail store.ClientDetail
	found  bool
}

func (s stubClientsX) Visible(context.Context, store.ID, store.ID) ([]store.Client, error) {
	return nil, nil
}
func (s stubClientsX) SharedList(context.Context, []store.ID) ([]store.SharedClient, error) {
	return nil, nil
}
func (s stubClientsX) OwnedSharing(context.Context, store.ID, store.ID) ([]store.ID, bool, error) {
	return nil, false, nil
}
func (s stubClientsX) SetSharing(context.Context, store.ID, store.ID, []store.ID) ([]store.ID, bool, error) {
	return nil, false, nil
}
func (s stubClientsX) Create(context.Context, store.ID, store.ID, int64, store.ClientWrite) (store.Client, store.Dup, error) {
	return store.Client{}, "", nil
}
func (s stubClientsX) Update(context.Context, store.ID, store.ID, store.ClientWrite) (store.Client, store.Dup, bool, error) {
	return store.Client{}, "", false, nil
}
func (s stubClientsX) Delete(context.Context, store.ID, store.ID) (bool, error) { return false, nil }
func (s stubClientsX) EnsureSupplier(context.Context, store.ID, store.ClientWrite) (bool, error) {
	return false, nil
}
func (s stubClientsX) Get(context.Context, store.ID, store.ID) (store.ClientDetail, bool, error) {
	return s.detail, s.found, nil
}
func (s stubClientsX) SetNotifyPreference(context.Context, store.ID, store.ID, string, *bool) (*bool, *bool, bool, error) {
	return nil, nil, false, nil
}
func (s stubClientsX) NotifyPreferences(context.Context, store.ID, bool) ([]store.ClientNotify, error) {
	return nil, nil
}

func (s *stubUsers) UpdatePassword(context.Context, store.ID, string) (bool, error) {
	return false, nil
}

func (s *stubCompanies) SetQueueOrder(context.Context, store.ID, []string) ([]string, bool, error) {
	return nil, false, nil
}

func (s *stubCompanies) Numbering(context.Context, store.ID, store.ID) (map[string]json.RawMessage, error) {
	return map[string]json.RawMessage{}, nil
}
func (s *stubCompanies) SetNumbering(context.Context, store.ID, store.ID, string, json.RawMessage) (bool, error) {
	return false, nil
}

func (s *stubUsers) SetTotpSecret(context.Context, store.ID, string) error { return nil }
func (s *stubUsers) SetTotpEnabled(context.Context, store.ID, bool) error  { return nil }
func (s *stubUsers) ClearTotp(context.Context, store.ID) error             { return nil }

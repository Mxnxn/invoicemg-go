package invoice

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

func callHistory(t *testing.T, s *stubInvoices, invoiceID string) (int, map[string]any) {
	t.Helper()
	h := New(s, &stubCompanies{}, &stubUsers{}, stubClientsX{}, stubJobsX{}, "")
	vals := url.Values{}
	if invoiceID != "" {
		vals.Set("invoice_id", invoiceID)
	}
	r := httptest.NewRequest("POST", "/", strings.NewReader(vals.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	h.History(rec, r)
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	return rec.Code, body
}

func TestHistory_RequiresInvoiceID(t *testing.T) {
	_, body := callHistory(t, &stubInvoices{}, "")
	if body["code"] != float64(422) {
		t.Errorf("code = %v, want 422", body["code"])
	}
}

func TestHistory_NotFound(t *testing.T) {
	_, body := callHistory(t, &stubInvoices{historyFound: false}, "inv1")
	if body["code"] != float64(404) || body["message"] != "No such invoice." {
		t.Errorf("got code=%v msg=%v, want 404", body["code"], body["message"])
	}
}

func TestHistory_AssemblesEntriesJobsTrail(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	s := &stubInvoices{historyFound: true, history: store.InvoiceHistory{
		InvoiceNumber: "MG/26-27/INV-00007",
		Entries: []store.InvoiceHistoryEntry{
			// amount 100 with 18% tax (cgst+sgst) -> total 118.
			{EntryID: "e1", Date: "2026-09-01", Material: "Vinyl", Description: "Banner", Qty: 2, Rate: 50,
				Amount: 100, Cgst: 9, Sgst: 9, CreatedAt: now, UpdatedAt: now},
		},
		Jobs: []store.InvoiceHistoryJob{{JobID: "j1", ChallanNumber: "CH-1"}},
		Trail: []store.InvoiceHistoryTrail{
			{At: now, Action: "Queue advanced", Detail: "Printing → Done", ActorName: "Alice", JobID: "j1"},
			{At: now, Action: "Created", Detail: "", ActorName: "Bob", JobID: "jX"}, // unknown job -> blank challan
		},
	}}
	code, body := callHistory(t, s, "inv1")
	if code != 200 || body["code"] != float64(200) {
		t.Fatalf("code = %v", body["code"])
	}
	if _, hasStatus := body["status"]; hasStatus {
		t.Error("history sends no status field")
	}
	data := body["data"].(map[string]any)
	if data["invoiceId"] != "MG/26-27/INV-00007" {
		t.Errorf("invoiceId = %v", data["invoiceId"])
	}
	entries := data["entries"].([]any)
	e0 := entries[0].(map[string]any)
	if e0["entry_id"] != "e1" || e0["total"] != float64(118) {
		t.Errorf("entry total = %v, want 118 (100 * 1.18)", e0["total"])
	}
	jobs := data["jobs"].([]any)
	if len(jobs) != 1 || jobs[0].(map[string]any)["challanNumber"] != "CH-1" {
		t.Errorf("jobs = %v", jobs)
	}
	trail := data["trail"].([]any)
	if len(trail) != 2 {
		t.Fatalf("trail len = %d, want 2", len(trail))
	}
	if trail[0].(map[string]any)["challanNumber"] != "CH-1" {
		t.Errorf("trail[0] challan = %v, want CH-1 (resolved by job_id)", trail[0])
	}
	if trail[1].(map[string]any)["challanNumber"] != "" {
		t.Errorf("trail[1] challan = %v, want blank (unknown job)", trail[1])
	}
}

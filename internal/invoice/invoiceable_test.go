package invoice

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

func callInvoiceable(t *testing.T, jobs []store.InvoiceableJob, clientID string) (int, map[string]any) {
	t.Helper()
	h := New(&stubInvoices{}, &stubCompanies{}, &stubUsers{}, stubClientsX{}, stubJobsX{invoiceable: jobs}, "")
	vals := url.Values{}
	if clientID != "" {
		vals.Set("client_id", clientID)
	}
	r := httptest.NewRequest("POST", "/", strings.NewReader(vals.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	h.InvoiceableJobs(rec, r)
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	return rec.Code, body
}

func TestInvoiceableJobs_RequiresClient(t *testing.T) {
	_, body := callInvoiceable(t, nil, "")
	if body["code"] != float64(422) {
		t.Errorf("code = %v, want 422", body["code"])
	}
}

func TestInvoiceableJobs_FiltersAndMath(t *testing.T) {
	jobs := []store.InvoiceableJob{
		// A: one unissued entry (100) + one already invoiced -> billable.
		{ID: "A", ChallanNumber: "CA", Total: 200, Rows: []store.InvoiceableRow{
			{EntryID: "e1", HasEntry: true, Amount: 100},
			{EntryID: "e2", HasEntry: true, HasIssued: true, Amount: 50},
		}},
		// B: an unconverted row still in Printing -> pending, excluded.
		{ID: "B", ChallanNumber: "CB", Rows: []store.InvoiceableRow{
			{Queue: "Printing"},
		}},
		// C: a Done unconverted row (convertible) and nothing else -> billable via convert.
		{ID: "C", ChallanNumber: "CC", Rows: []store.InvoiceableRow{
			{Queue: "Done"},
		}},
		// D: everything already invoiced -> nothing to bill, excluded.
		{ID: "D", ChallanNumber: "CD", Rows: []store.InvoiceableRow{
			{EntryID: "e9", HasEntry: true, HasIssued: true, Amount: 30},
		}},
		// E: one unissued entry carrying IGST -> billable, hasIgst true.
		{ID: "E", ChallanNumber: "CE", Rows: []store.InvoiceableRow{
			{EntryID: "e3", HasEntry: true, Amount: 20, Igst: 5},
		}},
	}
	code, body := callInvoiceable(t, jobs, "cl1")
	if code != 200 || body["code"] != float64(200) {
		t.Fatalf("code = %v", body["code"])
	}
	rows := body["data"].([]any)
	got := map[string]map[string]any{}
	for _, r := range rows {
		m := r.(map[string]any)
		got[m["_id"].(string)] = m
	}
	if len(rows) != 3 {
		t.Fatalf("included %d jobs (%v), want 3 (A, C, E)", len(rows), keysOf(got))
	}
	if _, ok := got["B"]; ok {
		t.Error("B has a pending row and must be excluded")
	}
	if _, ok := got["D"]; ok {
		t.Error("D has nothing to bill and must be excluded")
	}
	a := got["A"]
	if a["amount"] != float64(100) {
		t.Errorf("A amount = %v, want 100 (only the unissued entry)", a["amount"])
	}
	if ids := a["entryIds"].([]any); len(ids) != 1 || ids[0] != "e1" {
		t.Errorf("A entryIds = %v, want [e1]", a["entryIds"])
	}
	if a["invoicedRows"] != float64(1) || a["hasIgst"] != false {
		t.Errorf("A invoicedRows/hasIgst wrong: %v", a)
	}
	c := got["C"]
	if c["convertibleRows"] != float64(1) || len(c["entryIds"].([]any)) != 0 {
		t.Errorf("C should be convertible-only: %v", c)
	}
	if got["E"]["hasIgst"] != true {
		t.Errorf("E hasIgst = %v, want true", got["E"]["hasIgst"])
	}
}

func keysOf(m map[string]map[string]any) []string {
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	return out
}

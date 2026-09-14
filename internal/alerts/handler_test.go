package alerts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

const validID = "0123456789abcdef01234567" // 24 hex, passes the id guard

// stubAlerts stands in for the store so the handler's logic is testable without a database.
type stubAlerts struct {
	job        store.AlertJob
	jobErr     error
	client     store.AlertClient
	clientErr  error
	company    store.AlertCompany
	companyErr error
	jobCalls   int
}

func (s *stubAlerts) Job(_ context.Context, _ store.ID) (store.AlertJob, error) {
	s.jobCalls++
	return s.job, s.jobErr
}
func (s *stubAlerts) Client(_ context.Context, _ store.ID) (store.AlertClient, error) {
	return s.client, s.clientErr
}
func (s *stubAlerts) Company(_ context.Context, _ store.ID) (store.AlertCompany, error) {
	return s.company, s.companyErr
}

func serve(t *testing.T, s store.Alerts, jobID, jobcardID string) map[string]any {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /alert/{job_id}/job/{jobcard_id}", New(s).Detail)
	req := httptest.NewRequest("GET", "/alert/"+jobID+"/job/"+jobcardID, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response not json: %v (%s)", err, rec.Body.String())
	}
	return body
}

func code(body map[string]any) float64 { c, _ := body["code"].(float64); return c }

// #30/#32: a malformed job id is a dead link (404), and the store is never consulted.
func TestDetail_MalformedID(t *testing.T) {
	s := &stubAlerts{}
	body := serve(t, s, "nonsense", "anything")
	if code(body) != 404 {
		t.Fatalf("code = %v, want 404", body["code"])
	}
	if body["message"] != "This link is no longer valid." {
		t.Errorf("message = %v", body["message"])
	}
	if s.jobCalls != 0 {
		t.Errorf("store was consulted %d times for a malformed id, want 0", s.jobCalls)
	}
}

func TestDetail_JobNotFound(t *testing.T) {
	body := serve(t, &stubAlerts{jobErr: store.ErrNotFound}, validID, "x")
	if code(body) != 404 {
		t.Fatalf("code = %v, want 404", body["code"])
	}
}

func TestDetail_RowNotFound(t *testing.T) {
	job := store.AlertJob{ID: store.ID(validID), Rows: []store.AlertRow{{ID: "card-1"}}}
	body := serve(t, &stubAlerts{job: job}, validID, "no-such-card")
	if code(body) != 404 {
		t.Fatalf("code = %v, want 404", body["code"])
	}
}

func successStore() *stubAlerts {
	row := store.AlertRow{
		ID: "card-1", RowID: "ROW-1", Description: "Banner", Material: "Flex",
		Qty: 2, Length: "3", Width: "4", Rate: 10, Queue: "Printing", Progress: "Assign",
	}
	return &stubAlerts{
		job: store.AlertJob{
			ID: store.ID(validID), ChallanNumber: "MG/26-27/00001",
			ReceivedDate: "2026-09-01", Queue: "Printing",
			ClientID: "c1", CompanyID: "co1", Rows: []store.AlertRow{row},
		},
		client:  store.AlertClient{Name: "John", Firm: "John Co"},
		company: store.AlertCompany{Name: "Acme", Firm: "Acme Prints", Phone: "999", URL: "logo.png", Address: "addr", Gst: "GST1"},
	}
}

func TestDetail_SuccessByID(t *testing.T) {
	body := serve(t, successStore(), validID, "card-1")
	if code(body) != 200 {
		t.Fatalf("code = %v, want 200", body["code"])
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("no data object: %v", body)
	}
	if data["job_id"] != validID {
		t.Errorf("job_id = %v", data["job_id"])
	}
	if data["challanNumber"] != "MG/26-27/00001" {
		t.Errorf("challanNumber = %v", data["challanNumber"])
	}
	if data["receivedDate"] != "2026-09-01" {
		t.Errorf("receivedDate = %v", data["receivedDate"])
	}
	// 2 * (3*4) * 10 = 240, from jobmath.
	if data["total"] != float64(240) {
		t.Errorf("total = %v, want 240", data["total"])
	}
	company := data["company"].(map[string]any)
	if company["name"] != "Acme Prints" { // firm, not name
		t.Errorf("company.name = %v, want firm fallback", company["name"])
	}
	if company["logo"] != "logo.png" || company["gst"] != "GST1" {
		t.Errorf("company letterhead wrong: %v", company)
	}
	client := data["client"].(map[string]any)
	if client["name"] != "John" || client["firm"] != "John Co" {
		t.Errorf("client wrong: %v", client)
	}
	jobcard := data["jobcard"].(map[string]any)
	if jobcard["jobcard_id"] != "card-1" || jobcard["total"] != float64(240) || jobcard["hasDimensions"] != true {
		t.Errorf("jobcard wrong: %v", jobcard)
	}
	if rows, _ := data["rows"].([]any); len(rows) != 1 {
		t.Errorf("rows length = %d, want 1", len(rows))
	}
}

// A link built from the human rowId resolves too (findRow's second pass).
func TestDetail_SuccessByRowID(t *testing.T) {
	body := serve(t, successStore(), validID, "ROW-1")
	if code(body) != 200 {
		t.Fatalf("code = %v, want 200", body["code"])
	}
	data := body["data"].(map[string]any)
	if data["jobcard"].(map[string]any)["jobcard_id"] != "card-1" {
		t.Errorf("resolved the wrong row: %v", data["jobcard"])
	}
}

// A missing client is blank, not an error - Node's optional chaining.
func TestDetail_ClientMissingIsBlank(t *testing.T) {
	s := successStore()
	s.client = store.AlertClient{}
	s.clientErr = store.ErrNotFound
	body := serve(t, s, validID, "card-1")
	if code(body) != 200 {
		t.Fatalf("code = %v, want 200", body["code"])
	}
	client := body["data"].(map[string]any)["client"].(map[string]any)
	if client["name"] != "" || client["firm"] != "" {
		t.Errorf("client should be blank, got %v", client)
	}
}

// An older job with a blank receivedDate falls back to createdAt's date (UTC), the same as
// Model/Job.js's toJSON.
func TestDetail_ReceivedDateFallback(t *testing.T) {
	s := successStore()
	s.job.ReceivedDate = ""
	created := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	s.job.CreatedAt = &created
	body := serve(t, s, validID, "card-1")
	if got := body["data"].(map[string]any)["receivedDate"]; got != "2026-09-14" {
		t.Errorf("receivedDate fallback = %v, want 2026-09-14", got)
	}
}

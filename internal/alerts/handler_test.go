package alerts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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
	review     store.AlertReview
	reviewErr  error
	created    *store.NewReview
	createErr  error
}

func (s *stubAlerts) Review(_ context.Context, _ store.ID) (store.AlertReview, error) {
	return s.review, s.reviewErr
}

func (s *stubAlerts) CreateReview(_ context.Context, r store.NewReview) error {
	s.created = &r
	return s.createErr
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

func serveReview(t *testing.T, s store.Alerts, jobID string) map[string]any {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /alert/{job_id}/job/{jobcard_id}/review", New(s).Review)
	req := httptest.NewRequest("GET", "/alert/"+jobID+"/job/anycard/review", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response not json: %v (%s)", err, rec.Body.String())
	}
	return body
}

// #22: a job with no review sends reviewed:false and review:null (the key present, value null).
func TestReview_NotReviewed(t *testing.T) {
	body := serveReview(t, &stubAlerts{reviewErr: store.ErrNotFound}, validID)
	if code(body) != 200 {
		t.Fatalf("code = %v, want 200", body["code"])
	}
	data := body["data"].(map[string]any)
	if data["reviewed"] != false {
		t.Errorf("reviewed = %v, want false", data["reviewed"])
	}
	v, ok := data["review"]
	if !ok {
		t.Error("review key must be present (as null), not omitted")
	}
	if v != nil {
		t.Errorf("review = %v, want null", v)
	}
}

// #5 + #21: a reviewed job sends the projected object - _id, scores, comment, createdAt in
// JavaScript's Date.toJSON shape (…:56.000Z), and NO __v.
func TestReview_Reviewed(t *testing.T) {
	s := &stubAlerts{review: store.AlertReview{
		ID:        "rev-1",
		Scores:    store.ReviewScores{Quality: 5, Speed: 4, Communication: 5, Satisfaction: 4, Overall: 5},
		Comment:   "Great work",
		CreatedAt: time.Date(2026, 9, 14, 12, 34, 56, 0, time.UTC),
	}}
	body := serveReview(t, s, validID)
	if code(body) != 200 {
		t.Fatalf("code = %v, want 200", body["code"])
	}
	data := body["data"].(map[string]any)
	if data["reviewed"] != true {
		t.Errorf("reviewed = %v, want true", data["reviewed"])
	}
	review := data["review"].(map[string]any)
	if review["_id"] != "rev-1" || review["comment"] != "Great work" {
		t.Errorf("review fields wrong: %v", review)
	}
	if _, hasV := review["__v"]; hasV {
		t.Error("review must not carry __v (projection dropped it)")
	}
	if review["createdAt"] != "2026-09-14T12:34:56.000Z" {
		t.Errorf("createdAt = %v, want JS Date.toJSON millis form", review["createdAt"])
	}
	scores := review["scores"].(map[string]any)
	if scores["quality"] != float64(5) || scores["overall"] != float64(5) {
		t.Errorf("scores wrong: %v", scores)
	}
}

func TestReview_MalformedID(t *testing.T) {
	body := serveReview(t, &stubAlerts{}, "nonsense")
	if code(body) != 404 {
		t.Fatalf("code = %v, want 404", body["code"])
	}
}

func servePost(t *testing.T, s store.Alerts, jobID, jobcardID string, fields map[string]string) map[string]any {
	t.Helper()
	vals := url.Values{}
	for k, v := range fields {
		vals.Set(k, v)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /alert/{job_id}/job/{jobcard_id}/review", New(s).CreateReview)
	req := httptest.NewRequest("POST", "/alert/"+jobID+"/job/"+jobcardID+"/review", strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response not json: %v (%s)", err, rec.Body.String())
	}
	return body
}

// A stub set up so the pre-check passes (no existing review) and the write succeeds.
func writableStore() *stubAlerts {
	s := successStore()
	s.reviewErr = store.ErrNotFound // not yet reviewed
	return s
}

var fullScores = map[string]string{"quality": "5", "speed": "4", "communication": "5", "satisfaction": "4", "overall": "5", "comment": "Nice"}

func TestCreateReview_MalformedID(t *testing.T) {
	if code(servePost(t, &stubAlerts{}, "nonsense", "card-1", fullScores)) != 404 {
		t.Fatal("want 404 for malformed id")
	}
}

func TestCreateReview_InvalidScores(t *testing.T) {
	bad := map[string]string{"quality": "5", "speed": "4", "communication": "5", "satisfaction": "4"} // no overall
	body := servePost(t, writableStore(), validID, "card-1", bad)
	if code(body) != 422 {
		t.Fatalf("code = %v, want 422", body["code"])
	}
	if body["message"] != "Please give a rating from 1 to 5 for overall." {
		t.Errorf("message = %v", body["message"])
	}
}

func TestCreateReview_AlreadyReviewed(t *testing.T) {
	s := successStore()
	s.reviewErr = nil // Review() returns a found review -> pre-check 409
	body := servePost(t, s, validID, "card-1", fullScores)
	if code(body) != 409 {
		t.Fatalf("code = %v, want 409", body["code"])
	}
	if s.created != nil {
		t.Error("must not attempt a write when already reviewed")
	}
}

func TestCreateReview_Success(t *testing.T) {
	s := writableStore()
	body := servePost(t, s, validID, "card-1", fullScores)
	if code(body) != 200 {
		t.Fatalf("code = %v, want 200", body["code"])
	}
	if body["message"] != "Thank you for your feedback." {
		t.Errorf("message = %v", body["message"])
	}
	data := body["data"].(map[string]any)
	if data["comment"] != "Nice" {
		t.Errorf("comment = %v", data["comment"])
	}
	scores := data["scores"].(map[string]any)
	if scores["quality"] != float64(5) || scores["speed"] != float64(4) {
		t.Errorf("scores wrong: %v", scores)
	}
	// identity comes from the job, never the body: client/company/jobcard are snapshotted.
	if s.created == nil {
		t.Fatal("write was not attempted")
	}
	if s.created.JobcardID != "card-1" || s.created.ClientID != "c1" || s.created.CompanyID != "co1" {
		t.Errorf("snapshot wrong: %+v", *s.created)
	}
	if s.created.ClientName != "John Co" { // firm falls back over name
		t.Errorf("clientName = %q, want firm fallback", s.created.ClientName)
	}
}

func TestCreateReview_DuplicateRace(t *testing.T) {
	s := writableStore()
	s.createErr = store.ErrDuplicate
	if code(servePost(t, s, validID, "card-1", fullScores)) != 409 {
		t.Fatal("want 409 on duplicate race")
	}
}

func TestCreateReview_NoCompany(t *testing.T) {
	s := writableStore()
	s.createErr = store.ErrNotFound // no company resolvable
	if code(servePost(t, s, validID, "card-1", fullScores)) != 404 {
		t.Fatal("want 404 when no company can be resolved")
	}
}

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

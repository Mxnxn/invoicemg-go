package lifecycle

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

type stubJobs struct {
	list         []store.Job
	gotClientID  store.ID
	challans     []string
	byEntry      store.Job
	byEntryFound bool
	created      store.Job
	createDup    bool
	gotCreate    store.JobCreateInput
}

func (s *stubJobs) List(_ context.Context, _, _, clientID store.ID) ([]store.Job, error) {
	s.gotClientID = clientID
	return s.list, nil
}
func (s *stubJobs) ChallanNumbers(_ context.Context, _, _ store.ID) ([]string, error) {
	return s.challans, nil
}
func (s *stubJobs) ByEntry(_ context.Context, _, _, _ store.ID) (store.Job, bool, error) {
	return s.byEntry, s.byEntryFound, nil
}
func (s *stubJobs) Create(_ context.Context, in store.JobCreateInput) (store.Job, bool, error) {
	s.gotCreate = in
	return s.created, s.createDup, nil
}

func serve(t *testing.T, s store.Jobs) map[string]any {
	t.Helper()
	r := httptest.NewRequest("POST", "/", nil)
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	New(s, &stubNotes{}).List(rec, r)
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("not json: %v (%s)", err, rec.Body.String())
	}
	return out
}

func boolPtr(b bool) *bool { return &b }

func TestJobsList_ShapeAndDerivedState(t *testing.T) {
	s := &stubJobs{list: []store.Job{{
		ID: "j1", ChallanNumber: "JOB/26-27/000001", ReceivedDate: "2026-09-11", Total: 5000,
		Queue: "Printing", Progress: "Unassigned",
		Client:   &store.JobClient{ID: "c1", ClientName: "Priya", ClientFirm: "Acme"},
		Employee: &store.JobPerson{ID: "p1", Name: "Anita"},
		Rows: []store.JobRow{
			{ID: "r1", Material: "Vinyl", Qty: 10, Rate: 45, HasDimensions: boolPtr(false), Queue: "Done",
				Entry: &store.JobRowEntry{ID: "e1", HasIssued: true, Total: 100}, EntryIssued: true, InvoiceNumber: "INV/1"},
			{ID: "r2", Material: "Flex", Qty: 1, Rate: 2500, HasDimensions: boolPtr(true), Queue: "Printing"},
		},
	}}}
	body := serve(t, s)

	if body["message"] != "Operation successful." {
		t.Errorf("message = %v", body["message"])
	}
	job := body["data"].([]any)[0].(map[string]any)

	// populated client / employee objects, not ids
	if c, ok := job["client_id"].(map[string]any); !ok || c["clientName"] != "Priya" {
		t.Errorf("client_id not populated: %v", job["client_id"])
	}
	if e, ok := job["employee_id"].(map[string]any); !ok || e["name"] != "Anita" {
		t.Errorf("employee_id not populated: %v", job["employee_id"])
	}
	if job["vendor_id"] != nil {
		t.Errorf("vendor_id should be null, got %v", job["vendor_id"])
	}
	// one of two rows invoiced -> partial
	if job["invoiceState"] != "partial" || job["invoicedRows"] != float64(1) {
		t.Errorf("invoiceState/invoicedRows wrong: %v / %v", job["invoiceState"], job["invoicedRows"])
	}
	if nums, _ := job["invoiceNumbers"].([]any); len(nums) != 1 || nums[0] != "INV/1" {
		t.Errorf("invoiceNumbers wrong: %v", job["invoiceNumbers"])
	}
	// partial -> not ready for invoice
	if job["readyForInvoice"] != false {
		t.Errorf("readyForInvoice should be false")
	}
	// lock: invoiced+locked forbids edits
	lock := job["lock"].(map[string]any)
	if lock["canEditValues"] != false || lock["invoiced"] != true {
		t.Errorf("lock wrong: %v", lock)
	}
	// alerts present with both channels
	alerts := job["alerts"].(map[string]any)
	if _, ok := alerts["created"].(map[string]any); !ok {
		t.Errorf("alerts.created missing: %v", alerts)
	}
	// per-row invoiced flag
	rows := job["rows"].([]any)
	if rows[0].(map[string]any)["invoiced"] != true || rows[1].(map[string]any)["invoiced"] != false {
		t.Errorf("row invoiced flags wrong")
	}
	// row entry_id populated as object
	if ent, ok := rows[0].(map[string]any)["entry_id"].(map[string]any); !ok || ent["has_issued"] != true {
		t.Errorf("row entry_id not populated: %v", rows[0])
	}
}

func TestJobsList_EmptyIsArray(t *testing.T) {
	body := serve(t, &stubJobs{list: nil})
	if rows, ok := body["data"].([]any); !ok || len(rows) != 0 {
		t.Errorf("data should be [], got %v", body["data"])
	}
}

func TestNextChallan(t *testing.T) {
	s := &stubJobs{challans: []string{"MG/26-27/00004", "MG/26-27/00002"}}
	h := New(s, &stubNotes{})
	now = func() time.Time { return time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC) }
	defer func() { now = func() time.Time { return time.Now().UTC() } }()
	r := httptest.NewRequest("POST", "/", nil)
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	h.NextChallan(rec, r)
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["data"].(map[string]any)["challanNumber"] != "MG/26-27/00005" {
		t.Errorf("next challan: %v", body["data"])
	}
}

func TestByEntry(t *testing.T) {
	// found -> populated job object
	s := &stubJobs{byEntryFound: true, byEntry: store.Job{ID: "j1", ChallanNumber: "MG/26-27/00001"}}
	r := httptest.NewRequest("POST", "/", strings.NewReader(neturl.Values{"entry_id": {"e1"}}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	New(s, &stubNotes{}).ByEntry(rec, r)
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if _, ok := body["data"].(map[string]any); !ok {
		t.Errorf("found should return a job object, got %T: %v", body["data"], body["data"])
	}

	// not found -> data:null
	r2 := httptest.NewRequest("POST", "/", strings.NewReader(neturl.Values{"entry_id": {"e9"}}.Encode()))
	r2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r2 = r2.WithContext(auth.WithSession(r2.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec2 := httptest.NewRecorder()
	New(&stubJobs{byEntryFound: false}, &stubNotes{}).ByEntry(rec2, r2)
	var body2 map[string]any
	json.Unmarshal(rec2.Body.Bytes(), &body2)
	if v, ok := body2["data"]; !ok || v != nil {
		t.Errorf("not found should be data:null, got ok=%v v=%v", ok, v)
	}

	// missing entry_id -> 422
	r3 := httptest.NewRequest("POST", "/", nil)
	r3 = r3.WithContext(auth.WithSession(r3.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec3 := httptest.NewRecorder()
	New(&stubJobs{}, &stubNotes{}).ByEntry(rec3, r3)
	var body3 map[string]any
	json.Unmarshal(rec3.Body.Bytes(), &body3)
	if body3["code"] != float64(422) {
		t.Errorf("missing entry_id: %v", body3)
	}
}

type stubNotes struct {
	notes     []store.JobNote
	created   store.JobNote
	updated   store.JobNote
	found     bool
	forbidden bool
	history   []store.JobHistoryRow
	gotText   string
}

func (s *stubNotes) NotesList(_ context.Context, _, _, _ store.ID) ([]store.JobNote, error) {
	return s.notes, nil
}
func (s *stubNotes) NoteCreate(_ context.Context, _, _, _ store.ID, _ store.NoteActor, text string) (store.JobNote, error) {
	s.gotText = text
	return s.created, nil
}
func (s *stubNotes) NoteUpdate(_ context.Context, _, _, _ store.ID, _ store.NoteActor, text string) (store.JobNote, bool, bool, error) {
	s.gotText = text
	return s.updated, s.found, s.forbidden, nil
}
func (s *stubNotes) HistoryList(_ context.Context, _, _, _ store.ID) ([]store.JobHistoryRow, error) {
	return s.history, nil
}

func postNotes(t *testing.T, n store.JobNotes, fn func(*Handler) func(http.ResponseWriter, *http.Request), fields map[string]string) map[string]any {
	t.Helper()
	vals := neturl.Values{}
	for k, v := range fields {
		vals.Set(k, v)
	}
	r := httptest.NewRequest("POST", "/", strings.NewReader(vals.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1", Role: "admin"}))
	rec := httptest.NewRecorder()
	fn(New(&stubJobs{}, n))(rec, r)
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("not json: %v (%s)", err, rec.Body.String())
	}
	return out
}

func TestNotesList_CanEdit(t *testing.T) {
	// a fresh note authored by the acting admin (u1) is editable; an old one by someone else isn't
	n := &stubNotes{notes: []store.JobNote{
		{ID: "n1", AuthorID: "u1", Text: "fresh", CreatedAt: time.Now().UTC()},
		{ID: "n2", AuthorID: "other", Text: "old", CreatedAt: time.Now().UTC().Add(-48 * time.Hour)},
	}}
	body := postNotes(t, n, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.NotesList }, map[string]string{"job_id": "j1"})
	rows := body["data"].([]any)
	if rows[0].(map[string]any)["canEdit"] != true {
		t.Errorf("own fresh note should be editable")
	}
	if rows[1].(map[string]any)["canEdit"] != false {
		t.Errorf("other/old note should not be editable")
	}
}

func TestNoteCreate(t *testing.T) {
	n := &stubNotes{created: store.JobNote{ID: "n1", Text: "hi", AuthorID: "u1"}}
	body := postNotes(t, n, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.NoteCreate }, map[string]string{"job_id": "j1", "text": "hi"})
	if body["code"] != float64(200) || body["message"] != "Note added." {
		t.Fatalf("create: %v", body)
	}
	if body["data"].(map[string]any)["canEdit"] != true {
		t.Errorf("freshly created note should be canEdit:true")
	}
	if n.gotText != "hi" {
		t.Errorf("text passed = %q", n.gotText)
	}
	// missing text
	body = postNotes(t, &stubNotes{}, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.NoteCreate }, map[string]string{"job_id": "j1"})
	if body["code"] != float64(422) {
		t.Errorf("missing text: %v", body)
	}
}

func TestNoteUpdate(t *testing.T) {
	n := &stubNotes{found: true, updated: store.JobNote{ID: "n1", Text: "edited", AuthorID: "u1", CreatedAt: time.Now().UTC()}}
	body := postNotes(t, n, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.NoteUpdate }, map[string]string{"note_id": "n1", "text": "edited"})
	if body["code"] != float64(200) || body["message"] != "Note updated." {
		t.Fatalf("update: %v", body)
	}
	// forbidden
	body = postNotes(t, &stubNotes{found: true, forbidden: true}, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.NoteUpdate }, map[string]string{"note_id": "n1", "text": "x"})
	if body["code"] != float64(403) {
		t.Errorf("forbidden: %v", body)
	}
	// not found
	body = postNotes(t, &stubNotes{found: false}, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.NoteUpdate }, map[string]string{"note_id": "n9", "text": "x"})
	if body["code"] != float64(404) {
		t.Errorf("not found: %v", body)
	}
}

func TestHistoryList(t *testing.T) {
	n := &stubNotes{history: []store.JobHistoryRow{{ID: "h1", Action: "Created", Detail: "Queue: Created"}}}
	body := postNotes(t, n, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.HistoryList }, map[string]string{"job_id": "j1"})
	row := body["data"].([]any)[0].(map[string]any)
	if row["action"] != "Created" {
		t.Errorf("history row: %v", row)
	}
	body = postNotes(t, &stubNotes{}, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.HistoryList }, map[string]string{})
	if body["code"] != float64(422) {
		t.Errorf("missing job_id: %v", body)
	}
}

func TestJobCreate(t *testing.T) {
	s := &stubJobs{created: store.Job{ID: "j1", ChallanNumber: "MG/26-27/00001"}}
	body := postNotesJob(t, s, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Create }, map[string]string{
		"client_id": "c1", "challanNumber": "MG/26-27/00001",
		"rows": `[{"material":"Vinyl","qty":"10","length":"2","width":"3","rate":45,"cgst":9,"sgst":9}]`,
	})
	if body["code"] != float64(200) || body["message"] != "Job created." {
		t.Fatalf("create: %v", body)
	}
	// total = (10*2*3*45) * 1.18 = 2700 * 1.18 = 3186 ; one row; unassigned
	if len(s.gotCreate.Rows) != 1 || s.gotCreate.Total < 3185.9 || s.gotCreate.Total > 3186.1 {
		t.Errorf("create total/rows: %+v total=%v", s.gotCreate.Rows, s.gotCreate.Total)
	}
	if s.gotCreate.Progress != "Unassigned" {
		t.Errorf("no assignee -> Unassigned, got %q", s.gotCreate.Progress)
	}

	// with an assignee -> In Progress
	s2 := &stubJobs{created: store.Job{ID: "j2"}}
	postNotesJob(t, s2, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Create }, map[string]string{
		"client_id": "c1", "challanNumber": "X", "employee_id": "e1", "rows": `[{"material":"m","qty":1,"rate":1}]`,
	})
	if s2.gotCreate.Progress != "In Progress" {
		t.Errorf("assignee -> In Progress, got %q", s2.gotCreate.Progress)
	}

	// validation
	body = postNotesJob(t, &stubJobs{}, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Create }, map[string]string{"client_id": "c1"})
	if body["code"] != float64(422) {
		t.Errorf("missing fields: %v", body)
	}
	body = postNotesJob(t, &stubJobs{}, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Create }, map[string]string{"client_id": "c1", "challanNumber": "X", "rows": "[]"})
	if body["message"] != "A job needs at least one row." {
		t.Errorf("empty rows: %v", body)
	}
	// dup challan
	body = postNotesJob(t, &stubJobs{createDup: true}, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Create }, map[string]string{"client_id": "c1", "challanNumber": "X", "rows": `[{"material":"m","qty":1,"rate":1}]`})
	if body["message"] != "This job number is already in use." {
		t.Errorf("dup: %v", body)
	}
}

func postNotesJob(t *testing.T, s store.Jobs, fn func(*Handler) func(http.ResponseWriter, *http.Request), fields map[string]string) map[string]any {
	t.Helper()
	vals := neturl.Values{}
	for k, v := range fields {
		vals.Set(k, v)
	}
	r := httptest.NewRequest("POST", "/", strings.NewReader(vals.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1", Role: "admin"}))
	rec := httptest.NewRecorder()
	fn(New(s, &stubNotes{}))(rec, r)
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("not json: %v (%s)", err, rec.Body.String())
	}
	return out
}

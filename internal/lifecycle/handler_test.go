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
	updated      store.Job
	updateFound  bool
	updateDup    bool
	updateEmpty  bool
	gotUpdate    store.JobUpdateInput
	txJob        store.Job
	txStatus     store.JobTxStatus
	gotTx        string
	gotKind      string
	gotArg       string
	gotPerson    store.ID
	gotOrder     []string
	convEntries  []store.Entry
	convJobs     []store.Job
	convFound    bool
	gotConvert   []store.ID

	unlockChanged   bool
	gotUnlockWanted *bool

	completeMoved  int
	completeCalled bool

	deleteCalled bool
	gotDeleteID  store.ID
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
func (s *stubJobs) Update(_ context.Context, in store.JobUpdateInput) (store.Job, bool, bool, bool, error) {
	s.gotUpdate = in
	return s.updated, s.updateFound, s.updateDup, s.updateEmpty, nil
}

// tx captures the last transition call and returns the stub's tx result.
func (s *stubJobs) tx(kind string) (store.Job, store.JobTxStatus, error) {
	s.gotTx = kind
	return s.txJob, s.txStatus, nil
}
func (s *stubJobs) Assign(_ context.Context, _, _, _ store.ID, _ store.NoteActor, kind string, personID store.ID) (store.Job, store.JobTxStatus, error) {
	s.gotKind, s.gotPerson = kind, personID
	return s.tx("assign")
}
func (s *stubJobs) Progress(_ context.Context, _, _, _ store.ID, _ store.NoteActor, progress string) (store.Job, store.JobTxStatus, error) {
	s.gotArg = progress
	return s.tx("progress")
}
func (s *stubJobs) SetQueue(_ context.Context, _, _, _ store.ID, _ store.NoteActor, queue string) (store.Job, store.JobTxStatus, error) {
	s.gotArg = queue
	return s.tx("set-queue")
}
func (s *stubJobs) QueueOrder(_ context.Context, _, _, _ store.ID, _ store.NoteActor, order []string) (store.Job, store.JobTxStatus, error) {
	s.gotOrder = order
	return s.tx("queue")
}
func (s *stubJobs) RowAssign(_ context.Context, _, _, _, _ store.ID, _ store.NoteActor, employeeID store.ID) (store.Job, store.JobTxStatus, error) {
	s.gotPerson = employeeID
	return s.tx("row-assign")
}
func (s *stubJobs) RowSetQueue(_ context.Context, _, _, _, _ store.ID, _ store.NoteActor, queue string) (store.Job, store.JobTxStatus, error) {
	s.gotArg = queue
	return s.tx("row-queue")
}
func (s *stubJobs) RowQueueOrder(_ context.Context, _, _, _, _ store.ID, _ store.NoteActor, order []string) (store.Job, store.JobTxStatus, error) {
	s.gotOrder = order
	return s.tx("row-queue-order")
}
func (s *stubJobs) RowProgress(_ context.Context, _, _, _, _ store.ID, _ store.NoteActor, progress string) (store.Job, store.JobTxStatus, error) {
	s.gotArg = progress
	return s.tx("row-progress")
}
func (s *stubJobs) ConvertToEntries(_ context.Context, _, _ store.ID, jobIDs []store.ID, _ store.NoteActor) ([]store.Entry, []store.Job, bool, error) {
	s.gotConvert = jobIDs
	return s.convEntries, s.convJobs, s.convFound, nil
}
func (s *stubJobs) Unlock(_ context.Context, _, _, _ store.ID, _ store.NoteActor, wanted bool) (store.Job, bool, store.JobTxStatus, error) {
	s.gotUnlockWanted = &wanted
	return s.txJob, s.unlockChanged, s.txStatus, nil
}
func (s *stubJobs) RowsCompleteAll(_ context.Context, _, _, _ store.ID, _ store.NoteActor) (store.Job, int, store.JobTxStatus, error) {
	s.completeCalled = true
	return s.txJob, s.completeMoved, s.txStatus, nil
}
func (s *stubJobs) Delete(_ context.Context, _, _, id store.ID, _ store.NoteActor) (store.JobTxStatus, error) {
	s.gotDeleteID, s.deleteCalled = id, true
	return s.txStatus, nil
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

func TestJobUpdate(t *testing.T) {
	s := &stubJobs{updateFound: true, updated: store.Job{ID: "j1"}}
	body := postNotesJob(t, s, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Update }, map[string]string{
		"job_id": "j1", "challanNumber": "MG/26-27/00002", "rows": `[{"_id":"r1","material":"V","qty":1,"rate":10}]`,
	})
	if body["code"] != float64(200) || body["message"] != "Job updated." {
		t.Fatalf("update: %v", body)
	}
	if s.gotUpdate.ChallanNumber == nil || *s.gotUpdate.ChallanNumber != "MG/26-27/00002" {
		t.Errorf("challan patch: %v", s.gotUpdate.ChallanNumber)
	}
	if !s.gotUpdate.RowsSet || len(s.gotUpdate.Rows) != 1 || s.gotUpdate.Rows[0].ID != "r1" {
		t.Errorf("rows patch: set=%v rows=%v", s.gotUpdate.RowsSet, s.gotUpdate.Rows)
	}
	if s.gotUpdate.ClientID != nil {
		t.Errorf("client_id not submitted -> nil")
	}

	// missing job_id
	body = postNotesJob(t, &stubJobs{}, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Update }, map[string]string{})
	if body["code"] != float64(422) {
		t.Errorf("missing id: %v", body)
	}
	// empty rows
	body = postNotesJob(t, &stubJobs{updateEmpty: true}, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Update }, map[string]string{"job_id": "j1", "rows": "[]"})
	if body["message"] != "A job needs at least one row." {
		t.Errorf("empty rows: %v", body)
	}
	// dup challan
	body = postNotesJob(t, &stubJobs{updateDup: true}, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Update }, map[string]string{"job_id": "j1", "challanNumber": "X"})
	if body["message"] != "This job number is already in use." {
		t.Errorf("dup: %v", body)
	}
	// not found
	body = postNotesJob(t, &stubJobs{updateFound: false}, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Update }, map[string]string{"job_id": "j9"})
	if body["code"] != float64(404) {
		t.Errorf("not found: %v", body)
	}
}

func TestTransitions(t *testing.T) {
	okJob := store.Job{ID: "j1", ChallanNumber: "MG/26-27/00001"}

	// assign employee → 200 with populated job, kind+person passed through
	s := &stubJobs{txJob: okJob, txStatus: store.JobTxOK}
	body := postNotesJob(t, s, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Assign },
		map[string]string{"job_id": "j1", "type": "employee", "person_id": "p1"})
	if body["code"] != float64(200) || body["message"] != "Job updated." {
		t.Fatalf("assign: %v", body)
	}
	if s.gotKind != "employee" || s.gotPerson != "p1" {
		t.Errorf("assign args: kind=%q person=%q", s.gotKind, s.gotPerson)
	}
	// bad type
	body = postNotesJob(t, &stubJobs{}, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Assign },
		map[string]string{"job_id": "j1", "type": "boss"})
	if body["message"] != "type must be 'employee' or 'vendor'." {
		t.Errorf("bad type: %v", body)
	}

	// progress needs-assignee → 422 with the job-level message
	s = &stubJobs{txStatus: store.JobTxNeedsAssignee}
	body = postNotesJob(t, s, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Progress },
		map[string]string{"job_id": "j1", "progress": "Printing"})
	if body["code"] != float64(422) || body["message"] != "Assign an employee or vendor before changing progress." {
		t.Errorf("progress needs assignee: %v", body)
	}

	// job not found on set-queue → 404
	s = &stubJobs{txStatus: store.JobTxJobNotFound}
	body = postNotesJob(t, s, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.SetQueue },
		map[string]string{"job_id": "j9", "queue": "Done"})
	if body["code"] != float64(404) || body["message"] != "Job not found." {
		t.Errorf("set-queue 404: %v", body)
	}

	// queue reorder passes the parsed array
	s = &stubJobs{txJob: okJob, txStatus: store.JobTxOK}
	postNotesJob(t, s, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Queue },
		map[string]string{"job_id": "j1", "queueOrder": `["Created","Printing","Done"]`})
	if len(s.gotOrder) != 3 || s.gotOrder[2] != "Done" {
		t.Errorf("queue order: %v", s.gotOrder)
	}

	// row not found → 404
	s = &stubJobs{txStatus: store.JobTxRowNotFound}
	body = postNotesJob(t, s, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.RowAssign },
		map[string]string{"job_id": "j1", "row_id": "r9", "employee_id": "p1"})
	if body["code"] != float64(404) || body["message"] != "Row not found." {
		t.Errorf("row 404: %v", body)
	}

	// row/progress invalid value → 422 before hitting the store
	body = postNotesJob(t, &stubJobs{}, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.RowProgress },
		map[string]string{"job_id": "j1", "row_id": "r1", "progress": "Nope"})
	if body["message"] != "Invalid progress value." {
		t.Errorf("row progress invalid: %v", body)
	}
	// row/progress needs-assignee message
	s = &stubJobs{txStatus: store.JobTxNeedsAssignee}
	body = postNotesJob(t, s, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.RowProgress },
		map[string]string{"job_id": "j1", "row_id": "r1", "progress": "Complete"})
	if body["message"] != "Assign an employee before changing progress." {
		t.Errorf("row progress needs assignee: %v", body)
	}
}

func TestConvertToEntries(t *testing.T) {
	s := &stubJobs{convFound: true,
		convEntries: []store.Entry{{ID: "e1", Material: "Vinyl", Amount: 100, Advance: 50, Total: 68}},
		convJobs:    []store.Job{{ID: "j1", ChallanNumber: "MG/26-27/00001"}}}
	body := postNotesJob(t, s, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.ConvertToEntries },
		map[string]string{"job_ids": `["j1"]`})
	if body["code"] != float64(200) || body["message"] != "Converted to entries." {
		t.Fatalf("convert: %v", body)
	}
	data := body["data"].(map[string]any)
	if len(data["entries"].([]any)) != 1 || len(data["jobs"].([]any)) != 1 {
		t.Errorf("data shape: %v", data)
	}
	if len(s.gotConvert) != 1 || s.gotConvert[0] != "j1" {
		t.Errorf("job ids passed: %v", s.gotConvert)
	}
	// no convertible → 404
	body = postNotesJob(t, &stubJobs{convFound: false}, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.ConvertToEntries },
		map[string]string{"job_ids": `["j9"]`})
	if body["code"] != float64(404) {
		t.Errorf("none convertible: %v", body)
	}
	// validation
	body = postNotesJob(t, &stubJobs{}, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.ConvertToEntries }, map[string]string{})
	if body["code"] != float64(422) {
		t.Errorf("missing job_ids: %v", body)
	}
	body = postNotesJob(t, &stubJobs{}, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.ConvertToEntries }, map[string]string{"job_ids": "[]"})
	if body["message"] != "job_ids must be a non-empty array." {
		t.Errorf("empty job_ids: %v", body)
	}
}

// ConvertRow parity (values from routes/Lifecycle convert-to-entries math).
func TestConvertRowMath(t *testing.T) {
	// qty 1, L 2, W 3, rate 100 -> base 600; net 600; gross (18% tax) 708; job fully paid -> advance 708, total 0
	ce := store.ConvertRow(store.ConvertJobRow{Qty: 1, Length: "2", Width: "3", Rate: 100, Cgst: 9, Sgst: 9}, 1000, 1000, "HSN1", "JOB/1")
	if ce.Amount != 600 || ce.Advance < 707.9 || ce.Advance > 708.1 || ce.Total != 0 {
		t.Errorf("fully paid: %+v", ce)
	}
	// unpaid job (advance 0) -> advance 0, total = gross 708
	ce = store.ConvertRow(store.ConvertJobRow{Qty: 1, Length: "2", Width: "3", Rate: 100, Cgst: 9, Sgst: 9}, 1000, 0, "", "JOB/1")
	if ce.Advance != 0 || ce.Total < 707.9 || ce.Total > 708.1 {
		t.Errorf("unpaid: %+v", ce)
	}
}

func TestUnlock(t *testing.T) {
	okJob := store.Job{ID: "j1", ChallanNumber: "MG/26-27/00001"}

	// changed -> "Job unlocked.", status:true, wanted=true, populated job
	s := &stubJobs{txJob: okJob, txStatus: store.JobTxOK, unlockChanged: true}
	body := postNotesJob(t, s, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Unlock },
		map[string]string{"job_id": "j1", "unlocked": "true"})
	if body["code"] != float64(200) || body["message"] != "Job unlocked." || body["status"] != true {
		t.Fatalf("unlock: %v", body)
	}
	if s.gotUnlockWanted == nil || *s.gotUnlockWanted != true {
		t.Errorf("wanted should be true, got %v", s.gotUnlockWanted)
	}
	if body["data"] == nil {
		t.Error("unlock must return the populated job")
	}

	// unlocked=false and changed -> "Job locked."
	s = &stubJobs{txJob: okJob, txStatus: store.JobTxOK, unlockChanged: true}
	body = postNotesJob(t, s, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Unlock },
		map[string]string{"job_id": "j1", "unlocked": "false"})
	if body["message"] != "Job locked." || s.gotUnlockWanted == nil || *s.gotUnlockWanted != false {
		t.Errorf("lock: %v (wanted=%v)", body, s.gotUnlockWanted)
	}

	// no change -> "No change." (still 200, still returns the job)
	s = &stubJobs{txJob: okJob, txStatus: store.JobTxOK, unlockChanged: false}
	body = postNotesJob(t, s, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Unlock },
		map[string]string{"job_id": "j1", "unlocked": "true"})
	if body["code"] != float64(200) || body["message"] != "No change." {
		t.Errorf("no-change: %v", body)
	}

	// unlocked absent -> defaults to wanting true
	s = &stubJobs{txJob: okJob, txStatus: store.JobTxOK, unlockChanged: true}
	postNotesJob(t, s, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Unlock },
		map[string]string{"job_id": "j1"})
	if s.gotUnlockWanted == nil || *s.gotUnlockWanted != true {
		t.Errorf("absent unlocked should default to true, got %v", s.gotUnlockWanted)
	}

	// missing job_id -> 422
	body = postNotesJob(t, &stubJobs{}, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Unlock },
		map[string]string{"unlocked": "true"})
	if body["code"] != float64(422) || body["message"] != "Invalid request." {
		t.Errorf("missing id: %v", body)
	}

	// not found -> 404
	s = &stubJobs{txStatus: store.JobTxJobNotFound}
	body = postNotesJob(t, s, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Unlock },
		map[string]string{"job_id": "j9", "unlocked": "true"})
	if body["code"] != float64(404) || body["message"] != "Job not found." {
		t.Errorf("not found: %v", body)
	}
}

func TestRowsCompleteAll(t *testing.T) {
	okJob := store.Job{ID: "j1", ChallanNumber: "MG/26-27/00001"}

	// several moved -> "N rows marked done." + populated job + status:true
	s := &stubJobs{txJob: okJob, txStatus: store.JobTxOK, completeMoved: 3}
	body := postNotesJob(t, s, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.RowsCompleteAll },
		map[string]string{"job_id": "j1"})
	if body["code"] != float64(200) || body["message"] != "3 rows marked done." || body["status"] != true {
		t.Fatalf("many: %v", body)
	}
	if !s.completeCalled || body["data"] == nil {
		t.Error("should call the store and return the populated job")
	}

	// exactly one -> singular
	s = &stubJobs{txJob: okJob, txStatus: store.JobTxOK, completeMoved: 1}
	body = postNotesJob(t, s, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.RowsCompleteAll },
		map[string]string{"job_id": "j1"})
	if body["message"] != "1 row marked done." {
		t.Errorf("singular: %v", body)
	}

	// none moved -> "Every row was already done."
	s = &stubJobs{txJob: okJob, txStatus: store.JobTxOK, completeMoved: 0}
	body = postNotesJob(t, s, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.RowsCompleteAll },
		map[string]string{"job_id": "j1"})
	if body["code"] != float64(200) || body["message"] != "Every row was already done." {
		t.Errorf("none: %v", body)
	}

	// invoice lock -> 403 with the queue refusal wording
	s = &stubJobs{txStatus: store.JobTxLocked}
	body = postNotesJob(t, s, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.RowsCompleteAll },
		map[string]string{"job_id": "j1"})
	if body["code"] != float64(403) || body["message"] != "This job is invoiced and locked. Unlock it to change production stages." {
		t.Errorf("locked: %v", body)
	}

	// missing id -> 422
	body = postNotesJob(t, &stubJobs{}, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.RowsCompleteAll },
		map[string]string{})
	if body["code"] != float64(422) || body["message"] != "Invalid request." {
		t.Errorf("missing id: %v", body)
	}

	// not found -> 404
	s = &stubJobs{txStatus: store.JobTxJobNotFound}
	body = postNotesJob(t, s, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.RowsCompleteAll },
		map[string]string{"job_id": "j9"})
	if body["code"] != float64(404) || body["message"] != "Job not found." {
		t.Errorf("not found: %v", body)
	}
}

func TestJobDelete(t *testing.T) {
	// success -> "Job moved to trash.", status:true, no data
	s := &stubJobs{txStatus: store.JobTxOK}
	body := postNotesJob(t, s, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Delete },
		map[string]string{"job_id": "j1"})
	if body["code"] != float64(200) || body["message"] != "Job moved to trash." || body["status"] != true {
		t.Fatalf("delete: %v", body)
	}
	if !s.deleteCalled || s.gotDeleteID != "j1" {
		t.Errorf("store not called with the job id: %v %v", s.deleteCalled, s.gotDeleteID)
	}
	if _, hasData := body["data"]; hasData {
		t.Error("delete sends no data")
	}

	// missing id -> 422
	body = postNotesJob(t, &stubJobs{}, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Delete },
		map[string]string{})
	if body["code"] != float64(422) || body["message"] != "Invalid request." {
		t.Errorf("missing id: %v", body)
	}

	// invoice lock -> 403 with the delete refusal wording
	s = &stubJobs{txStatus: store.JobTxLocked}
	body = postNotesJob(t, s, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Delete },
		map[string]string{"job_id": "j1"})
	if body["code"] != float64(403) || body["message"] != "This job is invoiced and cannot be deleted. Delete the invoice first." {
		t.Errorf("locked: %v", body)
	}

	// not found -> 404
	s = &stubJobs{txStatus: store.JobTxJobNotFound}
	body = postNotesJob(t, s, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Delete },
		map[string]string{"job_id": "j9"})
	if body["code"] != float64(404) || body["message"] != "Job not found." {
		t.Errorf("not found: %v", body)
	}
}

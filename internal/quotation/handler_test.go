package quotation

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

type stubQuotations struct {
	addResult   store.QuotationRowToJobResult
	list        []store.Quotation
	gotClientID store.ID

	numbers     []string
	got         store.Quotation
	found       bool
	created     store.Quotation
	updated     store.Quotation
	dup         bool
	deleteFound bool
	rowDeleted  store.Quotation
	gotCreate   store.QuotationWrite
	gotUpdate   store.QuotationUpdate
	gotRowID    store.ID
}

func (s *stubQuotations) List(_ context.Context, _, _, clientID store.ID) ([]store.Quotation, error) {
	s.gotClientID = clientID
	return s.list, nil
}
func (s *stubQuotations) Numbers(_ context.Context, _, _ store.ID) ([]string, error) {
	return s.numbers, nil
}
func (s *stubQuotations) Get(_ context.Context, _, _, _ store.ID) (store.Quotation, bool, error) {
	return s.got, s.found, nil
}
func (s *stubQuotations) Create(_ context.Context, _, _ store.ID, in store.QuotationWrite) (store.Quotation, bool, error) {
	s.gotCreate = in
	return s.created, s.dup, nil
}
func (s *stubQuotations) Update(_ context.Context, _, _, _ store.ID, in store.QuotationUpdate) (store.Quotation, bool, bool, error) {
	s.gotUpdate = in
	return s.updated, s.dup, s.found, nil
}
func (s *stubQuotations) Delete(_ context.Context, _, _, _ store.ID) (bool, error) {
	return s.deleteFound, nil
}
func (s *stubQuotations) AddRowToJob(_ context.Context, _, _, _, _ store.ID) (store.QuotationRowToJobResult, error) {
	return s.addResult, nil
}
func (s *stubQuotations) RowDelete(_ context.Context, _, _, _, rowID store.ID) (store.Quotation, bool, error) {
	s.gotRowID = rowID
	return s.rowDeleted, s.found, nil
}

func serve(t *testing.T, s store.Quotations) map[string]any {
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

func TestList_PopulatedClientAndRows(t *testing.T) {
	now := time.Date(2026, 9, 10, 4, 0, 0, 0, time.UTC)
	s := &stubQuotations{list: []store.Quotation{{
		ID: "q1", UID: "u1", CompanyID: "co1", QuotationNumber: "QT/26-27/000001", Date: "2026-09-10",
		Client:    &store.QuotationClient{ID: "c1", ClientName: "Priya", ClientFirm: "Acme"},
		Rows:      []store.QuotationRow{{ID: "r1", Material: "Vinyl", Qty: 10, Rate: 45}},
		CreatedAt: now, UpdatedAt: now,
	}}}
	body := serve(t, s)

	q := body["data"].([]any)[0].(map[string]any)
	if q["quotationNumber"] != "QT/26-27/000001" {
		t.Errorf("number = %v", q["quotationNumber"])
	}
	// client_id is the POPULATED object, not a string.
	client, ok := q["client_id"].(map[string]any)
	if !ok {
		t.Fatalf("client_id should be a populated object, got %T: %v", q["client_id"], q["client_id"])
	}
	if client["_id"] != "c1" || client["clientName"] != "Priya" || client["clientFirm"] != "Acme" {
		t.Errorf("populated client wrong: %v", client)
	}
	rows := q["rows"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["material"] != "Vinyl" {
		t.Errorf("rows wrong: %v", rows)
	}
	if _, hasV := q["__v"]; !hasV {
		t.Error("__v must be present")
	}
}

// A dangling client (deleted) populates to null, not an object.
func TestList_NullClientWhenMissing(t *testing.T) {
	s := &stubQuotations{list: []store.Quotation{{ID: "q1", QuotationNumber: "QT/1", Client: nil, Rows: nil}}}
	body := serve(t, s)
	q := body["data"].([]any)[0].(map[string]any)
	if v, ok := q["client_id"]; !ok || v != nil {
		t.Errorf("client_id must be present and null, got ok=%v v=%v", ok, v)
	}
	// rows still an array
	if _, ok := q["rows"].([]any); !ok {
		t.Errorf("rows should be [], got %v", q["rows"])
	}
}

func TestList_ClientFilterPassedThrough(t *testing.T) {
	s := &stubQuotations{list: nil}
	r := httptest.NewRequest("POST", "/", nil)
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	// no client_id field -> empty filter
	rec := httptest.NewRecorder()
	New(s).List(rec, r)
	if s.gotClientID != "" {
		t.Errorf("client filter should be empty, got %q", s.gotClientID)
	}
}

func postQ(t *testing.T, s store.Quotations, fn func(*Handler) http.HandlerFunc, fields map[string]string) map[string]any {
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

func TestNextNumber(t *testing.T) {
	s := &stubQuotations{numbers: []string{"MG/26-27/QT-00002", "MG/26-27/QT-00001"}}
	h := New(s)
	now = func() time.Time { return time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC) }
	defer func() { now = func() time.Time { return time.Now().UTC() } }()
	r := httptest.NewRequest("POST", "/", nil)
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	h.NextNumber(rec, r)
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["data"].(map[string]any)["quotationNumber"] != "MG/26-27/QT-00003" {
		t.Errorf("next number: %v", body["data"])
	}
}

func TestCreate_ParsesRows(t *testing.T) {
	s := &stubQuotations{created: store.Quotation{ID: "q1", QuotationNumber: "MG/26-27/QT-00001"}}
	body := postQ(t, s, func(h *Handler) http.HandlerFunc { return h.Create }, map[string]string{
		"client_id": "c1", "date": "2026-09-15", "quotationNumber": "MG/26-27/QT-00001",
		"rows": `[{"material":"Vinyl","qty":"10","rate":45,"length":"","width":"2"}]`,
	})
	if body["code"] != float64(200) || body["message"] != "Quotation created." {
		t.Fatalf("envelope: %v", body)
	}
	if len(s.gotCreate.Rows) != 1 {
		t.Fatalf("rows: %+v", s.gotCreate.Rows)
	}
	row := s.gotCreate.Rows[0]
	// qty coerced from string, rate from number, blank length defaults to "1"
	if row.Qty != 10 || row.Rate != 45 || row.Length != "1" || row.Width != "2" {
		t.Errorf("row coercion: %+v", row)
	}
}

func TestCreate_ValidationAndDup(t *testing.T) {
	body := postQ(t, &stubQuotations{}, func(h *Handler) http.HandlerFunc { return h.Create }, map[string]string{"client_id": "c1"})
	if body["code"] != float64(422) || body["message"] != "Invalid request." {
		t.Errorf("missing fields: %v", body)
	}
	body = postQ(t, &stubQuotations{}, func(h *Handler) http.HandlerFunc { return h.Create }, map[string]string{
		"client_id": "c1", "date": "d", "quotationNumber": "n", "rows": "{not an array}",
	})
	if body["code"] != float64(422) || body["message"] != "rows must be a JSON array." {
		t.Errorf("bad rows: %v", body)
	}
	body = postQ(t, &stubQuotations{dup: true}, func(h *Handler) http.HandlerFunc { return h.Create }, map[string]string{
		"client_id": "c1", "date": "d", "quotationNumber": "n", "rows": "[]",
	})
	if body["code"] != float64(422) || body["message"] != "This quotation number is already in use." {
		t.Errorf("dup: %v", body)
	}
}

func TestUpdate_PartialAndNotFound(t *testing.T) {
	s := &stubQuotations{found: true, updated: store.Quotation{ID: "q1"}}
	body := postQ(t, s, func(h *Handler) http.HandlerFunc { return h.Update }, map[string]string{
		"quotation_id": "q1", "date": "2026-10-01", "rows": `[{"material":"X","_id":"r1"}]`,
	})
	if body["code"] != float64(200) || body["message"] != "Quotation updated." {
		t.Fatalf("envelope: %v", body)
	}
	if s.gotUpdate.Date == nil || *s.gotUpdate.Date != "2026-10-01" {
		t.Errorf("date patch: %v", s.gotUpdate.Date)
	}
	if s.gotUpdate.ClientID != nil {
		t.Errorf("client_id not submitted -> nil, got %v", s.gotUpdate.ClientID)
	}
	if s.gotUpdate.Rows == nil || len(*s.gotUpdate.Rows) != 1 || (*s.gotUpdate.Rows)[0].ID != "r1" {
		t.Errorf("rows patch (with _id preserved): %v", s.gotUpdate.Rows)
	}

	body = postQ(t, &stubQuotations{}, func(h *Handler) http.HandlerFunc { return h.Update }, map[string]string{})
	if body["code"] != float64(422) {
		t.Errorf("missing id: %v", body)
	}
	body = postQ(t, &stubQuotations{found: false}, func(h *Handler) http.HandlerFunc { return h.Update }, map[string]string{"quotation_id": "q9"})
	if body["code"] != float64(404) {
		t.Errorf("not found: %v", body)
	}
}

func TestDeleteAndRowDelete(t *testing.T) {
	body := postQ(t, &stubQuotations{deleteFound: true}, func(h *Handler) http.HandlerFunc { return h.Delete }, map[string]string{"quotation_id": "q1"})
	if body["code"] != float64(200) || body["status"] != true || body["message"] != "Quotation deleted." {
		t.Errorf("delete: %v", body)
	}
	body = postQ(t, &stubQuotations{deleteFound: false}, func(h *Handler) http.HandlerFunc { return h.Delete }, map[string]string{"quotation_id": "q1"})
	if body["code"] != float64(404) {
		t.Errorf("delete 404: %v", body)
	}
	s := &stubQuotations{found: true, rowDeleted: store.Quotation{ID: "q1"}}
	body = postQ(t, s, func(h *Handler) http.HandlerFunc { return h.RowDelete }, map[string]string{"quotation_id": "q1", "row_id": "r5"})
	if body["code"] != float64(200) || body["message"] != "Row removed." || s.gotRowID != "r5" {
		t.Errorf("row delete: %v (rowID=%q)", body, s.gotRowID)
	}
	body = postQ(t, &stubQuotations{}, func(h *Handler) http.HandlerFunc { return h.RowDelete }, map[string]string{"quotation_id": "q1"})
	if body["code"] != float64(422) {
		t.Errorf("row delete missing row_id: %v", body)
	}
}

func TestAddRowToJob(t *testing.T) {
	s := &stubQuotations{addResult: store.QuotationRowToJobResult{
		Status: store.QRJOk, IsNew: true,
		Quotation: store.Quotation{ID: "q1", QuotationNumber: "MG/Q/1"},
		Job:       store.Job{ID: "j1", ChallanNumber: "MG/26-27/00001", Total: 708, Queue: "Created", Progress: "Unassigned"},
	}}
	body := postQ(t, s, func(h *Handler) http.HandlerFunc { return h.AddRowToJob },
		map[string]string{"quotation_id": "q1", "row_id": "r1"})
	if body["code"] != float64(200) || body["message"] != "Job created from quotation row." {
		t.Fatalf("new job: %v", body)
	}
	data := body["data"].(map[string]any)
	if data["job"].(map[string]any)["challanNumber"] != "MG/26-27/00001" {
		t.Errorf("job shape: %v", data["job"])
	}
	// existing job -> different message
	s.addResult.IsNew = false
	body = postQ(t, s, func(h *Handler) http.HandlerFunc { return h.AddRowToJob }, map[string]string{"quotation_id": "q1", "row_id": "r1"})
	if body["message"] != "Row added to the quotation's existing job." {
		t.Errorf("existing job msg: %v", body)
	}
	// status mappings
	cases := []struct {
		st   store.QuotationRowToJobStatus
		code float64
		msg  string
	}{
		{store.QRJQuotationNotFound, 404, "Quotation not found."},
		{store.QRJRowNotFound, 404, "Row not found."},
		{store.QRJAlreadyAdded, 422, "This row has already been added to a job."},
		{store.QRJDupChallan, 422, "Job number collision - try again."},
	}
	for _, c := range cases {
		body = postQ(t, &stubQuotations{addResult: store.QuotationRowToJobResult{Status: c.st}},
			func(h *Handler) http.HandlerFunc { return h.AddRowToJob }, map[string]string{"quotation_id": "q1", "row_id": "r1"})
		if body["code"] != c.code || body["message"] != c.msg {
			t.Errorf("status %v -> %v", c.st, body)
		}
	}
	// missing ids -> 422
	body = postQ(t, s, func(h *Handler) http.HandlerFunc { return h.AddRowToJob }, map[string]string{"quotation_id": "q1"})
	if body["code"] != float64(422) || body["message"] != "Invalid request." {
		t.Errorf("missing row_id: %v", body)
	}
}

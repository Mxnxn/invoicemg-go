package entry

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

type stubEntries struct {
	added    store.Entry
	gotAdd   store.EntryWrite
	updated  store.Entry
	upFound  bool
	gotUp    store.EntryUpdate
	got      store.Entry
	getFound bool
	list     []store.Entry
}

func (s *stubEntries) Add(_ context.Context, _, _ store.ID, in store.EntryWrite) (store.Entry, error) {
	s.gotAdd = in
	return s.added, nil
}
func (s *stubEntries) Update(_ context.Context, _, _ store.ID, in store.EntryUpdate) (store.Entry, bool, error) {
	s.gotUp = in
	return s.updated, s.upFound, nil
}
func (s *stubEntries) Get(_ context.Context, _, _, _ store.ID) (store.Entry, bool, error) {
	return s.got, s.getFound, nil
}
func (s *stubEntries) List(_ context.Context, _, _ store.ID) ([]store.Entry, error) {
	return s.list, nil
}

// fullForm is a request body that satisfies the add/update validation guard.
func fullForm() map[string]string {
	return map[string]string{
		"client_id": "c1", "date": "2026-09-15", "material": "Vinyl", "description": "Banner",
		"item_length": "3", "item_width": "4", "qty": "2", "rate": "50", "amount": "1200",
		"cgst": "9", "sgst": "9", "total": "416", "advance": "1000",
	}
}

func do(t *testing.T, s store.Entries, fn func(*Handler) http.HandlerFunc, form map[string]string) map[string]any {
	t.Helper()
	vals := neturl.Values{}
	for k, v := range form {
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

func TestAdd(t *testing.T) {
	s := &stubEntries{added: store.Entry{
		ID: "e1", Material: "Vinyl", Amount: 1200, Advance: 1000, Total: 416, ClientID: "c1",
		Client: &store.EntryClient{ID: "c1", ClientName: "Priya", ClientFirm: "Acme"},
	}}
	body := do(t, s, func(h *Handler) http.HandlerFunc { return h.Add }, fullForm())
	if body["code"] != float64(200) || body["message"] != "Entry saved successfully." {
		t.Fatalf("add envelope: %v", body)
	}
	// input coerced: strings -> floats, item_length -> length
	if s.gotAdd.Qty != 2 || s.gotAdd.Length != "3" || s.gotAdd.Igst != 0 {
		t.Errorf("coercion wrong: %+v", s.gotAdd)
	}
	data := body["data"].(map[string]any)
	client, ok := data["client_id"].(map[string]any)
	if !ok || client["clientName"] != "Priya" {
		t.Errorf("client_id should be populated object: %v", data["client_id"])
	}
	if data["quotation_id"] != nil {
		t.Errorf("quotation_id should be null: %v", data["quotation_id"])
	}
}

func TestAddValidation(t *testing.T) {
	f := fullForm()
	delete(f, "amount")
	body := do(t, &stubEntries{}, func(h *Handler) http.HandlerFunc { return h.Add }, f)
	if body["code"] != float64(422) || body["message"] != "Invalid request." {
		t.Errorf("missing amount should 422: %v", body)
	}
}

func TestUpdate(t *testing.T) {
	s := &stubEntries{upFound: true, updated: store.Entry{ID: "e1", ClientID: "c1", Amount: 1200}}
	f := fullForm()
	f["entry_id"] = "e1"
	body := do(t, s, func(h *Handler) http.HandlerFunc { return h.Update }, f)
	// Node quirk: /update returns status:false even on success, with no message.
	if body["code"] != float64(200) || body["status"] != false {
		t.Errorf("update envelope: %v", body)
	}
	if s.gotUp.EntryID != "e1" {
		t.Errorf("entry id not passed: %v", s.gotUp.EntryID)
	}
	data := body["data"].(map[string]any)
	if data["client_id"] != "c1" {
		t.Errorf("update client_id should be raw id: %v", data["client_id"])
	}
}

func TestUpdateNotFound(t *testing.T) {
	f := fullForm()
	f["entry_id"] = "missing"
	body := do(t, &stubEntries{upFound: false}, func(h *Handler) http.HandlerFunc { return h.Update }, f)
	if body["code"] != float64(200) || body["data"] != nil || body["status"] != false {
		t.Errorf("not found should be data:null status:false: %v", body)
	}
}

func TestUpdateValidation(t *testing.T) {
	body := do(t, &stubEntries{}, func(h *Handler) http.HandlerFunc { return h.Update }, fullForm())
	if body["code"] != float64(422) {
		t.Errorf("missing entry_id should 422: %v", body)
	}
}

func TestGet(t *testing.T) {
	s := &stubEntries{getFound: true, got: store.Entry{ID: "e1", ClientID: "c1", Material: "Vinyl"}}
	body := do(t, s, func(h *Handler) http.HandlerFunc { return h.Get }, map[string]string{"entry_id": "e1"})
	if body["message"] != "Operation successful." || body["data"].(map[string]any)["_id"] != "e1" {
		t.Errorf("get: %v", body)
	}
	// missing id
	body = do(t, s, func(h *Handler) http.HandlerFunc { return h.Get }, map[string]string{})
	if body["code"] != float64(422) {
		t.Errorf("get missing id: %v", body)
	}
	// not found -> data null
	body = do(t, &stubEntries{getFound: false}, func(h *Handler) http.HandlerFunc { return h.Get }, map[string]string{"entry_id": "x"})
	if body["data"] != nil {
		t.Errorf("get not found data null: %v", body)
	}
}

func TestGetAll(t *testing.T) {
	s := &stubEntries{list: []store.Entry{{ID: "e1"}, {ID: "e2"}}}
	body := do(t, s, func(h *Handler) http.HandlerFunc { return h.GetAll }, map[string]string{})
	rows, ok := body["data"].([]any)
	if !ok || len(rows) != 2 {
		t.Errorf("getall data: %v", body["data"])
	}
	// empty -> [] not null
	body = do(t, &stubEntries{list: nil}, func(h *Handler) http.HandlerFunc { return h.GetAll }, map[string]string{})
	if rows, ok := body["data"].([]any); !ok || len(rows) != 0 {
		t.Errorf("getall empty should be []: %v", body["data"])
	}
}

package person

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type stubPeople struct {
	list    []store.Person
	gotUID  store.ID
	gotType string
}

func (s *stubPeople) List(_ context.Context, uid store.ID, personType string) ([]store.Person, error) {
	s.gotUID, s.gotType = uid, personType
	return s.list, nil
}

// stubPeople satisfies the write half with no-ops (the list tests use it).
func (s *stubPeople) Create(context.Context, store.ID, store.PersonWrite) (store.Person, bool, error) {
	return store.Person{}, false, nil
}
func (s *stubPeople) Update(context.Context, store.ID, store.ID, store.PersonPatch) (store.Person, bool, bool, error) {
	return store.Person{}, false, false, nil
}
func (s *stubPeople) Delete(context.Context, store.ID, store.ID) (bool, error) { return false, nil }

// writeStub drives the write-path tests.
type writeStub struct {
	created     store.Person
	updated     store.Person
	dupCreate   bool
	dupUpdate   bool
	updateFound bool
	deleteFound bool
	gotCreate   store.PersonWrite
	gotPatch    store.PersonPatch
	gotDeleteID store.ID
}

func (s *writeStub) List(context.Context, store.ID, string) ([]store.Person, error) { return nil, nil }
func (s *writeStub) Create(_ context.Context, _ store.ID, in store.PersonWrite) (store.Person, bool, error) {
	s.gotCreate = in
	return s.created, s.dupCreate, nil
}
func (s *writeStub) Update(_ context.Context, _, _ store.ID, patch store.PersonPatch) (store.Person, bool, bool, error) {
	s.gotPatch = patch
	return s.updated, s.dupUpdate, s.updateFound, nil
}
func (s *writeStub) Delete(_ context.Context, _, id store.ID) (bool, error) {
	s.gotDeleteID = id
	return s.deleteFound, nil
}

func serve(t *testing.T, s store.People, typeField string) map[string]any {
	t.Helper()
	body := ""
	if typeField != "" {
		body = url.Values{"type": {typeField}}.Encode()
	}
	r := httptest.NewRequest("POST", "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1"}))
	rec := httptest.NewRecorder()
	New(s).List(rec, r)
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("not json: %v (%s)", err, rec.Body.String())
	}
	return out
}

func boolPtr(b bool) *bool { return &b }

// An employee row: permissions present, notifyPo* absent (nil) -> the keys must be omitted.
func TestList_Employee_OmitsUnsetFlags(t *testing.T) {
	s := &stubPeople{list: []store.Person{{
		ID: "p1", Name: "Anita", Type: "Employee", Email: "a@x.test",
		Permissions: []string{"products", "customers"}, IsActive: true,
	}}}
	body := serve(t, s, "Employee")

	if s.gotUID != "u1" || s.gotType != "Employee" {
		t.Errorf("scope/filter wrong: uid=%q type=%q", s.gotUID, s.gotType)
	}
	row := body["data"].([]any)[0].(map[string]any)
	if row["name"] != "Anita" || row["type"] != "Employee" {
		t.Errorf("fields wrong: %v", row)
	}
	if perms, ok := row["permissions"].([]any); !ok || len(perms) != 2 {
		t.Errorf("permissions wrong: %v", row["permissions"])
	}
	for _, k := range []string{"notifyPoCreated", "notifyPoUpdated", "notifyPoConfirmed"} {
		if _, present := row[k]; present {
			t.Errorf("%s must be omitted when unset", k)
		}
	}
}

// A supplier with a set flag: the key is present with its value.
func TestList_Supplier_KeepsSetFlag(t *testing.T) {
	s := &stubPeople{list: []store.Person{{
		ID: "p2", Name: "Metro", Type: "Supplier", Permissions: []string{},
		NotifyPoCreated: boolPtr(true),
	}}}
	body := serve(t, s, "Supplier")
	row := body["data"].([]any)[0].(map[string]any)
	if row["notifyPoCreated"] != true {
		t.Errorf("set flag must be present and true, got %v", row["notifyPoCreated"])
	}
	// empty permissions is still an array, never null
	if perms, ok := row["permissions"].([]any); !ok || len(perms) != 0 {
		t.Errorf("permissions should be [], got %v", row["permissions"])
	}
}

// No type field -> no filter (empty string passed through).
func TestList_NoTypeFilter(t *testing.T) {
	s := &stubPeople{list: nil}
	serve(t, s, "")
	if s.gotType != "" {
		t.Errorf("type filter should be empty, got %q", s.gotType)
	}
}

func postW(t *testing.T, s store.People, fn func(*Handler) http.HandlerFunc, fields map[string]string) map[string]any {
	t.Helper()
	vals := url.Values{}
	for k, v := range fields {
		vals.Set(k, v)
	}
	r := httptest.NewRequest("POST", "/", strings.NewReader(vals.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1"}))
	rec := httptest.NewRecorder()
	fn(New(s))(rec, r)
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("not json: %v (%s)", err, rec.Body.String())
	}
	return out
}

func TestCreate_EmployeeWithCredentials(t *testing.T) {
	s := &writeStub{created: store.Person{ID: "p1", Name: "Anita", Type: "Employee", Email: "a@x.com", Permissions: []string{"invoices:view"}}}
	body := postW(t, s, func(h *Handler) http.HandlerFunc { return h.Create }, map[string]string{
		"name": "Anita", "type": "Employee", "email": "a@x.com", "password": "secret1",
		"permissions": `["invoices:view","invoices:approve"]`,
	})
	if body["code"] != float64(200) || body["message"] != "Person created." {
		t.Fatalf("envelope: %v", body)
	}
	if s.gotCreate.PasswordHash == nil {
		t.Errorf("employee with email+password should get a hash")
	}
	// unknown action dropped, known kept
	if len(s.gotCreate.Permissions) != 1 || s.gotCreate.Permissions[0] != "invoices:view" {
		t.Errorf("permissions normalised: %v", s.gotCreate.Permissions)
	}
	data := body["data"].(map[string]any)
	if data["name"] != "Anita" || data["type"] != "Employee" {
		t.Errorf("data: %v", data)
	}
	if _, ok := data["password"]; ok {
		t.Errorf("password hash must never be returned")
	}
}

func TestCreate_SupplierNoCredentials(t *testing.T) {
	s := &writeStub{created: store.Person{ID: "p2", Name: "Steel Co", Type: "Supplier"}}
	postW(t, s, func(h *Handler) http.HandlerFunc { return h.Create }, map[string]string{
		"name": "Steel Co", "type": "Supplier", "email": "s@x.com", "password": "secret1",
		"permissions": `["invoices:view"]`,
	})
	if s.gotCreate.PasswordHash != nil {
		t.Errorf("a Supplier must not get a password")
	}
	if s.gotCreate.Permissions != nil {
		t.Errorf("a Supplier must not get permissions, got %v", s.gotCreate.Permissions)
	}
	if s.gotCreate.Email == nil || *s.gotCreate.Email != "s@x.com" {
		t.Errorf("supplier email should still be stored: %v", s.gotCreate.Email)
	}
}

func TestCreate_ValidationAndDup(t *testing.T) {
	body := postW(t, &writeStub{}, func(h *Handler) http.HandlerFunc { return h.Create }, map[string]string{"name": "X"})
	if body["code"] != float64(422) || body["message"] != "Invalid request." {
		t.Errorf("missing type: %v", body)
	}
	body = postW(t, &writeStub{dupCreate: true}, func(h *Handler) http.HandlerFunc { return h.Create },
		map[string]string{"name": "X", "type": "Supplier"})
	if body["code"] != float64(422) || body["message"] != "That email is already registered to a person." {
		t.Errorf("dup email: %v", body)
	}
}

func TestUpdate_PartialAndClearEmail(t *testing.T) {
	s := &writeStub{updateFound: true, updated: store.Person{ID: "p1", Name: "New Name"}}
	body := postW(t, s, func(h *Handler) http.HandlerFunc { return h.Update }, map[string]string{
		"person_id": "p1", "name": "New Name", "email": "", "is_active": "false",
	})
	if body["code"] != float64(200) || body["message"] != "Person updated." {
		t.Fatalf("envelope: %v", body)
	}
	// name provided -> patched; email present-but-empty -> EmailSet with nil (clear to NULL)
	if s.gotPatch.Name == nil || *s.gotPatch.Name != "New Name" {
		t.Errorf("name patch: %v", s.gotPatch.Name)
	}
	if !s.gotPatch.EmailSet || s.gotPatch.Email != nil {
		t.Errorf("empty email should clear to NULL: set=%v val=%v", s.gotPatch.EmailSet, s.gotPatch.Email)
	}
	if s.gotPatch.IsActive == nil || *s.gotPatch.IsActive != false {
		t.Errorf("is_active=false should patch: %v", s.gotPatch.IsActive)
	}
	// gst was not submitted -> not patched
	if s.gotPatch.Gst != nil {
		t.Errorf("gst must be nil when not submitted: %v", s.gotPatch.Gst)
	}
}

func TestUpdate_MissingIdAndNotFound(t *testing.T) {
	body := postW(t, &writeStub{}, func(h *Handler) http.HandlerFunc { return h.Update }, map[string]string{"name": "x"})
	if body["code"] != float64(422) {
		t.Errorf("missing person_id: %v", body)
	}
	body = postW(t, &writeStub{updateFound: false}, func(h *Handler) http.HandlerFunc { return h.Update }, map[string]string{"person_id": "p9"})
	if body["code"] != float64(404) || body["message"] != "Person not found." {
		t.Errorf("not found: %v", body)
	}
}

func TestDelete(t *testing.T) {
	body := postW(t, &writeStub{deleteFound: true}, func(h *Handler) http.HandlerFunc { return h.Delete }, map[string]string{"person_id": "p1"})
	if body["code"] != float64(200) || body["status"] != true || body["message"] != "Person deleted." {
		t.Errorf("delete ok: %v", body)
	}
	body = postW(t, &writeStub{deleteFound: false}, func(h *Handler) http.HandlerFunc { return h.Delete }, map[string]string{"person_id": "p1"})
	if body["code"] != float64(404) {
		t.Errorf("delete 404: %v", body)
	}
	body = postW(t, &writeStub{}, func(h *Handler) http.HandlerFunc { return h.Delete }, map[string]string{})
	if body["code"] != float64(422) {
		t.Errorf("delete no id: %v", body)
	}
}

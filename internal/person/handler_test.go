package person

import (
	"context"
	"encoding/json"
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

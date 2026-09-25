package lookups

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type stubLookups struct {
	clients   []store.LookupClient
	materials []store.LookupMaterial
	people    []store.LookupPerson
}

func (s *stubLookups) Clients(_ context.Context, _, _ store.ID) ([]store.LookupClient, error) {
	return s.clients, nil
}
func (s *stubLookups) Materials(_ context.Context, _ store.ID) ([]store.LookupMaterial, error) {
	return s.materials, nil
}
func (s *stubLookups) People(_ context.Context, _ store.ID) ([]store.LookupPerson, error) {
	return s.people, nil
}

type stubUsers struct{ u store.User }

func (s *stubUsers) UpdateProfile(_ context.Context, _ store.ID, _, _ string) (store.User, bool, error) {
	return store.User{}, false, nil
}
func (s *stubUsers) FindByEmail(_ context.Context, _ string) (store.User, error) {
	return store.User{}, store.ErrNotFound
}
func (s *stubUsers) FindByID(_ context.Context, _ store.ID) (store.User, error) { return s.u, nil }
func (s *stubUsers) CreateSession(_ context.Context, _ store.NewSession) (store.Session, error) {
	return store.Session{}, nil
}

func call(t *testing.T, fn func(http.ResponseWriter, *http.Request)) map[string]any {
	t.Helper()
	r := httptest.NewRequest("POST", "/", nil)
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	fn(rec, r)
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("not json: %v (%s)", err, rec.Body.String())
	}
	return out
}

func TestMaterials(t *testing.T) {
	h := New(&stubLookups{materials: []store.LookupMaterial{{ID: "m1", MaterialName: "Vinyl", MaterialRate: 45, Hsn: "4911", Tax: 18}}}, &stubUsers{})
	row := call(t, h.Materials)["data"].([]any)[0].(map[string]any)
	if row["material_name"] != "Vinyl" || row["material_rate"] != float64(45) || row["tax"] != float64(18) {
		t.Errorf("material lookup wrong: %v", row)
	}
}

// People prepends the owner as an Admin, name falling back to email.
func TestPeople_PrependsOwnerAsAdmin(t *testing.T) {
	h := New(
		&stubLookups{people: []store.LookupPerson{{ID: "p1", Name: "Anita", Type: "Employee"}}},
		&stubUsers{u: store.User{ID: "u1", Name: "", Email: "owner@x.test"}},
	)
	data := call(t, h.People)["data"].([]any)
	if len(data) != 2 {
		t.Fatalf("want 2 (owner + 1), got %d", len(data))
	}
	owner := data[0].(map[string]any)
	if owner["type"] != "Admin" || owner["name"] != "owner@x.test" || owner["_id"] != "u1" {
		t.Errorf("owner row wrong: %v", owner)
	}
	if data[1].(map[string]any)["name"] != "Anita" {
		t.Errorf("person row wrong: %v", data[1])
	}
}

func (s *stubUsers) UpdatePassword(context.Context, store.ID, string) (bool, error) {
	return false, nil
}

func (s *stubUsers) SetTotpSecret(context.Context, store.ID, string) error { return nil }
func (s *stubUsers) SetTotpEnabled(context.Context, store.ID, bool) error  { return nil }
func (s *stubUsers) ClearTotp(context.Context, store.ID) error             { return nil }

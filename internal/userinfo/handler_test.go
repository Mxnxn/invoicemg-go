package userinfo

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type stubUsers struct{ user store.User }

func (s *stubUsers) FindByEmail(_ context.Context, _ string) (store.User, error) {
	return store.User{}, store.ErrNotFound
}
func (s *stubUsers) FindByID(_ context.Context, _ store.ID) (store.User, error) { return s.user, nil }
func (s *stubUsers) CreateSession(_ context.Context, _ store.NewSession) (store.Session, error) {
	return store.Session{}, nil
}

type stubCompanies struct{ company store.Company }

func (s *stubCompanies) List(_ context.Context, _ store.ID) ([]store.Company, error) { return nil, nil }
func (s *stubCompanies) Active(_ context.Context, _, _ store.ID) (store.Company, error) {
	return s.company, nil
}

func getBody(t *testing.T, u store.Users, c store.Companies, sess store.Session) map[string]any {
	t.Helper()
	r := httptest.NewRequest("POST", "/", nil)
	r = r.WithContext(auth.WithSession(r.Context(), sess))
	rec := httptest.NewRecorder()
	New(u, c).Get(rec, r)
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("not json: %v", err)
	}
	return body
}

func TestProfile_MergesUserAndCompany(t *testing.T) {
	u := &stubUsers{user: store.User{Name: "Owner", Email: "o@x.test", Role: "admin"}}
	c := &stubCompanies{company: store.Company{ID: "co1", Firm: "A LLP", Phone: "999", Gst: "GST1"}}
	body := getBody(t, u, c, store.Session{UID: "u1", CompanyID: "co1"})
	data := body["data"].(map[string]any)
	if data["name"] != "Owner" || data["firm"] != "A LLP" || data["gst"] != "GST1" {
		t.Errorf("merge wrong: %v", data)
	}
	if data["company_id"] != "co1" {
		t.Errorf("company_id = %v", data["company_id"])
	}
	if data["documentFont"] != "open-sans" || data["invoiceTemplate"] != "classic" {
		t.Errorf("template defaults wrong: %v", data)
	}
}

// No company (empty CompanyID) -> company_id is null and the company fields are blank; role
// falls back to "admin" when the user row carries none.
func TestProfile_NoCompanyNullId(t *testing.T) {
	body := getBody(t, &stubUsers{user: store.User{Name: "X"}}, &stubCompanies{}, store.Session{UID: "u1", CompanyID: ""})
	data := body["data"].(map[string]any)
	if v, ok := data["company_id"]; !ok || v != nil {
		t.Errorf("company_id must be present and null, got ok=%v v=%v", ok, v)
	}
	if data["role"] != "admin" {
		t.Errorf("role default = %v, want admin", data["role"])
	}
	if data["activeUntil"] != nil {
		t.Errorf("activeUntil = %v, want null", data["activeUntil"])
	}
}

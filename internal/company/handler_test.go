package company

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type stubCompanies struct {
	list   []store.Company
	active store.Company
	actErr error
}

func (s *stubCompanies) List(_ context.Context, _ store.ID) ([]store.Company, error) {
	return s.list, nil
}
func (s *stubCompanies) Active(_ context.Context, _, _ store.ID) (store.Company, error) {
	return s.active, s.actErr
}

// stubUsers implements store.Users; only FindByID matters here.
type stubUsers struct{ limit int }

func (s *stubUsers) FindByEmail(_ context.Context, _ string) (store.User, error) {
	return store.User{}, store.ErrNotFound
}
func (s *stubUsers) FindByID(_ context.Context, _ store.ID) (store.User, error) {
	return store.User{CompanyLimit: s.limit}, nil
}
func (s *stubUsers) CreateSession(_ context.Context, _ store.NewSession) (store.Session, error) {
	return store.Session{}, nil
}

func req(sess store.Session) *http.Request {
	r := httptest.NewRequest("POST", "/", nil)
	return r.WithContext(auth.WithSession(r.Context(), sess))
}

func TestList_LimitAndCanAdd(t *testing.T) {
	h := New(&stubCompanies{list: []store.Company{{ID: "co1", Name: "A", IsActive: true}}}, &stubUsers{limit: 3})
	rec := httptest.NewRecorder()
	h.List(rec, req(store.Session{UID: "u1", CompanyID: "co1", Role: "admin"}))

	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	data := body["data"].(map[string]any)
	if data["company_limit"] != float64(3) {
		t.Errorf("company_limit = %v, want 3", data["company_limit"])
	}
	if data["active_company_id"] != "co1" {
		t.Errorf("active_company_id = %v", data["active_company_id"])
	}
	if data["can_add_company"] != true { // 1 company < limit 3
		t.Errorf("can_add_company = %v, want true", data["can_add_company"])
	}
	if _, ok := body["status"]; ok {
		t.Error("company/list sends no status field")
	}
}

// limit is clamped to at least 1 (Math.max(1, ...)), so a user row with 0 still allows nothing
// beyond one and can_add is false once one exists.
func TestList_LimitClampedToOne(t *testing.T) {
	h := New(&stubCompanies{list: []store.Company{{ID: "co1"}}}, &stubUsers{limit: 0})
	rec := httptest.NewRecorder()
	h.List(rec, req(store.Session{UID: "u1", CompanyID: "co1", Role: "admin"}))
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	data := body["data"].(map[string]any)
	if data["company_limit"] != float64(1) {
		t.Errorf("limit = %v, want clamped to 1", data["company_limit"])
	}
	if data["can_add_company"] != false {
		t.Error("can_add_company should be false at the limit")
	}
}

func TestActive_NoCompany(t *testing.T) {
	h := New(&stubCompanies{}, &stubUsers{})
	rec := httptest.NewRecorder()
	h.Active(rec, req(store.Session{UID: "u1", CompanyID: ""}))
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["code"] != float64(404) {
		t.Errorf("code = %v, want 404 when no active company", body["code"])
	}
}

func TestActive_Success(t *testing.T) {
	h := New(&stubCompanies{active: store.Company{ID: "co1", Name: "A", Firm: "A LLP", IsActive: true}}, &stubUsers{})
	rec := httptest.NewRecorder()
	h.Active(rec, req(store.Session{UID: "u1", CompanyID: "co1"}))
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	data := body["data"].(map[string]any)
	if data["_id"] != "co1" || data["firm"] != "A LLP" {
		t.Errorf("company wrong: %v", data)
	}
	if et, ok := data["exportTemplate"].(map[string]any); !ok || et["logo"] != true {
		t.Errorf("exportTemplate defaults missing: %v", data["exportTemplate"])
	}
}

package company

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

type stubCompanies struct {
	list   []store.Company
	active store.Company
	actErr error

	created       store.Company
	updated       store.Company
	updateFound   bool
	findActive    store.Company
	found         bool
	deactivate    store.DeactivateResult
	gotCreate     store.CompanyWrite
	gotPatch      store.CompanyPatch
	gotDeactivate store.ID
}

func (s *stubCompanies) List(_ context.Context, _ store.ID) ([]store.Company, error) {
	return s.list, nil
}
func (s *stubCompanies) Active(_ context.Context, _, _ store.ID) (store.Company, error) {
	return s.active, s.actErr
}
func (s *stubCompanies) Count(_ context.Context, _ store.ID) (int, error) { return len(s.list), nil }
func (s *stubCompanies) Create(_ context.Context, _ store.ID, in store.CompanyWrite) (store.Company, error) {
	s.gotCreate = in
	return s.created, nil
}
func (s *stubCompanies) Update(_ context.Context, _, _ store.ID, patch store.CompanyPatch) (store.Company, bool, error) {
	s.gotPatch = patch
	return s.updated, s.updateFound, nil
}
func (s *stubCompanies) FindActive(_ context.Context, _, _ store.ID) (store.Company, bool, error) {
	return s.findActive, s.found, nil
}
func (s *stubCompanies) Deactivate(_ context.Context, _, companyID store.ID) (store.DeactivateResult, error) {
	s.gotDeactivate = companyID
	return s.deactivate, nil
}

// stubSessions implements store.Sessions; only BindCompany matters here.
type stubSessions struct {
	boundCompany store.ID
	boundTab     string
}

func (s *stubSessions) FindByToken(context.Context, string) (store.Session, error) {
	return store.Session{}, store.ErrNotFound
}
func (s *stubSessions) Deactivate(context.Context, store.ID) error { return nil }
func (s *stubSessions) ResolveCompany(context.Context, string, string, store.ID) (store.ID, error) {
	return "", nil
}
func (s *stubSessions) BindCompany(_ context.Context, _, tabID string, _, companyID store.ID) error {
	s.boundTab, s.boundCompany = tabID, companyID
	return nil
}

// stubUsers implements store.Users; only FindByID matters here.
type stubUsers struct{ limit int }

func (s *stubUsers) UpdateProfile(_ context.Context, _ store.ID, _, _ string) (store.User, bool, error) {
	return store.User{}, false, nil
}
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
	h := New(&stubCompanies{list: []store.Company{{ID: "co1", Name: "A", IsActive: true}}}, &stubUsers{limit: 3}, &stubSessions{})
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
	h := New(&stubCompanies{list: []store.Company{{ID: "co1"}}}, &stubUsers{limit: 0}, &stubSessions{})
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
	h := New(&stubCompanies{}, &stubUsers{}, &stubSessions{})
	rec := httptest.NewRecorder()
	h.Active(rec, req(store.Session{UID: "u1", CompanyID: ""}))
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["code"] != float64(404) {
		t.Errorf("code = %v, want 404 when no active company", body["code"])
	}
}

func TestActive_Success(t *testing.T) {
	h := New(&stubCompanies{active: store.Company{ID: "co1", Name: "A", Firm: "A LLP", IsActive: true}}, &stubUsers{}, &stubSessions{})
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

func postForm(t *testing.T, h *Handler, fn func(*Handler) http.HandlerFunc, sess store.Session, tabID string, fields map[string]string) map[string]any {
	t.Helper()
	vals := neturl.Values{}
	for k, v := range fields {
		vals.Set(k, v)
	}
	r := httptest.NewRequest("POST", "/", strings.NewReader(vals.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if tabID != "" {
		r.Header.Set("TAB-ID", tabID)
	}
	r = r.WithContext(auth.WithSession(r.Context(), sess))
	rec := httptest.NewRecorder()
	fn(h)(rec, r)
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("not json: %v (%s)", err, rec.Body.String())
	}
	return body
}

func TestCreate(t *testing.T) {
	s := &stubCompanies{created: store.Company{ID: "co2", Name: "New Co", Firm: "New Co", IsDefault: false, IsActive: true}}
	h := New(s, &stubUsers{}, &stubSessions{})
	body := postForm(t, h, func(h *Handler) http.HandlerFunc { return h.Create }, store.Session{UID: "u1", Role: "admin"}, "",
		map[string]string{"name": "New Co", "gst": "24AAA"})
	if body["code"] != float64(200) || body["message"] != "Company created." {
		t.Fatalf("envelope: %v", body)
	}
	if s.gotCreate.Name != "New Co" || s.gotCreate.Gst != "24AAA" {
		t.Errorf("create input: %+v", s.gotCreate)
	}
	if body["data"].(map[string]any)["_id"] != "co2" {
		t.Errorf("data: %v", body["data"])
	}

	// name required
	body = postForm(t, h, func(h *Handler) http.HandlerFunc { return h.Create }, store.Session{UID: "u1", Role: "admin"}, "", map[string]string{})
	if body["code"] != float64(422) || body["message"] != "Company name is required." {
		t.Errorf("missing name: %v", body)
	}
}

func TestUpdate(t *testing.T) {
	s := &stubCompanies{updateFound: true, updated: store.Company{ID: "co1", Name: "Renamed"}}
	h := New(s, &stubUsers{}, &stubSessions{})
	body := postForm(t, h, func(h *Handler) http.HandlerFunc { return h.Update }, store.Session{UID: "u1", Role: "admin"}, "",
		map[string]string{"company_id": "co1", "name": "Renamed", "gst": ""})
	if body["code"] != float64(200) || body["message"] != "Company updated." {
		t.Fatalf("envelope: %v", body)
	}
	// name submitted -> patched; gst present-but-empty -> patched to "" ; phone absent -> nil
	if s.gotPatch.Name == nil || *s.gotPatch.Name != "Renamed" {
		t.Errorf("name patch: %v", s.gotPatch.Name)
	}
	if s.gotPatch.Gst == nil || *s.gotPatch.Gst != "" {
		t.Errorf("gst present-empty should patch to empty: %v", s.gotPatch.Gst)
	}
	if s.gotPatch.Phone != nil {
		t.Errorf("phone absent should be nil: %v", s.gotPatch.Phone)
	}

	body = postForm(t, h, func(h *Handler) http.HandlerFunc { return h.Update }, store.Session{UID: "u1", Role: "admin"}, "", map[string]string{})
	if body["code"] != float64(422) {
		t.Errorf("missing company_id: %v", body)
	}
	body = postForm(t, New(&stubCompanies{updateFound: false}, &stubUsers{}, &stubSessions{}), func(h *Handler) http.HandlerFunc { return h.Update },
		store.Session{UID: "u1", Role: "admin"}, "", map[string]string{"company_id": "co9"})
	if body["code"] != float64(404) {
		t.Errorf("update 404: %v", body)
	}
}

func TestSwitch(t *testing.T) {
	sess := &stubSessions{}
	h := New(&stubCompanies{found: true, findActive: store.Company{ID: "co2", Name: "B"}}, &stubUsers{}, sess)
	body := postForm(t, h, func(h *Handler) http.HandlerFunc { return h.Switch },
		store.Session{UID: "u1", Token: "tok"}, "tab-7", map[string]string{"company_id": "co2"})
	if body["code"] != float64(200) || body["message"] != "Company switched." {
		t.Fatalf("envelope: %v", body)
	}
	if sess.boundTab != "tab-7" || sess.boundCompany != "co2" {
		t.Errorf("binding wrong: tab=%q company=%q", sess.boundTab, sess.boundCompany)
	}

	// missing TAB-ID
	body = postForm(t, h, func(h *Handler) http.HandlerFunc { return h.Switch }, store.Session{UID: "u1"}, "", map[string]string{"company_id": "co2"})
	if body["code"] != float64(422) || body["message"] != "TAB-ID header is required to switch company." {
		t.Errorf("missing tab: %v", body)
	}

	// company not owned/active
	body = postForm(t, New(&stubCompanies{found: false}, &stubUsers{}, &stubSessions{}), func(h *Handler) http.HandlerFunc { return h.Switch },
		store.Session{UID: "u1"}, "tab-1", map[string]string{"company_id": "cx"})
	if body["code"] != float64(404) {
		t.Errorf("switch 404: %v", body)
	}
}

func TestDeactivate(t *testing.T) {
	cases := []struct {
		res  store.DeactivateResult
		code float64
		msg  string
	}{
		{store.DeactivateOK, 200, "Company deactivated."},
		{store.DeactivateMustKeepOne, 422, "You must keep at least one active company."},
		{store.DeactivateIsDefault, 422, "Set another company as default before deactivating this one."},
		{store.DeactivateNotFound, 404, "Company not found."},
	}
	for _, c := range cases {
		h := New(&stubCompanies{deactivate: c.res}, &stubUsers{}, &stubSessions{})
		body := postForm(t, h, func(h *Handler) http.HandlerFunc { return h.Deactivate },
			store.Session{UID: "u1", Role: "admin"}, "", map[string]string{"company_id": "co1"})
		if body["code"] != c.code || body["message"] != c.msg {
			t.Errorf("result %d: got %v", c.res, body)
		}
	}
	h := New(&stubCompanies{}, &stubUsers{}, &stubSessions{})
	body := postForm(t, h, func(h *Handler) http.HandlerFunc { return h.Deactivate }, store.Session{UID: "u1", Role: "admin"}, "", map[string]string{})
	if body["code"] != float64(422) {
		t.Errorf("missing id: %v", body)
	}
}

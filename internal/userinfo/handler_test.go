package userinfo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type stubUsers struct{ user store.User }

func (s *stubUsers) UpdateProfile(_ context.Context, _ store.ID, email, name string) (store.User, bool, error) {
	s.user.Email, s.user.Name = email, name
	return s.user, true, nil
}
func (s *stubUsers) FindByEmail(_ context.Context, _ string) (store.User, error) {
	return store.User{}, store.ErrNotFound
}
func (s *stubUsers) FindByID(_ context.Context, _ store.ID) (store.User, error) { return s.user, nil }
func (s *stubUsers) CreateSession(_ context.Context, _ store.NewSession) (store.Session, error) {
	return store.Session{}, nil
}

type stubCompanies struct {
	company     store.Company
	updated     store.Company
	updateFound bool
	gotPatch    store.CompanyPatch
}

func (s *stubCompanies) List(_ context.Context, _ store.ID) ([]store.Company, error) { return nil, nil }
func (s *stubCompanies) Active(_ context.Context, _, _ store.ID) (store.Company, error) {
	return s.company, nil
}
func (s *stubCompanies) Count(_ context.Context, _ store.ID) (int, error) { return 0, nil }
func (s *stubCompanies) Scope(context.Context, store.ID, store.ID) ([]store.ID, bool, map[store.ID]string, error) {
	return nil, false, nil, nil
}
func (s *stubCompanies) SetReportsAcrossCompanies(context.Context, store.ID, store.ID, bool) (bool, error) {
	return false, nil
}
func (s *stubCompanies) Create(_ context.Context, _ store.ID, _ store.CompanyWrite) (store.Company, error) {
	return store.Company{}, nil
}
func (s *stubCompanies) Update(_ context.Context, _, _ store.ID, patch store.CompanyPatch) (store.Company, bool, error) {
	s.gotPatch = patch
	return s.updated, s.updateFound, nil
}
func (s *stubCompanies) FindActive(_ context.Context, _, _ store.ID) (store.Company, bool, error) {
	return store.Company{}, false, nil
}
func (s *stubCompanies) Deactivate(_ context.Context, _, _ store.ID) (store.DeactivateResult, error) {
	return store.DeactivateOK, nil
}

func getBody(t *testing.T, u store.Users, c store.Companies, sess store.Session) map[string]any {
	t.Helper()
	r := httptest.NewRequest("POST", "/", nil)
	r = r.WithContext(auth.WithSession(r.Context(), sess))
	rec := httptest.NewRecorder()
	New(u, c, "").Get(rec, r)
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
	if data["documentFont"] != "lato" || data["invoiceTemplate"] != "classic" {
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

func postForm(t *testing.T, u store.Users, c store.Companies, form string) map[string]any {
	t.Helper()
	r := httptest.NewRequest("POST", "/", strings.NewReader(form))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	New(u, c, "").Add(rec, r)
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	return body
}

func TestAdd(t *testing.T) {
	u := &stubUsers{user: store.User{Name: "Owner", Email: "o@x.test", Role: "admin"}}
	c := &stubCompanies{updateFound: true, updated: store.Company{ID: "co1", Firm: "New LLP", Phone: "111", Gst: "G2", AccountNo: "AC"}}
	body := postForm(t, u, c, "phone=111&firm=New+LLP&address=Road&gst=G2&account_no=AC")
	if body["code"] != float64(200) || body["message"] != "Operation successful." {
		t.Fatalf("add: %v", body)
	}
	if c.gotPatch.Firm == nil || *c.gotPatch.Firm != "New LLP" || c.gotPatch.AccountNo == nil {
		t.Errorf("patch not built: %+v", c.gotPatch)
	}
	data := body["data"].(map[string]any)
	if data["firm"] != "New LLP" || data["name"] != "Owner" {
		t.Errorf("profile: %v", data)
	}
	// missing gst -> 422 status true
	body = postForm(t, u, c, "phone=111&firm=X&address=Y")
	if body["code"] != float64(422) || body["message"] != "Invalid GST number." || body["status"] != true {
		t.Errorf("missing gst: %v", body)
	}
	// company not found -> 404
	body = postForm(t, u, &stubCompanies{updateFound: false}, "phone=1&firm=X&address=Y&gst=G")
	if body["code"] != float64(404) || body["message"] != "Company not found." {
		t.Errorf("not found: %v", body)
	}
}

func TestUpdate(t *testing.T) {
	u := &stubUsers{user: store.User{Name: "Old", Email: "old@x.test", Role: "admin"}}
	c := &stubCompanies{updateFound: true, updated: store.Company{ID: "co1", Firm: "F", Gst: "G"}}
	r := httptest.NewRequest("POST", "/", strings.NewReader("phone=1&firm=F&address=A&gst=G&email=new@x.test&name=New"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	New(u, c, "").Update(rec, r)
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	data, _ := body["data"].(map[string]any)
	if body["code"] != float64(200) || data["name"] != "New" || data["email"] != "new@x.test" {
		t.Fatalf("update: %v", body)
	}
	// missing name -> 422
	r2 := httptest.NewRequest("POST", "/", strings.NewReader("phone=1&firm=F&address=A&gst=G&email=e"))
	r2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r2 = r2.WithContext(auth.WithSession(r2.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec2 := httptest.NewRecorder()
	New(u, c, "").Update(rec2, r2)
	json.Unmarshal(rec2.Body.Bytes(), &body)
	if body["code"] != float64(422) {
		t.Errorf("missing name should 422: %v", body)
	}
}

func TestSetTemplate(t *testing.T) {
	u := &stubUsers{user: store.User{Name: "Owner", Email: "o@x.test"}}
	c := &stubCompanies{updateFound: true, updated: store.Company{ID: "co1", InvoiceTemplate: "modern"}}
	body := postForm2(t, u, c, "docType=invoice&template=modern", func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.SetTemplate })
	if body["code"] != float64(200) || body["message"] != "Template updated." {
		t.Fatalf("set-template: %v", body)
	}
	if c.gotPatch.InvoiceTemplate == nil || *c.gotPatch.InvoiceTemplate != "modern" {
		t.Errorf("patch: %+v", c.gotPatch)
	}
	if body["data"].(map[string]any)["invoiceTemplate"] != "modern" {
		t.Errorf("profile template: %v", body["data"])
	}
	// unknown docType -> 422
	body = postForm2(t, u, c, "docType=nope&template=x", func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.SetTemplate })
	if body["code"] != float64(422) {
		t.Errorf("unknown docType: %v", body)
	}
	// blank template -> 422
	body = postForm2(t, u, c, "docType=invoice", func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.SetTemplate })
	if body["code"] != float64(422) {
		t.Errorf("blank template: %v", body)
	}
}

func postForm2(t *testing.T, u store.Users, c store.Companies, form string, fn func(*Handler) func(http.ResponseWriter, *http.Request)) map[string]any {
	t.Helper()
	r := httptest.NewRequest("POST", "/", strings.NewReader(form))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	fn(New(u, c, ""))(rec, r)
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	return body
}

func (s *stubUsers) UpdatePassword(context.Context, store.ID, string) (bool, error) {
	return false, nil
}

func (s *stubCompanies) SetQueueOrder(context.Context, store.ID, []string) ([]string, bool, error) {
	return nil, false, nil
}

func (s *stubCompanies) Numbering(context.Context, store.ID, store.ID) (map[string]json.RawMessage, error) {
	return map[string]json.RawMessage{}, nil
}
func (s *stubCompanies) SetNumbering(context.Context, store.ID, store.ID, string, json.RawMessage) (bool, error) {
	return false, nil
}

func (s *stubUsers) SetTotpSecret(context.Context, store.ID, string) error { return nil }
func (s *stubUsers) SetTotpEnabled(context.Context, store.ID, bool) error  { return nil }
func (s *stubUsers) ClearTotp(context.Context, store.ID) error             { return nil }

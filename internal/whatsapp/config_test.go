package whatsapp

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

type stubCompanies struct {
	company     store.Company
	found       bool
	updated     store.Company
	updateFound bool
	gotPatch    store.CompanyPatch
}

func (s *stubCompanies) List(context.Context, store.ID) ([]store.Company, error) { return nil, nil }
func (s *stubCompanies) Active(context.Context, store.ID, store.ID) (store.Company, error) {
	return s.company, nil
}
func (s *stubCompanies) Count(context.Context, store.ID) (int, error) { return 0, nil }
func (s *stubCompanies) Create(context.Context, store.ID, store.CompanyWrite) (store.Company, error) {
	return store.Company{}, nil
}
func (s *stubCompanies) Update(_ context.Context, _, _ store.ID, patch store.CompanyPatch) (store.Company, bool, error) {
	s.gotPatch = patch
	return s.updated, s.updateFound, nil
}
func (s *stubCompanies) FindActive(context.Context, store.ID, store.ID) (store.Company, bool, error) {
	return s.company, s.found, nil
}
func (s *stubCompanies) Deactivate(context.Context, store.ID, store.ID) (store.DeactivateResult, error) {
	return store.DeactivateOK, nil
}

func post(t *testing.T, c store.Companies, fn func(*Handler) func(http.ResponseWriter, *http.Request), form string) map[string]any {
	t.Helper()
	r := httptest.NewRequest("POST", "/", strings.NewReader(form))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	fn(New("", c))(rec, r)
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("not json: %v (%s)", err, rec.Body.String())
	}
	return body
}

func TestConfig(t *testing.T) {
	c := &stubCompanies{found: true, company: store.Company{WaPhoneNumberID: "123", WaBusinessAccountID: "biz", WaAPIToken: "secret"}}
	body := post(t, c, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Config }, "")
	data := body["data"].(map[string]any)
	if data["phoneNumberId"] != "123" || data["hasApiToken"] != true || data["configured"] != true {
		t.Fatalf("config: %v", data)
	}
	// token never echoed
	if _, leaked := data["apiToken"]; leaked {
		t.Errorf("api token leaked")
	}
	// 404
	body = post(t, &stubCompanies{found: false}, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Config }, "")
	if body["code"] != float64(404) {
		t.Errorf("config 404: %v", body)
	}
}

func TestConfigUpdate(t *testing.T) {
	c := &stubCompanies{updateFound: true, updated: store.Company{WaPhoneNumberID: "999", WaAPIToken: "tok"}}
	body := post(t, c, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.ConfigUpdate }, "phoneNumberId=999&apiToken=tok")
	if body["message"] != "WhatsApp settings saved." {
		t.Fatalf("update: %v", body)
	}
	if c.gotPatch.WaPhoneNumberID == nil || *c.gotPatch.WaPhoneNumberID != "999" || c.gotPatch.WaAPIToken == nil {
		t.Errorf("patch: %+v", c.gotPatch)
	}
	// blank apiToken must NOT be patched (leave existing token)
	c2 := &stubCompanies{updateFound: true, updated: store.Company{}}
	post(t, c2, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.ConfigUpdate }, "phoneNumberId=1&apiToken=")
	if c2.gotPatch.WaAPIToken != nil {
		t.Errorf("blank token should not be patched: %+v", c2.gotPatch)
	}
}

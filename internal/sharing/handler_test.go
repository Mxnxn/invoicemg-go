package sharing

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
	store.Companies
	list []store.Company
}

func (s *stubCompanies) List(context.Context, store.ID) ([]store.Company, error) { return s.list, nil }

type stubMaterials struct {
	store.Materials
	before          []store.ID
	beforeFound     bool
	stored          []store.ID
	setFound        bool
	gotSetCompanies []store.ID
}

func (s *stubMaterials) OwnedSharing(context.Context, store.ID, store.ID) ([]store.ID, bool, error) {
	return s.before, s.beforeFound, nil
}
func (s *stubMaterials) SetSharing(_ context.Context, _, _ store.ID, comps []store.ID) ([]store.ID, bool, error) {
	s.gotSetCompanies = comps
	return s.stored, s.setFound, nil
}

// stubClients only needs to exist; the material path exercises the shared dispatch.
type stubClients struct{ store.Clients }

func post(sess store.Session, values neturl.Values) *http.Request {
	r := httptest.NewRequest("POST", "/", strings.NewReader(values.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r.WithContext(auth.WithSession(r.Context(), sess))
}

func body(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var b map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, rec.Body.String())
	}
	return b
}

func TestPreview_NamesLosingAndRequiresConfirmation(t *testing.T) {
	co := &stubCompanies{list: []store.Company{
		{ID: "c1", Name: "Alpha"}, {ID: "c2", Name: "Beta"}, {ID: "c3", Firm: "Gamma LLP"},
	}}
	m := &stubMaterials{before: []store.ID{"c1", "c2", "c3"}, beforeFound: true}
	h := New(co, m, &stubClients{})
	rec := httptest.NewRecorder()
	// Narrowing to just c1 removes c2 and c3.
	h.Preview(rec, post(store.Session{UID: "u1", CompanyID: "c1", Role: "admin"},
		neturl.Values{"kind": {"material"}, "record_id": {"m1"}, "companies": {"c1"}}))

	b := body(t, rec)
	if b["code"] != float64(200) {
		t.Fatalf("code = %v (%s)", b["code"], rec.Body.String())
	}
	data := b["data"].(map[string]any)
	if data["requiresConfirmation"] != true {
		t.Errorf("requiresConfirmation = %v, want true", data["requiresConfirmation"])
	}
	losing := data["losing"].([]any)
	if len(losing) != 2 {
		t.Fatalf("losing = %d, want 2", len(losing))
	}
	l0 := losing[0].(map[string]any)
	if l0["_id"] != "c2" || l0["name"] != "Beta" {
		t.Errorf("losing[0] = %v, want c2/Beta", l0)
	}
	l1 := losing[1].(map[string]any)
	if l1["name"] != "Gamma LLP" { // firm fallback
		t.Errorf("losing[1] name = %v, want firm fallback Gamma LLP", l1["name"])
	}
}

func TestPreview_AddingOnlyDoesNotWarn(t *testing.T) {
	co := &stubCompanies{list: []store.Company{{ID: "c1", Name: "Alpha"}, {ID: "c2", Name: "Beta"}}}
	m := &stubMaterials{before: []store.ID{"c1"}, beforeFound: true}
	h := New(co, m, &stubClients{})
	rec := httptest.NewRecorder()
	h.Preview(rec, post(store.Session{UID: "u1", CompanyID: "c1"},
		neturl.Values{"kind": {"material"}, "record_id": {"m1"}, "companies": {"c1,c2"}}))

	data := body(t, rec)["data"].(map[string]any)
	if data["requiresConfirmation"] != false {
		t.Errorf("requiresConfirmation = %v, want false when only adding", data["requiresConfirmation"])
	}
	if len(data["losing"].([]any)) != 0 {
		t.Errorf("losing = %v, want empty", data["losing"])
	}
}

func TestPreview_InvalidKindAndMissingRecord(t *testing.T) {
	h := New(&stubCompanies{}, &stubMaterials{}, &stubClients{})
	for _, v := range []neturl.Values{
		{"kind": {"widget"}, "record_id": {"m1"}}, // unknown kind
		{"kind": {"material"}},                     // no record_id
	} {
		rec := httptest.NewRecorder()
		h.Preview(rec, post(store.Session{UID: "u1", CompanyID: "c1"}, v))
		if got := body(t, rec)["code"]; got != float64(422) {
			t.Errorf("%v -> code %v, want 422", v, got)
		}
	}
}

func TestPreview_NotFound(t *testing.T) {
	m := &stubMaterials{beforeFound: false}
	h := New(&stubCompanies{}, m, &stubClients{})
	rec := httptest.NewRecorder()
	h.Preview(rec, post(store.Session{UID: "u1", CompanyID: "c1"},
		neturl.Values{"kind": {"material"}, "record_id": {"gone"}}))
	if got := body(t, rec)["code"]; got != float64(404) {
		t.Errorf("code = %v, want 404", got)
	}
}

func TestSet_FiltersToOwnedAndAlwaysKeepsActingCompany(t *testing.T) {
	co := &stubCompanies{list: []store.Company{{ID: "c1", Name: "Alpha"}, {ID: "c2", Name: "Beta"}}}
	m := &stubMaterials{setFound: true, stored: []store.ID{"c2", "c1"}}
	h := New(co, m, &stubClients{})
	rec := httptest.NewRecorder()
	// c9 is not owned (dropped); c2 is; the acting company c1 is force-kept.
	h.Set(rec, post(store.Session{UID: "u1", CompanyID: "c1", Role: "admin"},
		neturl.Values{"kind": {"material"}, "record_id": {"m1"}, "companies": {"c2,c9"}}))

	if len(m.gotSetCompanies) != 2 || m.gotSetCompanies[0] != "c2" || m.gotSetCompanies[1] != "c1" {
		t.Errorf("SetSharing got %v, want [c2, c1] (c9 dropped, acting c1 kept)", m.gotSetCompanies)
	}
	b := body(t, rec)
	if b["code"] != float64(200) || b["message"] != "Sharing updated." {
		t.Errorf("got code=%v msg=%v", b["code"], b["message"])
	}
	comps := b["data"].(map[string]any)["companies"].([]any)
	if len(comps) != 2 {
		t.Errorf("echoed companies = %v, want the stored pair", comps)
	}
}

func TestSet_NotFound(t *testing.T) {
	co := &stubCompanies{list: []store.Company{{ID: "c1", Name: "Alpha"}}}
	m := &stubMaterials{setFound: false}
	h := New(co, m, &stubClients{})
	rec := httptest.NewRecorder()
	h.Set(rec, post(store.Session{UID: "u1", CompanyID: "c1"},
		neturl.Values{"kind": {"material"}, "record_id": {"gone"}, "companies": {"c1"}}))
	if got := body(t, rec)["code"]; got != float64(404) {
		t.Errorf("code = %v, want 404", got)
	}
}

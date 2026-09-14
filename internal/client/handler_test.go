package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type stubClients struct {
	list       []store.Client
	err        error
	gotUID     store.ID
	gotCompany store.ID
}

func (s *stubClients) Visible(_ context.Context, uid, companyID store.ID) ([]store.Client, error) {
	s.gotUID, s.gotCompany = uid, companyID
	return s.list, s.err
}

func share(ids ...store.ID) *[]store.ID {
	out := append([]store.ID{}, ids...)
	return &out
}

func serve(t *testing.T, s store.Clients, fn func(*Handler) func(http.ResponseWriter, *http.Request)) map[string]any {
	t.Helper()
	req := httptest.NewRequest("POST", "/", nil)
	req = req.WithContext(auth.WithSession(req.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	fn(New(s))(rec, req)
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("not json: %v (%s)", err, rec.Body.String())
	}
	return body
}

// The three cases #1 turns on: a client the acting company owns, one shared IN from another of
// the owner's companies (borrowed), and a legacy row with a null company and no sharing field.
func sampleClients() []store.Client {
	return []store.Client{
		{ID: "own1", UID: "u1", CompanyID: "co1", ClientName: "Priya", ClientFirm: "Acme", OpeningBalance: 100, Sharing: share("co1")},
		{ID: "bor1", UID: "u1", CompanyID: "co2", ClientName: "Ravi", ClientFirm: "Bolt", OpeningBalance: 0, Sharing: share("co2", "co1")},
		{ID: "leg1", UID: "u1", CompanyID: "", ClientName: "Sam", ClientFirm: "Cobalt", OpeningBalance: 250, Sharing: nil},
	}
}

func TestGetall_SharingAndBorrowed(t *testing.T) {
	s := &stubClients{list: sampleClients()}
	body := serve(t, s, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Getall })

	if s.gotUID != "u1" || s.gotCompany != "co1" {
		t.Errorf("scope from session wrong: uid=%q company=%q", s.gotUID, s.gotCompany)
	}
	if body["message"] != "Operation successful" { // NO trailing period
		t.Errorf("message = %q", body["message"])
	}
	if _, hasStatus := body["status"]; hasStatus {
		t.Error("getall must not send a status field")
	}
	rows := body["data"].([]any)
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(rows))
	}

	own := rows[0].(map[string]any)
	if own["borrowed"] != false {
		t.Error("own client must not be borrowed")
	}
	if own["company_id"] != "co1" {
		t.Errorf("own company_id = %v", own["company_id"])
	}
	if sh, ok := own["sharing"].(map[string]any); !ok || len(sh["companies"].([]any)) != 1 {
		t.Errorf("own sharing wrong: %v", own["sharing"])
	}

	bor := rows[1].(map[string]any)
	if bor["borrowed"] != true {
		t.Error("client owned by co2 but shared with co1 must be borrowed")
	}

	leg := rows[2].(map[string]any)
	if leg["borrowed"] != false {
		t.Error("legacy null-company client must not be borrowed")
	}
	if v, ok := leg["company_id"]; !ok || v != nil {
		t.Errorf("legacy company_id must be present and null, got ok=%v v=%v", ok, v)
	}
	if _, hasSharing := leg["sharing"]; hasSharing {
		t.Error("legacy client with no sharing field must OMIT the sharing key")
	}
}

func TestOnly_Shape(t *testing.T) {
	body := serve(t, &stubClients{list: sampleClients()}, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Only })
	if body["message"] != "Operation successful" {
		t.Errorf("message = %q", body["message"])
	}
	rows := body["data"].([]any)
	own := rows[0].(map[string]any)
	if own["openingBalance"] != float64(100) {
		t.Errorf("openingBalance = %v, want 100", own["openingBalance"])
	}
	if _, hasSharing := own["sharing"]; hasSharing {
		t.Error("only must not include sharing")
	}
	if _, hasBorrowed := own["borrowed"]; hasBorrowed {
		t.Error("only must not include borrowed")
	}
}

func TestGetall_EmptyIsArray(t *testing.T) {
	body := serve(t, &stubClients{list: nil}, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Getall })
	if rows, ok := body["data"].([]any); !ok || len(rows) != 0 {
		t.Errorf("data should be [], got %v", body["data"])
	}
}

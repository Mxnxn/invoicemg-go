package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

// fakeSessions stands in for the database so the RULE can be tested rather than the driver.
type fakeSessions struct {
	session     store.Session
	err         error
	deactivated []store.ID
	companyID   store.ID
	companyErr  error
	lastTabID   string
}

func (f *fakeSessions) FindByToken(_ context.Context, token string) (store.Session, error) {
	if f.err != nil {
		return store.Session{}, f.err
	}
	if f.session.Token != token {
		return store.Session{}, store.ErrNotFound
	}
	return f.session, nil
}

func (f *fakeSessions) Deactivate(_ context.Context, id store.ID) error {
	f.deactivated = append(f.deactivated, id)
	return nil
}

func (f *fakeSessions) ResolveCompany(_ context.Context, _ string, tabID string, _ store.ID) (store.ID, error) {
	f.lastTabID = tabID
	return f.companyID, f.companyErr
}

func (f *fakeSessions) BindCompany(_ context.Context, _, _ string, _, _ store.ID) error { return nil }

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response was not json: %v (%s)", err, rec.Body.String())
	}
	return body
}

func run(t *testing.T, m *Middleware, headers map[string]string) (*httptest.ResponseRecorder, bool) {
	t.Helper()
	reached := false
	h := m.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/sheet/only", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec, reached
}

// The single most important property of this service: an auth failure is HTTP 200 with a code
// in the body. The React client branches on data.code and never on res.status, so a real 401
// reads to it as a SUCCESSFUL request that returned no data - a blank screen, no error.
func TestRefusalIsHTTP200WithCodeInBody(t *testing.T) {
	m := New(&fakeSessions{})
	rec, reached := run(t, m, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200 - the client reads data.code, not the status line", rec.Code)
	}
	if reached {
		t.Fatal("handler ran without a token")
	}
	body := decode(t, rec)
	if body["code"] != float64(401) {
		t.Fatalf("code = %v, want 401", body["code"])
	}
	if body["status"] != false {
		t.Fatalf("status = %v, want false", body["status"])
	}
}

func TestUnknownTokenIsRefused(t *testing.T) {
	m := New(&fakeSessions{session: store.Session{Token: "good", IsActive: true}})
	rec, reached := run(t, m, map[string]string{"SESSION-TOKEN": "wrong"})
	if reached {
		t.Fatal("handler ran for an unknown token")
	}
	if decode(t, rec)["code"] != float64(401) {
		t.Fatal("want 401 for an unknown token")
	}
}

func TestInactiveSessionIsRefused(t *testing.T) {
	m := New(&fakeSessions{session: store.Session{Token: "t", IsActive: false}})
	_, reached := run(t, m, map[string]string{"SESSION-TOKEN": "t"})
	if reached {
		t.Fatal("handler ran for an inactive session")
	}
}

// An expired session is RETIRED on the way past, not merely refused - otherwise the token can
// be replayed and the row sits in the collection for ever.
func TestExpiredSessionIsRetiredNotJustRefused(t *testing.T) {
	past := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	f := &fakeSessions{session: store.Session{
		Token: "t", IsActive: true, SessionID: store.ID("sess1"), ExpiresAt: &past,
	}}
	m := New(f)
	m.Now = func() time.Time { return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) }

	rec, reached := run(t, m, map[string]string{"SESSION-TOKEN": "t"})
	if reached {
		t.Fatal("handler ran for an expired session")
	}
	if len(f.deactivated) != 1 || f.deactivated[0] != store.ID("sess1") {
		t.Fatalf("session was refused but not retired: %v", f.deactivated)
	}
	if got := decode(t, rec)["message"]; got != "Your session has expired. Please sign in again." {
		t.Fatalf("message = %q, want the wording the Node helper sends", got)
	}
}

// A nil expiry means never. Model/UserSession.js stores null for a session that does not age
// out, and treating that as "expired at the zero time" would log everyone out at once.
func TestNilExpiryNeverExpires(t *testing.T) {
	f := &fakeSessions{session: store.Session{Token: "t", IsActive: true, ExpiresAt: nil}}
	m := New(f)
	_, reached := run(t, m, map[string]string{"SESSION-TOKEN": "t"})
	if !reached {
		t.Fatal("a session with no expiry was refused")
	}
	if len(f.deactivated) != 0 {
		t.Fatal("a session with no expiry was retired")
	}
}

// The tab header decides which company the request acts as. Dropping it would silently put
// every tab on the user's default company - the multi-company feature quietly not working.
func TestTabHeaderReachesCompanyResolution(t *testing.T) {
	f := &fakeSessions{
		session:   store.Session{Token: "t", IsActive: true, UID: store.ID("u1")},
		companyID: store.ID("c9"),
	}
	m := New(f)

	var got store.Session
	h := m.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = MustFrom(r.Context())
	}))
	req := httptest.NewRequest(http.MethodPost, "/sheet/only", nil)
	req.Header.Set("SESSION-TOKEN", "t")
	req.Header.Set("TAB-ID", "tab-42")
	h.ServeHTTP(httptest.NewRecorder(), req)

	if f.lastTabID != "tab-42" {
		t.Fatalf("tab id reaching the store = %q, want tab-42", f.lastTabID)
	}
	if got.CompanyID != store.ID("c9") {
		t.Fatalf("company on the session = %q, want c9", got.CompanyID)
	}
}

func TestRequireAdminRefusesOtherRoles(t *testing.T) {
	f := &fakeSessions{session: store.Session{Token: "t", IsActive: true, Role: "employee"}}
	m := New(f)

	reached := false
	h := Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true }),
		m.Require, RequireAdmin)

	req := httptest.NewRequest(http.MethodPost, "/sheet/only", nil)
	req.Header.Set("SESSION-TOKEN", "t")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if reached {
		t.Fatal("an employee reached an admin-only route")
	}
	body := decode(t, rec)
	if body["code"] != float64(403) {
		t.Fatalf("code = %v, want 403", body["code"])
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200", rec.Code)
	}
}

func TestRequireAdminAllowsAdminAndSuperadmin(t *testing.T) {
	for _, role := range []string{"admin", "superadmin"} {
		f := &fakeSessions{session: store.Session{Token: "t", IsActive: true, Role: role}}
		m := New(f)
		reached := false
		h := Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true }),
			m.Require, RequireAdmin)
		req := httptest.NewRequest(http.MethodPost, "/sheet/only", nil)
		req.Header.Set("SESSION-TOKEN", "t")
		h.ServeHTTP(httptest.NewRecorder(), req)
		if !reached {
			t.Fatalf("%s was refused", role)
		}
	}
}

func (f *fakeSessions) DeactivateOthers(context.Context, store.ID, string) error { return nil }

func (f *fakeSessions) LastLoginAt(context.Context, store.ID) (*time.Time, error) { return nil, nil }

package users

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type stubUsers struct{}

func (stubUsers) FindByEmail(context.Context, string) (store.User, error) {
	return store.User{}, store.ErrNotFound
}
func (stubUsers) FindByID(context.Context, store.ID) (store.User, error) { return store.User{}, nil }
func (stubUsers) UpdateProfile(context.Context, store.ID, string, string) (store.User, bool, error) {
	return store.User{}, false, nil
}
func (stubUsers) CreateSession(context.Context, store.NewSession) (store.Session, error) {
	return store.Session{}, nil
}

type stubSessions struct {
	deactivated store.ID
	err         error
}

func (s *stubSessions) FindByToken(context.Context, string) (store.Session, error) {
	return store.Session{}, nil
}
func (s *stubSessions) Deactivate(_ context.Context, id store.ID) error {
	s.deactivated = id
	return s.err
}
func (s *stubSessions) ResolveCompany(context.Context, string, string, store.ID) (store.ID, error) {
	return "", nil
}
func (s *stubSessions) BindCompany(context.Context, string, string, store.ID, store.ID) error {
	return nil
}

func TestLogout(t *testing.T) {
	sess := &stubSessions{}
	h := New(stubUsers{}, sess)
	r := httptest.NewRequest("POST", "/", nil)
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", SessionID: "sid1"}))
	rec := httptest.NewRecorder()
	h.Logout(rec, r)
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["code"] != float64(200) || body["message"] != "Logout successful." || body["status"] != true {
		t.Fatalf("logout: %v", body)
	}
	if sess.deactivated != "sid1" {
		t.Errorf("deactivated wrong session: %v", sess.deactivated)
	}
}

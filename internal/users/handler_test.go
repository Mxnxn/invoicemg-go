package users

import (
	"time"
	"context"
	"encoding/json"
	"net/http/httptest"
	neturl "net/url"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

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
func (stubUsers) UpdatePassword(context.Context, store.ID, string) (bool, error) { return true, nil }
func (stubUsers) SetTotpSecret(context.Context, store.ID, string) error         { return nil }
func (stubUsers) SetTotpEnabled(context.Context, store.ID, bool) error          { return nil }
func (stubUsers) ClearTotp(context.Context, store.ID) error                     { return nil }

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
func (s *stubSessions) DeactivateOthers(context.Context, store.ID, string) error { return nil }

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

// --- /user/password/change -------------------------------------------------------------

type pwUsers struct {
	user        store.User
	findErr     error
	updateFound bool
	updatedHash string
	updateCalls int
}

func (s *pwUsers) FindByEmail(context.Context, string) (store.User, error) {
	return store.User{}, store.ErrNotFound
}
func (s *pwUsers) FindByID(context.Context, store.ID) (store.User, error) { return s.user, s.findErr }
func (s *pwUsers) UpdateProfile(context.Context, store.ID, string, string) (store.User, bool, error) {
	return store.User{}, false, nil
}
func (s *pwUsers) CreateSession(context.Context, store.NewSession) (store.Session, error) {
	return store.Session{}, nil
}
func (s *pwUsers) UpdatePassword(_ context.Context, _ store.ID, hash string) (bool, error) {
	s.updatedHash, s.updateCalls = hash, s.updateCalls+1
	return s.updateFound, nil
}

type pwSessions struct {
	othersUID  store.ID
	keptToken  string
	othersDone bool
}

func (s *pwSessions) FindByToken(context.Context, string) (store.Session, error) {
	return store.Session{}, nil
}
func (s *pwSessions) Deactivate(context.Context, store.ID) error { return nil }
func (s *pwSessions) ResolveCompany(context.Context, string, string, store.ID) (store.ID, error) {
	return "", nil
}
func (s *pwSessions) BindCompany(context.Context, string, string, store.ID, store.ID) error {
	return nil
}
func (s *pwSessions) DeactivateOthers(_ context.Context, uid store.ID, keep string) error {
	s.othersUID, s.keptToken, s.othersDone = uid, keep, true
	return nil
}

func hashOf(t *testing.T, pw string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	return string(h)
}

func postPwChange(t *testing.T, u store.Users, s store.Sessions, current, next string) map[string]any {
	t.Helper()
	vals := neturl.Values{}
	vals.Set("currentPassword", current)
	vals.Set("newPassword", next)
	req := httptest.NewRequest("POST", "/user/password/change", strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(auth.WithSession(req.Context(), store.Session{UID: "u1", Token: "keep-tok"}))
	rec := httptest.NewRecorder()
	New(u, s).PasswordChange(rec, req)
	var b map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("not json: %v (%s)", err, rec.Body.String())
	}
	return b
}

func TestPasswordChange_Success(t *testing.T) {
	u := &pwUsers{user: store.User{ID: "u1", PasswordHash: hashOf(t, "oldpass1")}, updateFound: true}
	s := &pwSessions{}
	b := postPwChange(t, u, s, "oldpass1", "newpass9")
	if b["code"] != float64(200) || b["status"] != true || b["message"] != "Password changed. Other devices have been signed out." {
		t.Fatalf("envelope: %v", b)
	}
	if u.updateCalls != 1 || u.updatedHash == "" {
		t.Errorf("password not written: %+v", u)
	}
	// the new hash must verify against the new password
	if bcrypt.CompareHashAndPassword([]byte(u.updatedHash), []byte("newpass9")) != nil {
		t.Error("stored hash does not verify the new password")
	}
	if !s.othersDone || s.othersUID != "u1" || s.keptToken != "keep-tok" {
		t.Errorf("other sessions not retired with the caller's token kept: %+v", s)
	}
}

func TestPasswordChange_Validation(t *testing.T) {
	u := &pwUsers{user: store.User{ID: "u1", PasswordHash: hashOf(t, "oldpass1")}, updateFound: true}
	// missing fields
	if b := postPwChange(t, u, &pwSessions{}, "", "newpass9"); b["code"] != float64(422) || b["message"] != "Both the current and the new password are required." {
		t.Errorf("missing: %v", b)
	}
	// too short
	if b := postPwChange(t, u, &pwSessions{}, "oldpass1", "short"); b["code"] != float64(422) || b["message"] != "Use at least 8 characters." {
		t.Errorf("short: %v", b)
	}
	// same as current
	if b := postPwChange(t, u, &pwSessions{}, "samepass1", "samepass1"); b["code"] != float64(422) || b["message"] != "That is the password you already have." {
		t.Errorf("same: %v", b)
	}
	if u.updateCalls != 0 {
		t.Error("no write should happen on a validation failure")
	}
}

func TestPasswordChange_WrongCurrent(t *testing.T) {
	u := &pwUsers{user: store.User{ID: "u1", PasswordHash: hashOf(t, "oldpass1")}, updateFound: true}
	s := &pwSessions{}
	b := postPwChange(t, u, s, "wrongpass", "newpass9")
	if b["code"] != float64(422) || b["message"] != "That is not your current password." {
		t.Fatalf("wrong current: %v", b)
	}
	if u.updateCalls != 0 || s.othersDone {
		t.Error("a wrong current password must not write or sign anyone out")
	}
}

func TestPasswordChange_AccountGone(t *testing.T) {
	u := &pwUsers{findErr: store.ErrNotFound}
	if b := postPwChange(t, u, &pwSessions{}, "oldpass1", "newpass9"); b["code"] != float64(404) || b["message"] != "Account not found." {
		t.Errorf("missing account should be 404: %v", b)
	}
}

func (s *pwUsers) SetTotpSecret(context.Context, store.ID, string) error { return nil }
func (s *pwUsers) SetTotpEnabled(context.Context, store.ID, bool) error  { return nil }
func (s *pwUsers) ClearTotp(context.Context, store.ID) error             { return nil }

func (stubUsers) Register(context.Context, string, string, string, time.Time) (store.ID, bool, error) {
	return "", false, nil
}

func (s *pwUsers) Register(context.Context, string, string, string, time.Time) (store.ID, bool, error) {
	return "", false, nil
}

func (s *pwUsers) SetActive(context.Context, store.ID, *bool, *time.Time, bool) (store.User, bool, error) {
	return store.User{}, false, nil
}
func (s *pwUsers) SetCompanyLimit(context.Context, store.ID, int) (store.User, bool, error) {
	return store.User{}, false, nil
}
func (s *pwUsers) AdminList(context.Context) ([]store.User, error) { return nil, nil }

func (stubUsers) SetActive(context.Context, store.ID, *bool, *time.Time, bool) (store.User, bool, error) {
	return store.User{}, false, nil
}
func (stubUsers) SetCompanyLimit(context.Context, store.ID, int) (store.User, bool, error) {
	return store.User{}, false, nil
}
func (stubUsers) AdminList(context.Context) ([]store.User, error) { return nil, nil }

func (s *stubSessions) LastLoginAt(context.Context, store.ID) (*time.Time, error) { return nil, nil }

func (s *pwSessions) LastLoginAt(context.Context, store.ID) (*time.Time, error) { return nil, nil }

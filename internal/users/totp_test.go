package users

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

// RFC 6238 vector: this base32 secret's 6-digit code at T=59 is 287082.
const rfcSecret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

type totpUsers struct {
	stubUsers
	user       store.User
	gotSecret  string
	gotEnabled *bool
	cleared    bool
}

func (u *totpUsers) FindByID(context.Context, store.ID) (store.User, error) { return u.user, nil }
func (u *totpUsers) SetTotpSecret(_ context.Context, _ store.ID, s string) error {
	u.gotSecret = s
	return nil
}
func (u *totpUsers) SetTotpEnabled(_ context.Context, _ store.ID, on bool) error {
	u.gotEnabled = &on
	return nil
}
func (u *totpUsers) ClearTotp(context.Context, store.ID) error { u.cleared = true; return nil }

func runTotp(u store.Users, fn func(*Handler, http.ResponseWriter, *http.Request), v url.Values) map[string]any {
	h := New(u, nil)
	h.Now = func() time.Time { return time.Unix(59, 0) }
	r := httptest.NewRequest("POST", "/", strings.NewReader(v.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1"}))
	rec := httptest.NewRecorder()
	fn(h, rec, r)
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	return body
}

func TestTotpStatus(t *testing.T) {
	body := runTotp(&totpUsers{user: store.User{TotpEnabled: true}}, (*Handler).TotpStatus, url.Values{})
	if body["data"].(map[string]any)["enabled"] != true {
		t.Errorf("status = %v, want enabled true", body["data"])
	}
}

func TestTotpSetup(t *testing.T) {
	if body := runTotp(&totpUsers{user: store.User{TotpEnabled: true}}, (*Handler).TotpSetup, url.Values{}); body["code"] != float64(422) {
		t.Errorf("already on -> %v, want 422", body["code"])
	}
	u := &totpUsers{user: store.User{Email: "a@b.co"}}
	body := runTotp(u, (*Handler).TotpSetup, url.Values{})
	if body["code"] != float64(200) || u.gotSecret == "" {
		t.Errorf("setup -> code=%v secret stored=%q", body["code"], u.gotSecret)
	}
	if body["data"].(map[string]any)["secret"] != u.gotSecret {
		t.Error("returned secret should match the stored one")
	}
}

func TestTotpEnable(t *testing.T) {
	// no secret yet
	if body := runTotp(&totpUsers{user: store.User{}}, (*Handler).TotpEnable, url.Values{"code": {"287082"}}); body["code"] != float64(422) {
		t.Errorf("no secret -> %v, want 422", body["code"])
	}
	// wrong code
	if body := runTotp(&totpUsers{user: store.User{TotpSecret: rfcSecret}}, (*Handler).TotpEnable, url.Values{"code": {"000000"}}); body["code"] != float64(422) {
		t.Errorf("wrong code -> %v, want 422", body["code"])
	}
	// right code
	u := &totpUsers{user: store.User{TotpSecret: rfcSecret}}
	body := runTotp(u, (*Handler).TotpEnable, url.Values{"code": {"287082"}})
	if body["code"] != float64(200) || u.gotEnabled == nil || !*u.gotEnabled {
		t.Errorf("enable -> code=%v enabled=%v", body["code"], u.gotEnabled)
	}
}

func TestTotpDisable(t *testing.T) {
	if body := runTotp(&totpUsers{user: store.User{TotpEnabled: false}}, (*Handler).TotpDisable, url.Values{"code": {"287082"}}); body["code"] != float64(422) {
		t.Errorf("not on -> %v, want 422", body["code"])
	}
	u := &totpUsers{user: store.User{TotpEnabled: true, TotpSecret: rfcSecret}}
	body := runTotp(u, (*Handler).TotpDisable, url.Values{"code": {"287082"}})
	if body["code"] != float64(200) || !u.cleared {
		t.Errorf("disable -> code=%v cleared=%v", body["code"], u.cleared)
	}
}

func TestTotpReveal(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("pw"), bcrypt.MinCost)
	base := store.User{Email: "a@b.co", TotpEnabled: true, TotpSecret: rfcSecret, PasswordHash: string(hash)}

	if body := runTotp(&totpUsers{user: base}, (*Handler).TotpReveal, url.Values{}); body["code"] != float64(422) {
		t.Errorf("no password -> %v, want 422", body["code"])
	}
	if body := runTotp(&totpUsers{user: base}, (*Handler).TotpReveal, url.Values{"password": {"wrong"}}); body["code"] != float64(422) {
		t.Errorf("wrong password -> %v, want 422", body["code"])
	}
	body := runTotp(&totpUsers{user: base}, (*Handler).TotpReveal, url.Values{"password": {"pw"}})
	if body["code"] != float64(200) || body["data"].(map[string]any)["secret"] != rfcSecret {
		t.Errorf("reveal -> %v", body)
	}
}

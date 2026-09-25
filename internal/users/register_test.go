package users

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type regUsers struct {
	stubUsers
	existing  bool // FindByEmail finds a user
	gotEmail  string
	gotName   string
	registered bool
}

func (u *regUsers) FindByEmail(_ context.Context, _ string) (store.User, error) {
	if u.existing {
		return store.User{ID: "u0"}, nil
	}
	return store.User{}, store.ErrNotFound
}
func (u *regUsers) Register(_ context.Context, email, _, name string, _ time.Time) (store.ID, bool, error) {
	u.registered = true
	u.gotEmail, u.gotName = email, name
	return "u9", false, nil
}

type regTokens struct {
	tok       store.RegistrationToken
	found     bool
	markedFor store.ID
}

func (r *regTokens) FindByToken(context.Context, string) (store.RegistrationToken, bool, error) {
	return r.tok, r.found, nil
}
func (r *regTokens) MarkUsed(_ context.Context, _, uid store.ID) error { r.markedFor = uid; return nil }
func (r *regTokens) Create(context.Context, string, time.Time) (store.RegistrationToken, error) {
	return store.RegistrationToken{}, nil
}
func (r *regTokens) List(context.Context) ([]store.RegistrationToken, error) { return nil, nil }

func runRegister(u store.Users, rt store.RegistrationTokens, v url.Values) map[string]any {
	h := New(u, nil)
	h.RegTokens = rt
	h.Now = func() time.Time { return time.Unix(1_700_000_000, 0) }
	r := httptest.NewRequest("POST", "/", strings.NewReader(v.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.Register(rec, r)
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	return body
}

func liveToken() store.RegistrationToken {
	return store.RegistrationToken{ID: "t1", Token: "abc", ExpiresAt: time.Unix(1_700_000_000, 0).Add(time.Hour)}
}

func TestRegister_Validation(t *testing.T) {
	if b := runRegister(&regUsers{}, &regTokens{}, url.Values{"email": {"a@b.co"}, "password": {"pw"}}); b["code"] != float64(422) {
		t.Errorf("no name -> %v, want 422", b["code"])
	}
	if b := runRegister(&regUsers{}, &regTokens{}, url.Values{"email": {"a@b.co"}, "password": {"pw"}, "name": {"A"}}); b["code"] != float64(422) {
		t.Errorf("no token -> %v, want 422", b["code"])
	}
}

func TestRegister_BadToken(t *testing.T) {
	v := url.Values{"email": {"a@b.co"}, "password": {"pw"}, "name": {"A"}, "registrationToken": {"x"}}
	if b := runRegister(&regUsers{}, &regTokens{found: false}, v); b["code"] != float64(401) {
		t.Errorf("unknown token -> %v, want 401", b["code"])
	}
	expired := store.RegistrationToken{ID: "t1", ExpiresAt: time.Unix(1, 0)}
	if b := runRegister(&regUsers{}, &regTokens{found: true, tok: expired}, v); b["code"] != float64(401) {
		t.Errorf("expired token -> %v, want 401", b["code"])
	}
}

func TestRegister_DupAndSuccess(t *testing.T) {
	v := url.Values{"email": {"a@b.co"}, "password": {"pw"}, "name": {"Asha"}, "registrationToken": {"abc"}}
	if b := runRegister(&regUsers{existing: true}, &regTokens{found: true, tok: liveToken()}, v); b["code"] != float64(401) {
		t.Errorf("existing email -> %v, want 401", b["code"])
	}
	rt := &regTokens{found: true, tok: liveToken()}
	ru := &regUsers{}
	b := runRegister(ru, rt, v)
	if b["code"] != float64(200) || !ru.registered || rt.markedFor != "u9" {
		t.Errorf("success -> code=%v registered=%v marked=%v", b["code"], ru.registered, rt.markedFor)
	}
}

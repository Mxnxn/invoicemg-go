package devadmin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type devUsers struct {
	store.Users
	user      store.User
	notFound  bool
	gotLimit  int
	gotActive *bool
}

func (u *devUsers) FindByID(context.Context, store.ID) (store.User, error) {
	if u.notFound {
		return store.User{}, store.ErrNotFound
	}
	return u.user, nil
}
func (u *devUsers) SetActive(_ context.Context, _ store.ID, isActive *bool, _ *time.Time, _ bool) (store.User, bool, error) {
	u.gotActive = isActive
	return u.user, true, nil
}
func (u *devUsers) SetCompanyLimit(_ context.Context, _ store.ID, limit int) (store.User, bool, error) {
	u.gotLimit = limit
	uu := u.user
	uu.CompanyLimit = limit
	return uu, true, nil
}

type devCompanies struct {
	store.Companies
	count int
}

func (c *devCompanies) Count(context.Context, store.ID) (int, error) { return c.count, nil }

func devRun(u store.Users, c store.Companies, fn func(*Handler, http.ResponseWriter, *http.Request), v url.Values) map[string]any {
	h := &Handler{users: u, companies: c}
	r := httptest.NewRequest("POST", "/", strings.NewReader(v.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	fn(h, rec, r)
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	return body
}

func TestAdminStatus(t *testing.T) {
	// missing uid
	if b := devRun(&devUsers{}, nil, (*Handler).AdminStatus, url.Values{}); b["code"] != float64(422) {
		t.Errorf("no uid -> %v, want 422", b["code"])
	}
	// nothing to update
	if b := devRun(&devUsers{user: store.User{Role: "admin"}}, nil, (*Handler).AdminStatus, url.Values{"uid": {"u1"}}); b["code"] != float64(422) {
		t.Errorf("nothing to update -> %v, want 422", b["code"])
	}
	// superadmin refused
	if b := devRun(&devUsers{user: store.User{Role: "superadmin"}}, nil, (*Handler).AdminStatus, url.Values{"uid": {"u1"}, "is_active": {"false"}}); b["code"] != float64(403) {
		t.Errorf("superadmin -> %v, want 403", b["code"])
	}
	// not found
	if b := devRun(&devUsers{notFound: true}, nil, (*Handler).AdminStatus, url.Values{"uid": {"u1"}, "is_active": {"false"}}); b["code"] != float64(404) {
		t.Errorf("missing -> %v, want 404", b["code"])
	}
	// ok
	u := &devUsers{user: store.User{Role: "admin", Name: "A", Email: "a@b.co"}}
	if b := devRun(u, nil, (*Handler).AdminStatus, url.Values{"uid": {"u1"}, "is_active": {"false"}}); b["code"] != float64(200) || u.gotActive == nil || *u.gotActive {
		t.Errorf("ok -> code=%v active=%v", b["code"], u.gotActive)
	}
}

func TestAdminCompanyLimit(t *testing.T) {
	// bad limit
	if b := devRun(&devUsers{}, &devCompanies{}, (*Handler).AdminCompanyLimit, url.Values{"uid": {"u1"}, "companyLimit": {"0"}}); b["code"] != float64(422) {
		t.Errorf("limit 0 -> %v, want 422", b["code"])
	}
	// below owned
	u := &devUsers{user: store.User{Name: "A"}}
	if b := devRun(u, &devCompanies{count: 3}, (*Handler).AdminCompanyLimit, url.Values{"uid": {"u1"}, "companyLimit": {"2"}}); b["code"] != float64(422) {
		t.Errorf("below owned -> %v, want 422", b["code"])
	}
	// ok
	u2 := &devUsers{user: store.User{Name: "A", Email: "a@b.co"}}
	b := devRun(u2, &devCompanies{count: 1}, (*Handler).AdminCompanyLimit, url.Values{"uid": {"u1"}, "companyLimit": {"5"}})
	if b["code"] != float64(200) || u2.gotLimit != 5 {
		t.Errorf("ok -> code=%v limit=%v", b["code"], u2.gotLimit)
	}
}

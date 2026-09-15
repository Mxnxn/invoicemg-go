package person

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	neturl "net/url"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type loginPeople struct {
	emp   store.EmployeeAuth
	found bool
}

func (l *loginPeople) FindEmployeeByEmail(context.Context, string) (store.EmployeeAuth, bool, error) {
	return l.emp, l.found, nil
}
func (l *loginPeople) List(context.Context, store.ID, string) ([]store.Person, error) {
	return nil, nil
}
func (l *loginPeople) Create(context.Context, store.ID, store.PersonWrite) (store.Person, bool, error) {
	return store.Person{}, false, nil
}
func (l *loginPeople) Update(context.Context, store.ID, store.ID, store.PersonPatch) (store.Person, bool, bool, error) {
	return store.Person{}, false, false, nil
}
func (l *loginPeople) Delete(context.Context, store.ID, store.ID) (bool, error) { return false, nil }

type loginUsers struct{ got store.NewSession }

func (u *loginUsers) FindByEmail(context.Context, string) (store.User, error) {
	return store.User{}, store.ErrNotFound
}
func (u *loginUsers) FindByID(context.Context, store.ID) (store.User, error) {
	return store.User{}, nil
}
func (u *loginUsers) UpdateProfile(context.Context, store.ID, string, string) (store.User, bool, error) {
	return store.User{}, false, nil
}
func (u *loginUsers) CreateSession(_ context.Context, s store.NewSession) (store.Session, error) {
	u.got = s
	return store.Session{SessionID: "sid", UID: s.UID, Role: s.Role, PersonID: s.PersonID, Permissions: s.Permissions}, nil
}

func postLogin(t *testing.T, p store.People, u store.Users, form map[string]string) map[string]any {
	t.Helper()
	vals := neturl.Values{}
	for k, v := range form {
		vals.Set(k, v)
	}
	r := httptest.NewRequest("POST", "/", strings.NewReader(vals.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	New(p, u).Login(rec, r)
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("not json: %v (%s)", err, rec.Body.String())
	}
	return body
}

func TestPersonLogin(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.DefaultCost)
	emp := store.EmployeeAuth{ID: "p1", UID: "u1", Name: "Anita", Email: "a@x.test", PasswordHash: string(hash), IsActive: true, Permissions: []string{"invoices:create"}}

	u := &loginUsers{}
	body := postLogin(t, &loginPeople{emp: emp, found: true}, u, map[string]string{"email": "a@x.test", "password": "secret"})
	if body["code"] != float64(200) || body["status"] != true {
		t.Fatalf("login: %v", body)
	}
	data := body["data"].(map[string]any)
	if data["role"] != "employee" || data["name"] != "Anita" || data["email"] != "a@x.test" {
		t.Errorf("data: %v", data)
	}
	if u.got.PersonID != "p1" || u.got.Role != "employee" || len(u.got.Permissions) != 1 {
		t.Errorf("session not created for employee: %+v", u.got)
	}

	// wrong password -> 422 Invalid credential.
	body = postLogin(t, &loginPeople{emp: emp, found: true}, &loginUsers{}, map[string]string{"email": "a@x.test", "password": "nope"})
	if body["code"] != float64(422) || body["message"] != "Invalid credential." {
		t.Errorf("wrong pw: %v", body)
	}
	// unknown email -> 422 Email doesn't exist.
	body = postLogin(t, &loginPeople{found: false}, &loginUsers{}, map[string]string{"email": "x", "password": "y"})
	if body["message"] != "Email doesn't exist." {
		t.Errorf("unknown: %v", body)
	}
	// disabled -> 401
	dis := emp
	dis.IsActive = false
	body = postLogin(t, &loginPeople{emp: dis, found: true}, &loginUsers{}, map[string]string{"email": "a@x.test", "password": "secret"})
	if body["code"] != float64(401) || body["message"] != "This account has been disabled." {
		t.Errorf("disabled: %v", body)
	}
	// missing fields -> 422 Invalid request.
	body = postLogin(t, &loginPeople{}, &loginUsers{}, map[string]string{"email": "a@x.test"})
	if body["code"] != float64(422) || body["message"] != "Invalid request." {
		t.Errorf("missing: %v", body)
	}
}

// The employee portal login (routes/Person.js /login). Unlike the admin login it is
// unauthenticated (it issues the session) and produces an "employee" session carrying the
// person's id and permissions snapshot.
package person

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"golang.org/x/crypto/bcrypt"

	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Login is POST /person/login: authenticate an Employee against their portal password and open
// an "employee" session. Node's exact messages are reproduced, including the 422 that admits an
// unknown email ("Email doesn't exist.") - the employee portal is not a public sign-up surface,
// so it does not conceal which employee addresses exist the way the admin login does.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	form, _ := httpx.ReadForm(r)
	email, password := form.String("email"), form.String("password")
	if email == "" || password == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	emp, found, err := h.store.FindEmployeeByEmail(ctx, email)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found || emp.PasswordHash == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Email doesn't exist.", Status: httpx.False()})
		return
	}
	if !emp.IsActive {
		httpx.Write(w, httpx.Envelope{Code: 401, Message: "This account has been disabled.", Status: httpx.False()})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(emp.PasswordHash), []byte(password)); err != nil {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid credential.", Status: httpx.False()})
		return
	}

	token, err := newToken()
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if _, err := h.users.CreateSession(ctx, store.NewSession{
		Token:       token,
		UID:         emp.UID,
		Role:        "employee",
		PersonID:    emp.ID,
		Permissions: emp.Permissions,
	}); err != nil {
		httpx.Internal(w, err)
		return
	}
	perms := emp.Permissions
	if perms == nil {
		perms = []string{}
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Status: httpx.True(), Data: map[string]any{
		"token":       token,
		"email":       emp.Email,
		"uid":         string(emp.UID),
		"role":        "employee",
		"name":        emp.Name,
		"permissions": perms,
	}})
}

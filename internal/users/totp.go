package users

import (
	"errors"
	"net/http"

	"golang.org/x/crypto/bcrypt"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
	"github.com/mxnxn/invoicemg-go/internal/totp"
)

func (h *Handler) currentUser(r *http.Request) (store.User, bool) {
	sess := auth.MustFrom(r.Context())
	u, err := h.Users.FindByID(r.Context(), sess.UID)
	if errors.Is(err, store.ErrNotFound) {
		return store.User{}, false
	}
	return u, err == nil
}

// TotpStatus is POST /user/totp/status: whether 2FA is on for this account.
func (h *Handler) TotpStatus(w http.ResponseWriter, r *http.Request) {
	u, ok := h.currentUser(r)
	if !ok {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Account not found.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Status: httpx.True(), Data: map[string]any{"enabled": u.TotpEnabled}})
}

// TotpSetup is POST /user/totp/setup: mint a secret to enrol (not yet enabled).
func (h *Handler) TotpSetup(w http.ResponseWriter, r *http.Request) {
	u, ok := h.currentUser(r)
	if !ok {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Account not found.", Status: httpx.False()})
		return
	}
	if u.TotpEnabled {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Two-factor is already on. Turn it off first to re-enrol.", Status: httpx.False()})
		return
	}
	secret, err := totp.GenerateSecret()
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if err := h.Users.SetTotpSecret(r.Context(), u.ID, secret); err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Status: httpx.True(), Data: map[string]any{
		"secret": secret, "otpauth": totp.OtpauthURI(secret, u.Email),
	}})
}

// TotpEnable is POST /user/totp/enable: confirm enrolment with a current code.
func (h *Handler) TotpEnable(w http.ResponseWriter, r *http.Request) {
	u, ok := h.currentUser(r)
	if !ok {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Account not found.", Status: httpx.False()})
		return
	}
	if u.TotpSecret == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Start the setup first.", Status: httpx.False()})
		return
	}
	form, _ := httpx.ReadForm(r)
	if !totp.Verify(u.TotpSecret, form.String("code"), h.Now()) {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "That code isn't right. Codes change every 30 seconds - try the current one.", Status: httpx.False()})
		return
	}
	if err := h.Users.SetTotpEnabled(r.Context(), u.ID, true); err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Status: httpx.True(), Message: "Two-factor is on. You'll be asked for a code next time you sign in."})
}

// TotpDisable is POST /user/totp/disable: turn 2FA off, requiring a current code (not just a session).
func (h *Handler) TotpDisable(w http.ResponseWriter, r *http.Request) {
	u, ok := h.currentUser(r)
	if !ok {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Account not found.", Status: httpx.False()})
		return
	}
	if !u.TotpEnabled {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Two-factor isn't on.", Status: httpx.False()})
		return
	}
	form, _ := httpx.ReadForm(r)
	if !totp.Verify(u.TotpSecret, form.String("code"), h.Now()) {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Enter a current code to turn two-factor off.", Status: httpx.False()})
		return
	}
	if err := h.Users.ClearTotp(r.Context(), u.ID); err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Status: httpx.True(), Message: "Two-factor is off."})
}

// TotpReveal is POST /user/totp/reveal: show the enrolled secret again, gated on the password (so a
// second device can be added without disabling 2FA).
func (h *Handler) TotpReveal(w http.ResponseWriter, r *http.Request) {
	form, _ := httpx.ReadForm(r)
	password := form.String("password")
	if password == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Enter your account password.", Status: httpx.False()})
		return
	}
	u, ok := h.currentUser(r)
	if !ok {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Account not found.", Status: httpx.False()})
		return
	}
	if !u.TotpEnabled || u.TotpSecret == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Two-factor isn't on yet. Set it up first.", Status: httpx.False()})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "That is not your account password.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Status: httpx.True(), Data: map[string]any{
		"secret": u.TotpSecret, "otpauth": totp.OtpauthURI(u.TotpSecret, u.Email),
	}})
}

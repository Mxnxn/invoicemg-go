package devadmin

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
	"github.com/mxnxn/invoicemg-go/internal/tokenexpiry"
	"github.com/mxnxn/invoicemg-go/internal/totp"
)

const (
	devRateLimit  = 10
	devRateWindow = time.Minute
)

func createTokenValue() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

type regTokenDTO struct {
	Token     string     `json:"token"`
	ExpiresAt httpx.Time `json:"expiresAt"`
}

// RegistrationTokenCreate is POST /dev/registration-token/create (superadmin).
func (h *Handler) RegistrationTokenCreate(w http.ResponseWriter, r *http.Request) {
	form, _ := httpx.ReadForm(r)
	expiresAt, ok, msg := tokenexpiry.Resolve(form.String("expiry"), h.now())
	if !ok {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: msg, Status: httpx.False()})
		return
	}
	t, err := h.regTokens.Create(r.Context(), createTokenValue(), expiresAt)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Registration token issued.", Status: httpx.True(),
		Data: regTokenDTO{Token: t.Token, ExpiresAt: httpx.NewTime(t.ExpiresAt)}})
}

// RegistrationTokenList is POST /dev/registration-token/list (superadmin).
func (h *Handler) RegistrationTokenList(w http.ResponseWriter, r *http.Request) {
	list, err := h.regTokens.List(r.Context())
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	type row struct {
		ID        string      `json:"_id"`
		Token     string      `json:"token"`
		ExpiresAt httpx.Time  `json:"expiresAt"`
		UsedAt    *httpx.Time `json:"usedAt"`
		UsedBy    *string     `json:"usedBy"`
		CreatedAt httpx.Time  `json:"createdAt"`
	}
	out := make([]row, 0, len(list))
	for _, t := range list {
		rw := row{ID: string(t.ID), Token: t.Token, ExpiresAt: httpx.NewTime(t.ExpiresAt), CreatedAt: httpx.NewTime(t.CreatedAt)}
		if t.UsedAt != nil {
			v := httpx.NewTime(*t.UsedAt)
			rw.UsedAt = &v
		}
		if t.UsedBy != "" {
			s := string(t.UsedBy)
			rw.UsedBy = &s
		}
		out = append(out, rw)
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Status: httpx.True(), Data: out})
}

// RegistrationTokenGet is GET /dev/registration-token?code=NNNNNN[&expiry=yyyymmdd]. Unauthenticated
// but DEV_TOTP_SECRET-gated, rate-limited, and each accepted step is burned so one observed code
// cannot mint more than one token.
func (h *Handler) RegistrationTokenGet(w http.ResponseWriter, r *http.Request) {
	now := h.now()
	if !h.checkRate(now) {
		httpx.Write(w, httpx.Envelope{Code: 429, Message: "Too many attempts. Try again shortly.", Status: httpx.False()})
		return
	}
	if h.devTotpSecret == "" {
		httpx.Write(w, httpx.Envelope{Code: 500, Message: "Internal error", Status: httpx.False()})
		return
	}
	ok, step := totp.VerifyStep(h.devTotpSecret, r.URL.Query().Get("code"), now)
	if !ok {
		httpx.Write(w, httpx.Envelope{Code: 401, Message: "Invalid code.", Status: httpx.False()})
		return
	}
	if !h.consumeStep(step, now) {
		httpx.Write(w, httpx.Envelope{Code: 401, Message: "Code already used.", Status: httpx.False()})
		return
	}
	expiresAt, valid, msg := tokenexpiry.Resolve(r.URL.Query().Get("expiry"), now)
	if !valid {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: msg, Status: httpx.False()})
		return
	}
	t, err := h.regTokens.Create(r.Context(), createTokenValue(), expiresAt)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Registration token issued.", Status: httpx.True(),
		Data: regTokenDTO{Token: t.Token, ExpiresAt: httpx.NewTime(t.ExpiresAt)}})
}

func (h *Handler) checkRate(now time.Time) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	kept := h.attempts[:0]
	for _, at := range h.attempts {
		if now.Sub(at) < devRateWindow {
			kept = append(kept, at)
		}
	}
	h.attempts = kept
	if len(h.attempts) >= devRateLimit {
		return false
	}
	h.attempts = append(h.attempts, now)
	return true
}

func (h *Handler) consumeStep(step int64, now time.Time) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s, at := range h.usedSteps {
		if now.Sub(at) > 5*time.Minute {
			delete(h.usedSteps, s)
		}
	}
	if _, used := h.usedSteps[step]; used {
		return false
	}
	h.usedSteps[step] = now
	return true
}

// PasswordRequests is POST /dev/password-requests (superadmin): the pending reset queue.
func (h *Handler) PasswordRequests(w http.ResponseWriter, r *http.Request) {
	list, err := h.passwordResets.ListPending(r.Context())
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	type row struct {
		ID        string     `json:"_id"`
		Email     string     `json:"email"`
		UID       *string    `json:"uid"`
		Status    string     `json:"status"`
		CreatedAt httpx.Time `json:"createdAt"`
	}
	out := make([]row, 0, len(list))
	for _, p := range list {
		rw := row{ID: string(p.ID), Email: p.Email, Status: p.Status, CreatedAt: httpx.NewTime(p.CreatedAt)}
		if p.UID != "" {
			s := string(p.UID)
			rw.UID = &s
		}
		out = append(out, rw)
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Status: httpx.True(), Data: out})
}

// PasswordRequestsResolve is POST /dev/password-requests/resolve (superadmin): dismiss, or reset the
// account to a one-time temporary password (read out once, never stored) and kill its sessions.
func (h *Handler) PasswordRequestsResolve(w http.ResponseWriter, r *http.Request) {
	form, _ := httpx.ReadForm(r)
	id := form.String("request_id")
	if id == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	req, found, err := h.passwordResets.Get(r.Context(), store.ID(id))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Request not found.", Status: httpx.False()})
		return
	}
	if form.String("action") == "dismiss" {
		if err := h.passwordResets.Resolve(r.Context(), req.ID, "dismissed"); err != nil {
			httpx.Internal(w, err)
			return
		}
		httpx.Write(w, httpx.Envelope{Code: 200, Status: httpx.True(), Message: "Request dismissed."})
		return
	}

	// Resolve to a reset: find the account (by uid, else by email).
	var user store.User
	if req.UID != "" {
		user, err = h.users.FindByID(r.Context(), req.UID)
	} else {
		user, err = h.users.FindByEmail(r.Context(), req.Email)
	}
	if err != nil {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "No account with that address - nothing to reset.", Status: httpx.False()})
		return
	}

	temporary := temporaryPassword()
	hash, err := bcrypt.GenerateFromPassword([]byte(temporary), 10)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if _, err := h.users.UpdatePassword(r.Context(), user.ID, string(hash)); err != nil {
		httpx.Internal(w, err)
		return
	}
	// Everything that account had open dies with the old password (keepToken "" retires all).
	_ = h.sessions.DeactivateOthers(r.Context(), user.ID, "")
	if err := h.passwordResets.Resolve(r.Context(), req.ID, "done"); err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Status: httpx.True(),
		Message: "Password reset. Read this out once - it is not stored anywhere.",
		Data:    map[string]any{"email": user.Email, "temporaryPassword": temporary}})
}

func temporaryPassword() string {
	b := make([]byte, 9)
	_, _ = rand.Read(b)
	s := base64.RawURLEncoding.EncodeToString(b)
	if len(s) > 12 {
		s = s[:12]
	}
	return s
}

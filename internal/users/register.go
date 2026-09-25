package users

import (
	"errors"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

func redeemable(t store.RegistrationToken, now time.Time) bool {
	return t.UsedAt == nil && t.ExpiresAt.After(now)
}

// defaultActiveUntil is Helpers/Tenancy.defaultActiveUntil: one year out.
func defaultActiveUntil(now time.Time) time.Time { return now.AddDate(1, 0, 0) }

// Register is POST /user/register: create an account by redeeming a registration token.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	form, _ := httpx.ReadForm(r)
	email := normaliseEmail(form.String("email"))
	password := form.String("password")
	name := form.String("name")
	regToken := form.String("registrationToken")

	if email == "" || password == "" || name == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	if regToken == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Registration token is required.", Status: httpx.False()})
		return
	}
	tok, found, err := h.RegTokens.FindByToken(r.Context(), regToken)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found || !redeemable(tok, h.Now()) {
		httpx.Write(w, httpx.Envelope{Code: 401, Message: "Invalid or expired registration token.", Status: httpx.False()})
		return
	}

	// Pre-check for a friendlier 401 (Register also guards with the unique index).
	if _, err := h.Users.FindByEmail(r.Context(), email); err == nil {
		httpx.Write(w, httpx.Envelope{Code: 401, Message: "Email already Exists.", Status: httpx.False()})
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		httpx.Internal(w, err)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	uid, dup, err := h.Users.Register(r.Context(), email, string(hash), name, defaultActiveUntil(h.Now()))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if dup {
		httpx.Write(w, httpx.Envelope{Code: 401, Message: "Email already Exists.", Status: httpx.False()})
		return
	}
	if err := h.RegTokens.MarkUsed(r.Context(), tok.ID, uid); err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Registration successful", Status: httpx.True()})
}

// PasswordRequestReset is POST /user/password/request-reset: queue a reset ask. Always answers the
// same, so the form cannot be used to learn which addresses have accounts.
func (h *Handler) PasswordRequestReset(w http.ResponseWriter, r *http.Request) {
	form, _ := httpx.ReadForm(r)
	email := normaliseEmail(form.String("email"))
	same := httpx.Envelope{Code: 200, Status: httpx.True(), Message: "If that address has an account, the team has been asked to reset it."}
	if email == "" {
		httpx.Write(w, same)
		return
	}
	pending, err := h.PasswordResets.HasPending(r.Context(), email)
	if err == nil && !pending {
		var uid store.ID
		if u, e := h.Users.FindByEmail(r.Context(), email); e == nil {
			uid = u.ID
		}
		_ = h.PasswordResets.Create(r.Context(), email, uid)
	}
	httpx.Write(w, same)
}

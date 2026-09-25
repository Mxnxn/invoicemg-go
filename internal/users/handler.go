// Package users is signing in.
//
// It is here because nothing else in the application can be reached without it: the React
// client gates every route on a uid in localStorage, which only a successful login writes. The
// local all-in-one stack needs this to be usable at all.
package users

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
	"github.com/mxnxn/invoicemg-go/internal/totp"
)

type Handler struct {
	Users    store.Users
	Sessions store.Sessions
	Now      func() time.Time
}

func New(users store.Users, sessions store.Sessions) *Handler {
	return &Handler{Users: users, Sessions: sessions, Now: time.Now}
}

// Session lifetimes, from Helpers/SessionLifetime.js.
const (
	defaultHours   = 12
	rememberedDays = 30
)

func lifetime(remembered bool) time.Duration {
	if remembered {
		return time.Duration(rememberedDays) * 24 * time.Hour
	}
	return time.Duration(defaultHours) * time.Hour
}

// normaliseEmail is Helpers/NormalizeEmail.js: lowercase and trimmed. It lives on the caller's
// side of the store so both backends cannot disagree about what "the same address" means.
func normaliseEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// flags are the capability booleans the client reads instead of inferring authorisation from
// the role string - Helpers/SessionFlags.js. UI hints only: every endpoint still enforces its
// own rules, so a tampered flag buys nothing.
type flags struct {
	IsOwner            bool     `json:"isOwner"`
	IsSuperadmin       bool     `json:"isSuperadmin"`
	IsEmployee         bool     `json:"isEmployee"`
	CanManageCompanies bool     `json:"canManageCompanies"`
	CanManageAccount   bool     `json:"canManageAccount"`
	CanManagePeople    bool     `json:"canManagePeople"`
	CanOpenDevPanel    bool     `json:"canOpenDevPanel"`
	Permissions        []string `json:"permissions"`
}

func sessionFlags(role string, permissions []string) flags {
	isSuperadmin := role == "superadmin"
	isOwner := role == "admin" || isSuperadmin
	if permissions == nil {
		permissions = []string{}
	}
	return flags{
		IsOwner:            isOwner,
		IsSuperadmin:       isSuperadmin,
		IsEmployee:         role == "employee",
		CanManageCompanies: isOwner,
		CanManageAccount:   isOwner,
		CanManagePeople:    isOwner,
		CanOpenDevPanel:    isSuperadmin,
		Permissions:        permissions,
	}
}

type loginData struct {
	Token     string      `json:"token"`
	Email     string      `json:"email"`
	UID       string      `json:"uid"`
	Role      string      `json:"role"`
	Flags     flags       `json:"flags"`
	ExpiresAt *httpx.Time `json:"expiresAt"`
}

// newToken is the session key.
//
// The Node route signs a JWT carrying a random uuid. That signature is never verified by
// anything: TokenHelper looks the string up in the sessions collection, so the JWT was doing
// no work beyond being unique and opaque. 32 random bytes are both, without a signing key to
// configure or leak. The client treats it as an opaque string either way.
func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Login is POST /user/login.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	form, err := httpx.ReadForm(r)
	if err != nil {
		httpx.Invalid(w, "")
		return
	}
	email, password := form.String("email"), form.String("password")
	if email == "" || password == "" {
		httpx.Invalid(w, "")
		return
	}

	user, err := h.Users.FindByEmail(ctx, normaliseEmail(email))
	if errors.Is(err, store.ErrNotFound) {
		// The SAME message a wrong password gets, deliberately. Saying "that email doesn't
		// exist" turns the login form into a directory: anyone can submit addresses and learn
		// which ones hold accounts here. The person who genuinely mistyped is no worse off -
		// they retype it either way.
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid credential.", Status: httpx.False()})
		return
	}
	if err != nil {
		httpx.Internal(w, err)
		return
	}

	// bcrypt.CompareHashAndPassword is constant-time and reads the cost and salt out of the
	// stored hash, so hashes written by bcryptjs ($2a$) verify here unchanged.
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid credential.", Status: httpx.False()})
		return
	}

	// Second factor, when the account has one. Checked AFTER the password, so a wrong password
	// never reveals whether an account has 2FA - and no session exists until both have passed.
	if user.TotpEnabled {
		code := strings.TrimSpace(form.String("totp"))
		if code == "" {
			// No code yet: tell the client to ask for one. A success envelope with no session -
			// totpRequired is how the login form knows to show the code field.
			httpx.Write(w, httpx.Envelope{Code: 200, Status: httpx.True(), TotpRequired: true,
				Message: "Enter the 6-digit code from your authenticator app."})
			return
		}
		if !totp.Verify(user.TotpSecret, code, h.Now()) {
			httpx.Write(w, httpx.Envelope{Code: 422, Status: httpx.False(), TotpRequired: true,
				Message: "That code isn't right. Codes change every 30 seconds - try the current one."})
			return
		}
	}

	// What actually locks a login out, checked before a session is created rather than after.
	if user.ActiveUntil != nil && !user.ActiveUntil.After(h.Now()) {
		httpx.Write(w, httpx.Envelope{
			Code:    403,
			Message: "This account is inactive. Please contact support.",
			Status:  httpx.False(),
		})
		return
	}

	token, err := newToken()
	if err != nil {
		httpx.Internal(w, err)
		return
	}

	role := user.Role
	if role == "" {
		role = "admin"
	}
	remembered := form.Bool("remember")
	expires := h.Now().Add(lifetime(remembered))

	created, err := h.Users.CreateSession(ctx, store.NewSession{
		Token:      token,
		UID:        user.ID,
		Role:       role,
		Remembered: remembered,
		ExpiresAt:  &expires,
	})
	if err != nil {
		httpx.Internal(w, err)
		return
	}

	httpx.Write(w, httpx.Envelope{
		Code:   200,
		Status: httpx.True(),
		Data: loginData{
			Token: token,
			Email: user.Email,
			UID:   user.ID.String(),
			// The client stores this to gate the sidebar. When it was absent, localStorage
			// held the literal string "undefined".
			Role:      role,
			Flags:     sessionFlags(role, created.Permissions),
			ExpiresAt: httpx.NewTimePtr(created.ExpiresAt),
		},
	})
}

// Logout is POST /user/logout: retire the caller's session so its token can't be replayed.
// Runs behind a valid-session guard, so the session is already resolved on the context.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	if err := h.Sessions.Deactivate(r.Context(), sess.SessionID); err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Logout successful.", Status: httpx.True()})
}

// PasswordChange is POST /user/password/change: re-set the admin's own password after proving
// the current one. Behind a valid session only. The current password (not merely a live
// session) is required, so an unattended signed-in browser is not enough to take the account
// over; and on success every OTHER session is retired while this one survives, so the person is
// not thrown out mid-task. Validation runs in Node's order.
func (h *Handler) PasswordChange(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)
	form, _ := httpx.ReadForm(r)

	current := form.String("currentPassword")
	next := form.String("newPassword")
	if current == "" || next == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Both the current and the new password are required.", Status: httpx.False()})
		return
	}
	if utf8.RuneCountInString(next) < 8 {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Use at least 8 characters.", Status: httpx.False()})
		return
	}
	if next == current {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "That is the password you already have.", Status: httpx.False()})
		return
	}

	user, err := h.Users.FindByID(ctx, sess.UID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httpx.Write(w, httpx.Envelope{Code: 404, Message: "Account not found.", Status: httpx.False()})
			return
		}
		httpx.Internal(w, err)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(current)) != nil {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "That is not your current password.", Status: httpx.False()})
		return
	}

	// genSalt(10) in Node; bcrypt.DefaultCost is 10, so hashes stay cross-compatible.
	hash, err := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	found, err := h.Users.UpdatePassword(ctx, sess.UID, string(hash))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Account not found.", Status: httpx.False()})
		return
	}
	if err := h.Sessions.DeactivateOthers(ctx, sess.UID, sess.Token); err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Password changed. Other devices have been signed out.", Status: httpx.True()})
}

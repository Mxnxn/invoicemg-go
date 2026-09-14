// Package auth reproduces Helpers/TokenHelper.js and Helpers/RoleHelper.js.
//
// "Reproduces" is meant strictly. While both services are live the same browser tab may hit
// either one, so a token or a tab binding that works against Node must work identically here.
// Any difference is a session that behaves differently depending on which service answered -
// the hardest class of bug to reproduce and the easiest to ship.
package auth

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type ctxKey struct{}

// From returns the session a Require middleware attached, and whether there was one.
func From(ctx context.Context) (store.Session, bool) {
	sess, ok := ctx.Value(ctxKey{}).(store.Session)
	return sess, ok
}

// WithSession attaches a session to a context - the inverse of From, and what Require does
// internally. Exported so a handler test can stand in for the middleware and exercise a handler
// directly, without minting a token and threading it through a fake session store.
func WithSession(ctx context.Context, sess store.Session) context.Context {
	return context.WithValue(ctx, ctxKey{}, sess)
}

// MustFrom is for handlers that sit behind Require and would be a programming error without
// it. It panics rather than returning a zero session, because a zero session has an empty
// CompanyID, and an empty CompanyID silently widens every query to "no company" instead of
// failing - which is a tenancy leak, not an error message.
func MustFrom(ctx context.Context) store.Session {
	sess, ok := From(ctx)
	if !ok {
		panic("auth: handler ran without the Require middleware")
	}
	return sess
}

type Middleware struct {
	Sessions store.Sessions
	// Now is injectable so the expiry rule can be tested without waiting.
	Now func() time.Time
}

func New(sessions store.Sessions) *Middleware {
	return &Middleware{Sessions: sessions, Now: time.Now}
}

// Require resolves SESSION-TOKEN and TAB-ID, exactly as tokenHelper does.
func (m *Middleware) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("SESSION-TOKEN")
		if token == "" {
			httpx.Unauthorized(w, "")
			return
		}

		ctx := r.Context()
		sess, err := m.Sessions.FindByToken(ctx, token)
		if errors.Is(err, store.ErrNotFound) {
			httpx.Unauthorized(w, "")
			return
		}
		if err != nil {
			// tokenHelper catches everything and answers 401, including a database failure.
			// Matched deliberately: a 500 here would tell the client the token might be fine,
			// and the frontend's interceptor treats the two differently.
			httpx.Unauthorized(w, "")
			return
		}

		// A session past its expiry is retired on the way through rather than merely refused,
		// so a token that has aged out cannot be replayed and does not sit in the collection
		// forever. A nil ExpiresAt means never - see Model/UserSession.js.
		if sess.IsActive && sess.ExpiresAt != nil && !sess.ExpiresAt.After(m.Now()) {
			if err := m.Sessions.Deactivate(ctx, sess.SessionID); err != nil {
				// Refusing is the important half; failing to record it is not worth a
				// different answer to the client.
				httpx.Unauthorized(w, "Your session has expired. Please sign in again.")
				return
			}
			httpx.Unauthorized(w, "Your session has expired. Please sign in again.")
			return
		}

		if !sess.IsActive {
			httpx.Unauthorized(w, "")
			return
		}

		companyID, err := m.Sessions.ResolveCompany(ctx, token, r.Header.Get("TAB-ID"), sess.UID)
		if err != nil {
			httpx.Unauthorized(w, "")
			return
		}
		sess.CompanyID = companyID

		next.ServeHTTP(w, r.WithContext(WithSession(ctx, sess)))
	})
}

// RequireRole layers on top of Require, as requireRole does in the Node app.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(roles))
	for _, role := range roles {
		allowed[role] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sess, ok := From(r.Context())
			if !ok || !allowed[sess.Role] {
				httpx.Forbidden(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAdmin is requireRole(["admin", "superadmin"]).
func RequireAdmin(next http.Handler) http.Handler {
	return RequireRole("admin", "superadmin")(next)
}

// Chain applies middleware left to right, so Chain(h, Require, RequireAdmin) reads in the
// order the request passes through them.
func Chain(h http.Handler, mw ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h
}

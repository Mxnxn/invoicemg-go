package auth

import (
	"net/http"
	"strings"

	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

// Helpers/Permissions.js, in Go.
//
// A permission is stored as "<feature>:<action>" - "invoices:create". Three actions: view,
// create, delete. Edit rides with create, because the forms are the same screen and splitting
// them produced a matrix nobody could reason about.
//
// Every subtlety below is load-bearing, and each one is a way an employee silently gains or
// loses access if it is not reproduced exactly. While both services are live, the same
// employee hits both, so a difference here is a permission that depends on which service
// answered.

const (
	ActionView   = "view"
	ActionCreate = "create"
	ActionDelete = "delete"
)

// Actions is the closed set. Anything else submitted by an admin's UI is dropped rather than
// stored, so a typo cannot become a permanent grant.
var Actions = []string{ActionView, ActionCreate, ActionDelete}

// HasPermission answers whether this session may do `action` on `feature`.
func HasPermission(sess store.Session, feature, action string) bool {
	if action == "" {
		action = ActionView
	}

	// Admins and superadmins bypass everything.
	//
	// superadmin is listed explicitly and must stay that way. Sessions used to be stamped
	// "admin" for every user, so this only ever saw "admin" or "employee"; once the role
	// became real, a superadmin matched neither branch and was refused every gated route.
	if sess.Role == "admin" || sess.Role == "superadmin" {
		return true
	}
	if sess.Role != "employee" {
		return false
	}

	for _, entry := range sess.Permissions {
		gotFeature, gotAction, hasAction := strings.Cut(entry, ":")
		if gotFeature != feature {
			continue
		}
		// A FLAT key ("invoices", no colon) grants every action on that feature. Every
		// employee predating the action split has flat keys on their Person record and in
		// their live session snapshot, so reading a flat key as view-only would revoke write
		// access from everyone the moment this service answered a request.
		if !hasAction || gotAction == "" {
			return true
		}
		if gotAction == action {
			return true
		}
	}
	return false
}

// RequireFeature is the view-level gate: may this session open the feature at all.
func RequireFeature(feature string) func(http.Handler) http.Handler {
	return requirePermission(feature, ActionView)
}

// RequireCreate and RequireDelete are the write-level gates, layered under RequireFeature on
// the routes that create or destroy.
func RequireCreate(feature string) func(http.Handler) http.Handler {
	return requirePermission(feature, ActionCreate)
}

func RequireDelete(feature string) func(http.Handler) http.Handler {
	return requirePermission(feature, ActionDelete)
}

func requirePermission(feature, action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sess, ok := From(r.Context())
			if !ok || !HasPermission(sess, feature, action) {
				httpx.Forbidden(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// NormalisePermissions cleans whatever an admin's UI submitted into a deduplicated list.
//
// A flat key is EXPANDED on the way in, so stored data is consistent going forward, while
// HasPermission still understands the old shape if it survives on an existing record.
func NormalisePermissions(list []string) []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(entry string) {
		if !seen[entry] {
			seen[entry] = true
			out = append(out, entry)
		}
	}

	for _, raw := range list {
		feature, action, hasAction := strings.Cut(raw, ":")
		if feature == "" {
			continue
		}
		if !hasAction || action == "" {
			for _, a := range Actions {
				add(feature + ":" + a)
			}
			continue
		}
		for _, a := range Actions {
			if a == action {
				add(feature + ":" + action)
				break
			}
		}
	}
	return out
}

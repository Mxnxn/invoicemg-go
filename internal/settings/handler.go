// Package settings serves routes/Settings.js: the per-person UI preferences (appearance and
// per-table columns) that follow whoever is logged in rather than the browser or the firm.
//
// These routes are deliberately behind only a valid session, NOT requireAdmin: an employee has
// the same right to pick a font size as the admin, and the blobs carry no business data. The
// owner is personId for an employee session and uid for an admin one - Model/UserSetting.js
// keys on "whoever is logged in" to keep one code path instead of branching on session type.
package settings

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type Handler struct{ store store.Settings }

func New(s store.Settings) *Handler { return &Handler{store: s} }

// owner is Node's `req.auth.personId || req.auth.uid`: the Person for an employee session, the
// User for an admin one.
func owner(sess store.Session) store.ID {
	if sess.PersonID != "" {
		return sess.PersonID
	}
	return sess.UID
}

// Appearance is POST /settings/appearance: the caller's saved appearance blob, or null on the
// first login before anything has been saved.
func (h *Handler) Appearance(w http.ResponseWriter, r *http.Request) {
	h.read(w, r, store.SettingsAppearance)
}

// AppearanceUpdate is POST /settings/appearance/update: upsert the appearance blob.
func (h *Handler) AppearanceUpdate(w http.ResponseWriter, r *http.Request) {
	h.update(w, r, store.SettingsAppearance, "appearance")
}

// Tables is POST /settings/tables: the caller's saved per-table column blob, or null. Kept
// separate from appearance so dragging a column cannot race a font change and overwrite it.
func (h *Handler) Tables(w http.ResponseWriter, r *http.Request) {
	h.read(w, r, store.SettingsTables)
}

// TablesUpdate is POST /settings/tables/update: upsert the per-table column blob.
func (h *Handler) TablesUpdate(w http.ResponseWriter, r *http.Request) {
	h.update(w, r, store.SettingsTables, "tables")
}

// read answers `{code:200, message:"Operation successful.", status:true, data: blob|null}`.
// A caller with no row yet gets data:null (httpx.Null, so the key is present and null, not
// omitted); once a row exists every section reads back at least {}.
func (h *Handler) read(w http.ResponseWriter, r *http.Request, section store.SettingsSection) {
	sess := auth.MustFrom(r.Context())
	blob, err := h.store.Get(r.Context(), owner(sess), section)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	data := httpx.Null
	if blob != nil {
		data = blob
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Status: httpx.True(), Data: data})
}

// update mirrors the Node handler exactly: the field arrives as a JSON string inside the form
// body, is parsed, and must be a non-array object - anything else (absent, empty, malformed,
// null, an array, or a scalar) is 422 "Invalid request.". The stored value is echoed back with
// "Settings saved.".
func (h *Handler) update(w http.ResponseWriter, r *http.Request, section store.SettingsSection, field string) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)

	var blob json.RawMessage
	// JSON() (no fallback) mirrors `JSON.parse(req.body.x)`: absent, empty and malformed all
	// error, exactly as JSON.parse(undefined) / JSON.parse("") / JSON.parse("{bad") all throw.
	// Node also 422s an absent field via its `!appearance` guard, so every non-object path is
	// the same 422 either way.
	if err := form.JSON(field, &blob); err != nil || !isJSONObject(blob) {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	saved, err := h.store.Set(r.Context(), owner(sess), section, blob)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Settings saved.", Status: httpx.True(), Data: saved})
}

// isJSONObject reports whether raw is a JSON object ({...}) rather than an array, null, or a
// scalar - the Go twin of Node's `typeof x === "object" && !Array.isArray(x)` after a truthy
// check. raw has already passed json validation in form.JSON, so a leading `{` is sufficient:
// a valid JSON value that starts with `{` is an object, and null/[]/numbers/strings/booleans
// begin with something else.
func isJSONObject(raw json.RawMessage) bool {
	t := bytes.TrimSpace(raw)
	return len(t) > 0 && t[0] == '{'
}

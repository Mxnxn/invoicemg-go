package person

import (
	"encoding/json"
	"net/http"

	"golang.org/x/crypto/bcrypt"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

func toDTO(p store.Person) personDTO {
	perms := p.Permissions
	if perms == nil {
		perms = []string{}
	}
	return personDTO{
		ID: string(p.ID), Name: p.Name, Type: p.Type, Email: p.Email, Phone: p.Phone,
		Firm: p.Firm, Address: p.Address, Gst: p.Gst, OpeningBalance: p.OpeningBalance,
		IsActive: p.IsActive, Permissions: perms,
		NotifyPoCreated: p.NotifyPoCreated, NotifyPoUpdated: p.NotifyPoUpdated,
		NotifyPoConfirmed: p.NotifyPoConfirmed, CreatedAt: httpx.NewTime(p.CreatedAt),
	}
}

func invalid(w http.ResponseWriter) {
	httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
}

// parsePermissions decodes a JSON string array and normalises it (flat legacy keys expanded,
// unknown actions dropped). A malformed payload yields ok=false and the caller leaves the field
// untouched, exactly as routes/Person.js swallows the parse error.
func parsePermissions(raw string) ([]string, bool) {
	var parsed []string
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, false
	}
	return auth.NormalisePermissions(parsed), true
}

func hashPassword(w http.ResponseWriter, pw string) (*string, bool) {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), 10)
	if err != nil {
		httpx.Internal(w, err)
		return nil, false
	}
	s := string(h)
	return &s, true
}

// Create is POST /person/create: add an employee or supplier (owner-scoped, admin-only). Only an
// Employee with both an email and a password gets portal credentials, and only an Employee's
// permissions are taken. A duplicate email is a 422, matching Node.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)
	form, _ := httpx.ReadForm(r)

	name := form.String("name")
	typ := form.String("type")
	if name == "" || typ == "" {
		invalid(w)
		return
	}
	in := store.PersonWrite{
		Name: name, Type: typ, Phone: form.String("phone"),
		Firm: form.String("firm"), Address: form.String("address"), Gst: form.String("gst"),
	}
	if email := form.String("email"); email != "" {
		in.Email = &email
	}
	if typ == "Employee" && form.String("email") != "" && form.String("password") != "" {
		hash, ok := hashPassword(w, form.String("password"))
		if !ok {
			return
		}
		in.PasswordHash = hash
	}
	if typ == "Employee" && form.Has("permissions") {
		if perms, ok := parsePermissions(form.String("permissions")); ok {
			in.Permissions = perms
		}
	}

	created, dupEmail, err := h.store.Create(ctx, sess.UID, in)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if dupEmail {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "That email is already registered to a person.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Person created.", Data: toDTO(created)})
}

// Update is POST /person/update: a partial edit of an owner-scoped person. Only submitted fields
// change; a password is re-hashed only when supplied.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)
	form, _ := httpx.ReadForm(r)

	personID := form.String("person_id")
	if personID == "" {
		invalid(w)
		return
	}
	var patch store.PersonPatch
	// name/type only change when non-empty (Node's truthy `if (req.body.name)`).
	if v := form.String("name"); v != "" {
		patch.Name = &v
	}
	if v := form.String("type"); v != "" {
		patch.Type = &v
	}
	// The rest change whenever the key is PRESENT (Node's `!== undefined`), so an empty submission
	// clears the field (email to NULL).
	if form.Present("email") {
		v := form.String("email")
		patch.EmailSet = true
		if v != "" {
			patch.Email = &v
		}
	}
	if form.Present("phone") {
		v := form.String("phone")
		patch.Phone = &v
	}
	if form.Present("firm") {
		v := form.String("firm")
		patch.Firm = &v
	}
	if form.Present("address") {
		v := form.String("address")
		patch.Address = &v
	}
	if form.Present("gst") {
		v := form.String("gst")
		patch.Gst = &v
	}
	if form.Present("is_active") {
		b := form.String("is_active") == "true"
		patch.IsActive = &b
	}
	if form.Present("permissions") {
		if perms, ok := parsePermissions(form.String("permissions")); ok {
			patch.Permissions = &perms
		}
	}
	if pw := form.String("password"); pw != "" {
		hash, ok := hashPassword(w, pw)
		if !ok {
			return
		}
		patch.PasswordHash = hash
	}

	updated, dupEmail, found, err := h.store.Update(ctx, sess.UID, store.ID(personID), patch)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if dupEmail {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "That email is already registered to a person.", Status: httpx.False()})
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Person not found.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Person updated.", Data: toDTO(updated)})
}

// Delete is POST /person/delete: hard-delete an owner-scoped person.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)
	form, _ := httpx.ReadForm(r)

	personID := form.String("person_id")
	if personID == "" {
		invalid(w)
		return
	}
	found, err := h.store.Delete(ctx, sess.UID, store.ID(personID))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Person not found.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Person deleted.", Status: httpx.True()})
}

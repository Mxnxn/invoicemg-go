// Package person serves POST /person/list from routes/Person.js - employees and suppliers,
// admin-only, scoped to the owner (uid), with an optional `type` filter that powers both the
// People and Suppliers tabs in Configure. Only the list read is ported.
package person

import (
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type Handler struct {
	store store.People
	users store.Users
}

func New(s store.People, users store.Users) *Handler { return &Handler{store: s, users: users} }

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)

	list, err := h.store.List(r.Context(), sess.UID, form.String("type"))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out := make([]personDTO, 0, len(list))
	for _, p := range list {
		perms := p.Permissions
		if perms == nil {
			perms = []string{}
		}
		out = append(out, personDTO{
			ID: string(p.ID), Name: p.Name, Type: p.Type, Email: p.Email, Phone: p.Phone,
			Firm: p.Firm, Address: p.Address, Gst: p.Gst, OpeningBalance: p.OpeningBalance,
			IsActive: p.IsActive, Permissions: perms,
			NotifyPoCreated: p.NotifyPoCreated, NotifyPoUpdated: p.NotifyPoUpdated,
			NotifyPoConfirmed: p.NotifyPoConfirmed, CreatedAt: httpx.NewTime(p.CreatedAt),
		})
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: out})
}

// NotifyPreference is POST /person/notify-preference: set one tri-state supplier-notify flag on
// an owner-scoped person. Admin-only (the router-level gate). The checks run in Node's order:
// missing id/field -> 422 "Invalid request."; an unrecognised field -> 422 "Not a notification
// setting."; a value that is not true/false/clear -> 422 "Invalid request."; a miss -> 404.
func (h *Handler) NotifyPreference(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)
	form, _ := httpx.ReadForm(r)

	personID := form.String("person_id")
	field := form.String("field")
	if personID == "" || field == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	if _, ok := store.NotifyFields[field]; !ok {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Not a notification setting.", Status: httpx.False()})
		return
	}
	var value *bool
	switch form.String("value") {
	case "true":
		t := true
		value = &t
	case "false":
		f := false
		value = &f
	case "clear":
		value = nil // stored null: "follow the default"
	default:
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}

	p, found, err := h.store.SetNotifyField(ctx, sess.UID, store.ID(personID), field, value)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Person not found.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Status: httpx.True(), Data: notifyDTO{
		ID: string(p.ID), Name: p.Name, Type: p.Type, Firm: p.Firm, Phone: p.Phone,
		NotifyPoCreated: p.NotifyPoCreated, NotifyPoUpdated: p.NotifyPoUpdated, NotifyPoConfirmed: p.NotifyPoConfirmed,
	}})
}

// notifyDTO is the .select(["name","type","firm","phone",...notifyFields]) projection Node
// returns from /person/notify-preference: _id plus those, no __v (inclusive projection), and the
// flags omitempty so an unset one is absent rather than null.
type notifyDTO struct {
	ID                string `json:"_id"`
	Name              string `json:"name"`
	Type              string `json:"type"`
	Firm              string `json:"firm"`
	Phone             string `json:"phone"`
	NotifyPoCreated   *bool  `json:"notifyPoCreated,omitempty"`
	NotifyPoUpdated   *bool  `json:"notifyPoUpdated,omitempty"`
	NotifyPoConfirmed *bool  `json:"notifyPoConfirmed,omitempty"`
}

// personDTO is the /person/list projection. The notifyPo* flags use omitempty on a *bool so an
// unset flag is absent on the wire (Model/Person.js gives them no default), matching Node's
// projection which simply has no such key.
type personDTO struct {
	ID                string     `json:"_id"`
	Name              string     `json:"name"`
	Type              string     `json:"type"`
	Email             string     `json:"email"`
	Phone             string     `json:"phone"`
	Firm              string     `json:"firm"`
	Address           string     `json:"address"`
	Gst               string     `json:"gst"`
	OpeningBalance    float64    `json:"openingBalance"`
	IsActive          bool       `json:"is_active"`
	Permissions       []string   `json:"permissions"`
	NotifyPoCreated   *bool      `json:"notifyPoCreated,omitempty"`
	NotifyPoUpdated   *bool      `json:"notifyPoUpdated,omitempty"`
	NotifyPoConfirmed *bool      `json:"notifyPoConfirmed,omitempty"`
	CreatedAt         httpx.Time `json:"createdAt"`
}

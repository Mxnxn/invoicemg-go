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

type Handler struct{ store store.People }

func New(s store.People) *Handler { return &Handler{store: s} }

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

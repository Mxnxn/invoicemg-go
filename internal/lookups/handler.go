// Package lookups serves the lifecycle name/rate pickers from routes/Lifecycle.js -
// /lifecycle/lookups/clients, /materials and /people. They sit BEFORE the lifecycle feature
// gate in Node (TokenHelper only), because the Quotation and Purchase forms need them without
// granting the whole Lifecycle feature; they are still authenticated and company-scoped.
package lookups

import (
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type Handler struct {
	store store.Lookups
	users store.Users
}

func New(l store.Lookups, u store.Users) *Handler { return &Handler{store: l, users: u} }

func (h *Handler) Clients(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	list, err := h.store.Clients(r.Context(), sess.UID, sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out := make([]clientDTO, 0, len(list))
	for _, c := range list {
		out = append(out, clientDTO{ID: string(c.ID), ClientName: c.ClientName, ClientFirm: c.ClientFirm, ClientPhone: c.ClientPhone})
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: out})
}

func (h *Handler) Materials(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	list, err := h.store.Materials(r.Context(), sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out := make([]materialDTO, 0, len(list))
	for _, m := range list {
		out = append(out, materialDTO{ID: string(m.ID), MaterialName: m.MaterialName, MaterialRate: m.MaterialRate, Hsn: m.Hsn, Tax: m.Tax})
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: out})
}

func (h *Handler) People(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)
	list, err := h.store.People(ctx, sess.UID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out := make([]personDTO, 0, len(list)+1)
	// The account owner is prepended as an Admin option (small firms work jobs too), name
	// falling back to email - exactly as routes/Lifecycle.js does.
	if user, err := h.users.FindByID(ctx, sess.UID); err == nil {
		name := user.Name
		if name == "" {
			name = user.Email
		}
		out = append(out, personDTO{ID: string(user.ID), Name: name, Type: "Admin"})
	}
	for _, p := range list {
		out = append(out, personDTO{ID: string(p.ID), Name: p.Name, Type: p.Type})
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: out})
}

type clientDTO struct {
	ID          string `json:"_id"`
	ClientName  string `json:"clientName"`
	ClientFirm  string `json:"clientFirm"`
	ClientPhone string `json:"clientPhone"`
}

type materialDTO struct {
	ID           string  `json:"_id"`
	MaterialName string  `json:"material_name"`
	MaterialRate float64 `json:"material_rate"`
	Hsn          string  `json:"hsn"`
	Tax          float64 `json:"tax"`
}

type personDTO struct {
	ID   string `json:"_id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

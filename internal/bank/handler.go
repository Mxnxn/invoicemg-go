// Package bank serves the bank-account routes from routes/Bank.js. Only /bank/list (a read) is
// ported so far; it sits behind the batch_receive feature gate, as the Node router does.
package bank

import (
	"net/http"
	"strings"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type Handler struct{ store store.Banks }

func New(s store.Banks) *Handler { return &Handler{store: s} }

// List is POST /bank/list: a company's bank accounts in name order. The Node route sends the
// whole record, so this returns _id/uid/company_id/name/openingBalance/createdAt/updatedAt and
// __v (#21), with the dates in Date.toJSON form (#5). Note it sends NO `status` field, matching
// routes/Bank.js exactly.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())

	banks, err := h.store.List(r.Context(), sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}

	out := make([]bankDTO, 0, len(banks))
	for _, b := range banks {
		out = append(out, bankDTO{
			ID:             string(b.ID),
			UID:            string(b.UID),
			CompanyID:      string(b.CompanyID),
			Name:           b.Name,
			OpeningBalance: b.OpeningBalance,
			CreatedAt:      httpx.NewTime(b.CreatedAt),
			UpdatedAt:      httpx.NewTime(b.UpdatedAt),
			Version:        b.Version,
		})
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: out})
}

// bankDTO is the whole record, field names as Model/Bank.js serialises them.
type bankDTO struct {
	ID             string     `json:"_id"`
	UID            string     `json:"uid"`
	CompanyID      string     `json:"company_id"`
	Name           string     `json:"name"`
	OpeningBalance float64    `json:"openingBalance"`
	CreatedAt      httpx.Time `json:"createdAt"`
	UpdatedAt      httpx.Time `json:"updatedAt"`
	Version        int        `json:"__v"`
}

// Create is POST /bank/create: add a bank account for the batch-receive dropdown. The name is
// trimmed and required (blank/whitespace -> 422); the whole stored record comes back, as Node's
// route does. Behind the batch_receive feature.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)
	form, _ := httpx.ReadForm(r)

	name := strings.TrimSpace(form.String("name"))
	if name == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	saved, err := h.store.Create(ctx, sess.CompanyID, sess.UID, name)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Bank created.", Data: bankDTO{
		ID: string(saved.ID), UID: string(saved.UID), CompanyID: string(saved.CompanyID),
		Name: saved.Name, OpeningBalance: saved.OpeningBalance,
		CreatedAt: httpx.NewTime(saved.CreatedAt), UpdatedAt: httpx.NewTime(saved.UpdatedAt), Version: saved.Version,
	}})
}

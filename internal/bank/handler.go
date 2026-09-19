// Package bank serves the bank-account routes from routes/Bank.js. Only /bank/list (a read) is
// ported so far; it sits behind the batch_receive feature gate, as the Node router does.
package bank

import (
	"fmt"
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
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Bank created.", Data: toDTO(saved)})
}

// Update is POST /bank/update: rename a bank and, when the openingBalance field was submitted,
// re-set its balance. A missing id or blank name is 422 "A bank needs a name." (Node's exact
// wording, distinct from create's "Invalid request."); a bank that isn't the company's is 404.
// A malformed id reproduces Node's CastError -> 500 on the document store.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)
	form, _ := httpx.ReadForm(r)

	bankID := form.String("bank_id")
	name := strings.TrimSpace(form.String("name"))
	if bankID == "" || name == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "A bank needs a name.", Status: httpx.False()})
		return
	}
	// Only when the field actually arrived (Node's `!== undefined`), so a name-only edit does
	// not silently zero a balance somebody entered. Number("") is 0 and Number("abc") is 0,
	// which Float(...,0) reproduces.
	var opening *float64
	if form.Present("openingBalance") {
		v := form.Float("openingBalance", 0)
		opening = &v
	}
	updated, found, err := h.store.Update(ctx, sess.CompanyID, store.ID(bankID), name, opening)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Bank not found.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Bank updated.", Status: httpx.True(), Data: toDTO(updated)})
}

// Remove is POST /bank/remove: delete a bank, but only when no receipt, supplier payment or
// expense references it - money already moved through it means the account must stay, and the
// refusal carries the exact transaction count so the UI can explain why. Behind requireDelete.
// Missing id -> 422; in use -> 422 with the count; not the company's -> 404.
func (h *Handler) Remove(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)
	form, _ := httpx.ReadForm(r)

	bankID := form.String("bank_id")
	if bankID == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	inUse, found, err := h.store.Remove(ctx, sess.CompanyID, store.ID(bankID))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if inUse > 0 {
		plural := "s"
		if inUse == 1 {
			plural = ""
		}
		httpx.Write(w, httpx.Envelope{
			Code:    422,
			Status:  httpx.False(),
			Message: fmt.Sprintf("This bank has %d transaction%s against it. Money already moved through it, so it can't be removed.", inUse, plural),
		})
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Bank not found.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Bank removed.", Status: httpx.True()})
}

// toDTO renders a stored bank as the whole record Node's routes echo.
func toDTO(b store.Bank) bankDTO {
	return bankDTO{
		ID: string(b.ID), UID: string(b.UID), CompanyID: string(b.CompanyID),
		Name: b.Name, OpeningBalance: b.OpeningBalance,
		CreatedAt: httpx.NewTime(b.CreatedAt), UpdatedAt: httpx.NewTime(b.UpdatedAt), Version: b.Version,
	}
}

// Package expense serves POST /expense/list from routes/Expense.js - a company's expenses,
// newest first, with bank_id populated and optional bank_id / from / to filters. Behind the
// batch_receive feature. Only the list read is ported.
package expense

import (
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type Handler struct{ store store.Expenses }

func New(s store.Expenses) *Handler { return &Handler{store: s} }

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)

	list, err := h.store.List(r.Context(), sess.CompanyID, store.ExpenseFilter{
		BankID: store.ID(form.String("bank_id")), From: form.String("from"), To: form.String("to"),
	})
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out := make([]expenseDTO, 0, len(list))
	for _, e := range list {
		dto := expenseDTO{
			ID: string(e.ID), UID: string(e.UID), CompanyID: idPtr(e.CompanyID),
			Date: e.Date, Amount: e.Amount, Notes: e.Notes,
			CreatedAt: httpx.NewTime(e.CreatedAt), UpdatedAt: httpx.NewTime(e.UpdatedAt), Version: e.Version,
		}
		if e.Bank != nil {
			dto.BankID = &bankDTO{ID: string(e.Bank.ID), Name: e.Bank.Name}
		}
		out = append(out, dto)
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: out})
}

func idPtr(id store.ID) *string {
	if id == "" {
		return nil
	}
	s := string(id)
	return &s
}

type expenseDTO struct {
	ID        string     `json:"_id"`
	UID       string     `json:"uid"`
	CompanyID *string    `json:"company_id"`
	BankID    *bankDTO   `json:"bank_id"`
	Date      string     `json:"date"`
	Amount    float64    `json:"amount"`
	Notes     string     `json:"notes"`
	CreatedAt httpx.Time `json:"createdAt"`
	UpdatedAt httpx.Time `json:"updatedAt"`
	Version   int        `json:"__v"`
}

type bankDTO struct {
	ID   string `json:"_id"`
	Name string `json:"name"`
}

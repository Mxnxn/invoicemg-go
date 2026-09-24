package expense

import (
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

// Create is POST /expense/create (requireCreate "batch_receive"): record a bank expense. Bank,
// date and a positive, finite amount are required (Node's Number()/isFinite/>0 guard); the saved
// row is echoed with its bank populated.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)

	bankID := form.String("bank_id")
	date := form.String("date")
	// Float(...,0) coerces like Number(): a blank or non-numeric amount becomes 0 and is rejected
	// by the >0 check, exactly as Node's !Number.isFinite || <=0 does.
	amount := form.Float("amount", 0)
	if bankID == "" || date == "" || amount <= 0 {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Bank, date and a positive amount are required.", Status: httpx.False()})
		return
	}

	saved, err := h.store.Create(r.Context(), sess.CompanyID, sess.UID, store.ExpenseWrite{
		BankID: store.ID(bankID), Date: date, Amount: amount, Notes: form.String("notes"),
	})
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Expense recorded.", Status: httpx.True(), Data: toExpenseDTO(saved)})
}

// Remove is POST /expense/remove (requireDelete "batch_receive"): company-scoped hard delete.
func (h *Handler) Remove(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)

	id := form.String("expense_id")
	if id == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	found, err := h.store.Delete(r.Context(), sess.CompanyID, store.ID(id))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "No such expense.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Expense removed.", Status: httpx.True()})
}

// toExpenseDTO is the shared list/create row shape (bank_id populated, __v included).
func toExpenseDTO(e store.Expense) expenseDTO {
	dto := expenseDTO{
		ID: string(e.ID), UID: string(e.UID), CompanyID: idPtr(e.CompanyID),
		Date: e.Date, Amount: e.Amount, Notes: e.Notes,
		CreatedAt: httpx.NewTime(e.CreatedAt), UpdatedAt: httpx.NewTime(e.UpdatedAt), Version: e.Version,
	}
	if e.Bank != nil {
		dto.BankID = &bankDTO{ID: string(e.Bank.ID), Name: e.Bank.Name}
	}
	return dto
}

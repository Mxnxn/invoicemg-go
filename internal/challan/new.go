package challan

import (
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

// New is POST /challan/new: record a cash delivery challan.
//
// This route is faithfully quirky. Node validates companyName/amount/qty/date/description and
// answers 422 *with status:true* on a miss (a bug, preserved). With no `type` it saves an
// "In CASH" challan and returns 200. With a `type` present Node's else-branch is dead code -
// `const Challan = new Challan(...)` is a temporal-dead-zone ReferenceError - so it always
// throws and answers 500; that 500 is reproduced rather than quietly "fixed", because a client
// that learned to expect it must keep seeing it during the sideways phase.
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	if !form.Has("companyName") || !form.Has("amount") || !form.Has("qty") || !form.Has("date") || !form.Has("description") {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request", Status: httpx.True()})
		return
	}
	// Node's type-present branch is an unconditional TDZ crash -> 500.
	if form.Has("type") {
		httpx.Internal(w, nil)
		return
	}
	ch, err := h.store.Create(r.Context(), sess.CompanyID, sess.UID, store.ChallanWrite{
		CompanyName: form.String("companyName"),
		Description: form.String("description"),
		Date:        form.String("date"),
		Type:        "In CASH",
		Quantity:    form.Float("qty", 0),
		Amount:      form.Float("amount", 0),
	})
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Status: httpx.True(), Data: challanDTO{
		ID: string(ch.ID), UID: string(ch.UID), CompanyID: idPtr(ch.CompanyID),
		CompanyName: ch.CompanyName, Description: ch.Description, Date: ch.Date, Type: ch.Type,
		Quantity: ch.Quantity, Amount: ch.Amount, Version: ch.Version,
	}})
}

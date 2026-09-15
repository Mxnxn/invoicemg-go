package wastage

import (
	"net/http"
	"strconv"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

// Add is POST /wastage/add: log an offcut/scrap entry (company-scoped, owned by the acting user).
// material_name, rate, length, height, total and date are required (truthy); purchase_rate and
// cost_total default to 0. The whole stored record comes back, as routes/Wastage.js sends it.
func (h *Handler) Add(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)
	form, _ := httpx.ReadForm(r)

	if !form.Has("material_name") || !form.Has("rate") || !form.Has("length") ||
		!form.Has("height") || !form.Has("total") || !form.Has("date") {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	in := store.WastageWrite{
		MaterialName: form.String("material_name"),
		Rate:         num(form.String("rate")),
		PurchaseRate: num(form.String("purchase_rate")),
		CostTotal:    num(form.String("cost_total")),
		Length:       num(form.String("length")),
		Height:       num(form.String("height")),
		Total:        num(form.String("total")),
		Date:         form.String("date"),
	}
	saved, err := h.store.Create(ctx, sess.CompanyID, sess.UID, in)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Wastage has added.", Data: dto{
		ID: string(saved.ID), UID: string(saved.UID), CompanyID: idPtr(saved.CompanyID), MaterialName: saved.MaterialName,
		Rate: saved.Rate, PurchaseRate: saved.PurchaseRate, CostTotal: saved.CostTotal, Length: saved.Length,
		Height: saved.Height, Total: saved.Total, Date: saved.Date,
		CreatedAt: httpx.NewTime(saved.CreatedAt), UpdatedAt: httpx.NewTime(saved.UpdatedAt), Version: saved.Version,
	}})
}

// num is Mongoose's Number cast of a submitted string: an unparseable or absent value is 0.
func num(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

// Package inventory serves POST /inventory/report from routes/Inventory.js - the stock report.
// Behind the products feature. Scoped to the acting company (report sharing across an owner's
// companies is a Postgres follow-up; the Node default also fails closed to the acting company).
package inventory

import (
	"encoding/json"
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/inventorymath"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type Handler struct {
	store     store.Inventory
	companies store.Companies
}

func New(s store.Inventory, c store.Companies) *Handler { return &Handler{store: s, companies: c} }

func (h *Handler) Report(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)
	form, _ := httpx.ReadForm(r)

	data, err := h.store.Data(ctx, sess.CompanyID, form.String("from"), form.String("to"))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	// the acting company's own label, so shared rows can name it (single company here).
	labels := map[string]string{}
	if c, err := h.companies.Active(ctx, sess.CompanyID, sess.UID); err == nil {
		name := c.Firm
		if name == "" {
			name = c.Name
		}
		labels[string(sess.CompanyID)] = name
	}

	in := inventorymath.Input{Labels: labels}
	for _, m := range data.Materials {
		in.Materials = append(in.Materials, inventorymath.Material{ID: m.ID, MaterialName: m.MaterialName, Unit: m.Unit, PurchaseRate: m.PurchaseRate})
	}
	for _, p := range data.PurchaseRows {
		in.PurchaseRows = append(in.PurchaseRows, inventorymath.PurchaseRow{ID: p.ID, Material: p.Material, Qty: p.Qty, Rate: p.Rate, CompanyID: p.CompanyID})
	}
	for _, j := range data.JobRows {
		in.JobRows = append(in.JobRows, inventorymath.JobRow{ID: j.ID, Material: j.Material, Length: j.Length, Width: j.Width, Qty: j.Qty, HasDimensions: j.HasDimensions, CompanyID: j.CompanyID})
	}
	for _, wr := range data.WastageRows {
		in.WastageRows = append(in.WastageRows, inventorymath.WastageRow{ID: wr.ID, MaterialName: wr.MaterialName, Length: wr.Length, Height: wr.Height, CompanyID: wr.CompanyID})
	}
	rows, notCounted := inventorymath.Build(in)

	hasRange := form.Has("from") || form.Has("to")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code": 200, "message": "Operation successful.", "status": true,
		"data": map[string]any{
			"rows": rows, "notCounted": notCounted, "shared": false, "companyCount": 1, "wastageIgnoresRange": hasRange,
		},
	})
}

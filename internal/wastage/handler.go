// Package wastage serves POST /wastage/getall from routes/Wastage.js - a company's wastage log,
// newest first. Behind the challan feature. Only the list read is ported.
package wastage

import (
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type Handler struct{ store store.Wastages }

func New(s store.Wastages) *Handler { return &Handler{store: s} }

func (h *Handler) GetAll(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	list, err := h.store.List(r.Context(), sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out := make([]dto, 0, len(list))
	for _, x := range list {
		out = append(out, dto{
			ID: string(x.ID), UID: string(x.UID), CompanyID: idPtr(x.CompanyID), MaterialName: x.MaterialName,
			Rate: x.Rate, PurchaseRate: x.PurchaseRate, CostTotal: x.CostTotal, Length: x.Length, Height: x.Height,
			Total: x.Total, Date: x.Date, CreatedAt: httpx.NewTime(x.CreatedAt), UpdatedAt: httpx.NewTime(x.UpdatedAt), Version: x.Version,
		})
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

type dto struct {
	ID           string     `json:"_id"`
	UID          string     `json:"uid"`
	CompanyID    *string    `json:"company_id"`
	MaterialName string     `json:"material_name"`
	Rate         float64    `json:"rate"`
	PurchaseRate float64    `json:"purchase_rate"`
	CostTotal    float64    `json:"cost_total"`
	Length       float64    `json:"length"`
	Height       float64    `json:"height"`
	Total        float64    `json:"total"`
	Date         string     `json:"date"`
	CreatedAt    httpx.Time `json:"createdAt"`
	UpdatedAt    httpx.Time `json:"updatedAt"`
	Version      int        `json:"__v"`
}

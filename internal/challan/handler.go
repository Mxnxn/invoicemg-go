// Package challan serves POST /challan/getAll from routes/Challan.js - a company's delivery
// challans. Behind the challan feature. The response is {code, data, status} with NO message,
// matching the Node route exactly. Only the list read is ported.
package challan

import (
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type Handler struct{ store store.Challans }

func New(s store.Challans) *Handler { return &Handler{store: s} }

func (h *Handler) GetAll(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	list, err := h.store.List(r.Context(), sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out := make([]challanDTO, 0, len(list))
	for _, c := range list {
		out = append(out, challanDTO{
			ID: string(c.ID), UID: string(c.UID), CompanyID: idPtr(c.CompanyID),
			CompanyName: c.CompanyName, Description: c.Description, Date: c.Date, Type: c.Type,
			Quantity: c.Quantity, Amount: c.Amount, Version: c.Version,
		})
	}
	// No message, status:true - the Node route sends exactly {code, data, status}.
	httpx.Write(w, httpx.Envelope{Code: 200, Status: httpx.True(), Data: out})
}

func idPtr(id store.ID) *string {
	if id == "" {
		return nil
	}
	s := string(id)
	return &s
}

type challanDTO struct {
	ID          string  `json:"_id"`
	UID         string  `json:"uid"`
	CompanyID   *string `json:"company_id"`
	CompanyName string  `json:"companyName"`
	Description string  `json:"description"`
	Date        string  `json:"date"`
	Type        string  `json:"type"`
	Quantity    float64 `json:"quantity"`
	Amount      float64 `json:"amount"`
	Version     int     `json:"__v"`
}

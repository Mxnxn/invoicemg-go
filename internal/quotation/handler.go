// Package quotation serves POST /quotation/list from routes/Quotation.js - a company's
// quotations, newest first, with client_id populated into a nested client object and an optional
// client_id filter. Behind the quotations feature. Only the list read is ported.
package quotation

import (
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type Handler struct{ store store.Quotations }

func New(s store.Quotations) *Handler { return &Handler{store: s} }

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)

	list, err := h.store.List(r.Context(), sess.UID, sess.CompanyID, store.ID(form.String("client_id")))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out := make([]quotationDTO, 0, len(list))
	for _, q := range list {
		out = append(out, toDTO(q))
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: out})
}

func toDTO(q store.Quotation) quotationDTO {
	rows := make([]rowDTO, 0, len(q.Rows))
	for _, r := range q.Rows {
		hasDim := true
		if r.HasDimensions != nil {
			hasDim = *r.HasDimensions
		}
		rows = append(rows, rowDTO{
			ID: string(r.ID), Material: r.Material, Description: r.Description,
			HasDimensions: hasDim, Length: r.Length, Width: r.Width, Qty: r.Qty, Rate: r.Rate,
			Cgst: r.Cgst, Sgst: r.Sgst, Igst: r.Igst, Discount: r.Discount, Charges: r.Charges,
			JobID:     idPtr(r.JobID),
			CreatedAt: httpx.NewTime(r.CreatedAt), UpdatedAt: httpx.NewTime(r.UpdatedAt),
		})
	}
	dto := quotationDTO{
		ID: string(q.ID), UID: string(q.UID), CompanyID: idPtr(q.CompanyID),
		QuotationNumber: q.QuotationNumber, Date: q.Date, Rows: rows,
		CreatedAt: httpx.NewTime(q.CreatedAt), UpdatedAt: httpx.NewTime(q.UpdatedAt), Version: q.Version,
	}
	// client_id is the POPULATED object, or null when the client was deleted (Mongoose populate).
	if q.Client != nil {
		dto.ClientID = &clientDTO{
			ID: string(q.Client.ID), ClientName: q.Client.ClientName, ClientFirm: q.Client.ClientFirm,
			ClientPhone: q.Client.ClientPhone, ClientAddress: q.Client.ClientAddress, ClientGST: q.Client.ClientGST,
		}
	}
	return dto
}

func idPtr(id store.ID) *string {
	if id == "" {
		return nil
	}
	s := string(id)
	return &s
}

type quotationDTO struct {
	ID        string  `json:"_id"`
	UID       string  `json:"uid"`
	CompanyID *string `json:"company_id"`
	// The populated client object under "client_id", as Mongoose .populate replaces it; null
	// when the client was deleted.
	ClientID        *clientDTO `json:"client_id"`
	QuotationNumber string     `json:"quotationNumber"`
	Date            string     `json:"date"`
	Rows            []rowDTO   `json:"rows"`
	CreatedAt       httpx.Time `json:"createdAt"`
	UpdatedAt       httpx.Time `json:"updatedAt"`
	Version         int        `json:"__v"`
}

type clientDTO struct {
	ID            string `json:"_id"`
	ClientName    string `json:"clientName"`
	ClientFirm    string `json:"clientFirm"`
	ClientPhone   string `json:"clientPhone"`
	ClientAddress string `json:"clientAddress"`
	ClientGST     string `json:"clientGST"`
}

type rowDTO struct {
	ID            string     `json:"_id"`
	Material      string     `json:"material"`
	Description   string     `json:"description"`
	HasDimensions bool       `json:"hasDimensions"`
	Length        string     `json:"length"`
	Width         string     `json:"width"`
	Qty           float64    `json:"qty"`
	Rate          float64    `json:"rate"`
	Cgst          float64    `json:"cgst"`
	Sgst          float64    `json:"sgst"`
	Igst          float64    `json:"igst"`
	Discount      float64    `json:"discount"`
	Charges       float64    `json:"charges"`
	JobID         *string    `json:"job_id"`
	CreatedAt     httpx.Time `json:"createdAt"`
	UpdatedAt     httpx.Time `json:"updatedAt"`
}

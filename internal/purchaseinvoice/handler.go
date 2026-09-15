// Package purchaseinvoice serves POST /purchase-invoice/list from routes/PurchaseInvoice.js -
// supplier bills, newest first, with supplier_id populated into a nested object. Behind the
// purchase_invoices feature. Only the list read is ported.
package purchaseinvoice

import (
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type Handler struct{ store store.PurchaseInvoices }

func New(s store.PurchaseInvoices) *Handler { return &Handler{store: s} }

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	list, err := h.store.List(r.Context(), sess.UID, sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out := make([]invoiceDTO, 0, len(list))
	for _, inv := range list {
		out = append(out, toDTO(inv))
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: out})
}

func toDTO(inv store.PurchaseInvoice) invoiceDTO {
	rows := make([]rowDTO, 0, len(inv.Rows))
	for _, r := range inv.Rows {
		// Purchase rows default to false (by quantity) when the flag is absent.
		hasDim := false
		if r.HasDimensions != nil {
			hasDim = *r.HasDimensions
		}
		rows = append(rows, rowDTO{
			ID: string(r.ID), Description: r.Description, Material: r.Material, Hsn: r.Hsn, Gst: r.Gst,
			HasDimensions: hasDim, Length: r.Length, Width: r.Width, Rate: r.Rate, Qty: r.Qty,
			Unit: r.Unit, Discount: r.Discount, Charges: r.Charges,
			CreatedAt: httpx.NewTime(r.CreatedAt), UpdatedAt: httpx.NewTime(r.UpdatedAt),
		})
	}
	dto := invoiceDTO{
		ID: string(inv.ID), UID: string(inv.UID), CompanyID: idPtr(inv.CompanyID),
		Date: inv.Date, InvoiceNumber: inv.InvoiceNumber, Rows: rows, Total: inv.Total, Amount: inv.Amount,
		CreatedAt: httpx.NewTime(inv.CreatedAt), UpdatedAt: httpx.NewTime(inv.UpdatedAt), Version: inv.Version,
	}
	// supplier_id is the POPULATED object, or null for a dangling ref.
	if inv.Supplier != nil {
		dto.SupplierID = &supplierDTO{
			ID: string(inv.Supplier.ID), Name: inv.Supplier.Name, Firm: inv.Supplier.Firm, Phone: inv.Supplier.Phone,
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

type invoiceDTO struct {
	ID            string       `json:"_id"`
	UID           string       `json:"uid"`
	CompanyID     *string      `json:"company_id"`
	SupplierID    *supplierDTO `json:"supplier_id"`
	Date          string       `json:"date"`
	InvoiceNumber string       `json:"invoiceNumber"`
	Rows          []rowDTO     `json:"rows"`
	Total         float64      `json:"total"`
	Amount        float64      `json:"amount"`
	CreatedAt     httpx.Time   `json:"createdAt"`
	UpdatedAt     httpx.Time   `json:"updatedAt"`
	Version       int          `json:"__v"`
}

type supplierDTO struct {
	ID    string `json:"_id"`
	Name  string `json:"name"`
	Firm  string `json:"firm"`
	Phone string `json:"phone"`
}

type rowDTO struct {
	ID            string     `json:"_id"`
	Description   string     `json:"description"`
	Material      string     `json:"material"`
	Hsn           string     `json:"hsn"`
	Gst           float64    `json:"gst"`
	HasDimensions bool       `json:"hasDimensions"`
	Length        string     `json:"length"`
	Width         string     `json:"width"`
	Rate          float64    `json:"rate"`
	Qty           float64    `json:"qty"`
	Unit          string     `json:"unit"`
	Discount      float64    `json:"discount"`
	Charges       float64    `json:"charges"`
	CreatedAt     httpx.Time `json:"createdAt"`
	UpdatedAt     httpx.Time `json:"updatedAt"`
}

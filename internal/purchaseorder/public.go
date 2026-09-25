package purchaseorder

import (
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type publicRowDTO struct {
	Description   string  `json:"description"`
	Material      string  `json:"material"`
	Hsn           string  `json:"hsn"`
	Gst           float64 `json:"gst"`
	HasDimensions bool    `json:"hasDimensions"`
	Length        string  `json:"length"`
	Width         string  `json:"width"`
	Qty           float64 `json:"qty"`
	Rate          float64 `json:"rate"`
	Unit          string  `json:"unit"`
	Discount      float64 `json:"discount"`
	Charges       float64 `json:"charges"`
}

// PublicView is GET /po-public/{supplier_id}/{po_id} - the unauthenticated supplier link. Both ids
// must match the same order; a mismatch (or malformed id) is a 404. A converted order's link is
// spent and returns only {closed:true}. The payload is ONE order and nothing else (Helpers/PoPublic).
func (h *Handler) PublicView(w http.ResponseWriter, r *http.Request) {
	supplierID := r.PathValue("supplier_id")
	poID := r.PathValue("po_id")
	view, found, err := h.store.PublicView(r.Context(), store.ID(supplierID), store.ID(poID))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Purchase order not found.", Status: httpx.False()})
		return
	}
	if view.Closed {
		httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Status: httpx.True(), Data: map[string]any{"closed": true}})
		return
	}

	rows := make([]publicRowDTO, 0, len(view.Rows))
	for _, r := range view.Rows {
		length, width := r.Length, r.Width
		if length == "" {
			length = "1"
		}
		if width == "" {
			width = "1"
		}
		rows = append(rows, publicRowDTO{
			Description: r.Description, Material: r.Material, Hsn: r.Hsn, Gst: r.Gst, HasDimensions: r.HasDimensions,
			Length: length, Width: width, Qty: r.Qty, Rate: r.Rate, Unit: r.Unit, Discount: r.Discount, Charges: r.Charges,
		})
	}

	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Status: httpx.True(), Data: map[string]any{
		"closed": false,
		"order": map[string]any{
			"poNumber": view.PoNumber, "date": view.Date, "total": view.Total,
			"supplier": map[string]any{"name": view.SupplierName, "firm": view.SupplierFirm},
			"rows":     rows,
		},
		"company": map[string]any{
			"firm": view.Company.Firm, "address": view.Company.Address, "phone": view.Company.Phone,
			"gst": view.Company.Gst, "url": view.Company.URL, "documentFont": view.Company.DocumentFont,
			// Not stored in this port yet; the Mongoose schema defaults (same as /company/active).
			"exportTemplate": map[string]any{"logo": true, "firm": true, "address": true, "phone": true, "gst": true, "showPeriod": true},
		},
	}})
}

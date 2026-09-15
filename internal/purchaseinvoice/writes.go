package purchaseinvoice

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

func invalid(w http.ResponseWriter, msg string) {
	httpx.Write(w, httpx.Envelope{Code: 422, Message: msg, Status: httpx.False()})
}

// rawPurchaseRow is one submitted row before coercion. Numeric fields accept a string or number.
type rawPurchaseRow struct {
	Description string
	Material    string
	Hsn         string
	Unit        string
	Gst         float64
	Rate        float64
	Qty         float64
	Discount    float64
	Charges     float64
}

func (r *rawPurchaseRow) UnmarshalJSON(b []byte) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	str := func(k string) string {
		if raw, ok := m[k]; ok {
			var s string
			if json.Unmarshal(raw, &s) == nil {
				return s
			}
		}
		return ""
	}
	numOf := func(k string) float64 {
		raw, ok := m[k]
		if !ok {
			return 0
		}
		var f float64
		if json.Unmarshal(raw, &f) == nil {
			return f
		}
		var s string
		if json.Unmarshal(raw, &s) == nil {
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				return f
			}
		}
		return 0
	}
	r.Description, r.Material, r.Hsn, r.Unit = str("description"), str("material"), str("hsn"), str("unit")
	r.Gst, r.Rate, r.Qty, r.Discount, r.Charges = numOf("gst"), numOf("rate"), numOf("qty"), numOf("discount"), numOf("charges")
	return nil
}

// rowTotal is routes/PurchaseInvoice.rowTotal: (qty*rate - discount + charges) * (1 + gst/100).
// Purchase rows are qty*rate (no area dimensions), net of discount/charges, inclusive of GST.
func rowTotal(r store.PurchaseRowInput) float64 {
	net := r.Qty*r.Rate - r.Discount + r.Charges
	return net * (1 + r.Gst/100)
}

func rowsTotal(rows []store.PurchaseRowInput) float64 {
	var sum float64
	for _, r := range rows {
		sum += rowTotal(r)
	}
	return sum
}

// parseRows returns the coerced rows, ok=false if the JSON is not an array. empty reports whether
// the array had no elements (a purchase invoice needs at least one row).
func parseRows(raw string) (rows []store.PurchaseRowInput, ok, empty bool) {
	var parsed []rawPurchaseRow
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, false, false
	}
	out := make([]store.PurchaseRowInput, 0, len(parsed))
	for _, r := range parsed {
		out = append(out, store.PurchaseRowInput{
			Description: r.Description, Material: r.Material, Hsn: r.Hsn, Gst: r.Gst,
			Rate: r.Rate, Qty: r.Qty, Unit: r.Unit, Discount: r.Discount, Charges: r.Charges,
		})
	}
	return out, true, len(out) == 0
}

// Create is POST /purchase-invoice/create.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	if !form.Has("supplier_id") || !form.Has("date") || !form.Has("invoiceNumber") || !form.Has("rows") {
		invalid(w, "Invalid request.")
		return
	}
	rows, ok, empty := parseRows(form.String("rows"))
	if !ok {
		invalid(w, "rows must be a JSON array.")
		return
	}
	if empty {
		invalid(w, "A purchase invoice needs at least one row.")
		return
	}
	inv, err := h.store.Create(r.Context(), sess.UID, sess.CompanyID, store.PurchaseInvoiceWrite{
		SupplierID: store.ID(form.String("supplier_id")), Date: form.String("date"),
		InvoiceNumber: form.String("invoiceNumber"), Rows: rows, Total: rowsTotal(rows),
	})
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Purchase invoice created.", Data: toDTO(inv)})
}

// Update is POST /purchase-invoice/update: a partial edit; changing rows recomputes the total and
// refuses if it would fall below what is already paid.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	id := form.String("purchase_invoice_id")
	if id == "" {
		invalid(w, "Invalid request.")
		return
	}
	var patch store.PurchaseInvoiceUpdate
	if form.Has("supplier_id") {
		s := store.ID(form.String("supplier_id"))
		patch.SupplierID = &s
	}
	if form.Has("date") {
		v := form.String("date")
		patch.Date = &v
	}
	if form.Has("invoiceNumber") {
		v := form.String("invoiceNumber")
		patch.InvoiceNumber = &v
	}
	if form.Present("rows") {
		rows, ok, empty := parseRows(form.String("rows"))
		if !ok {
			invalid(w, "rows must be a JSON array.")
			return
		}
		if empty {
			invalid(w, "A purchase invoice needs at least one row.")
			return
		}
		patch.Rows = &rows
		patch.NewTotal = rowsTotal(rows)
	}
	inv, res, err := h.store.Update(r.Context(), sess.UID, sess.CompanyID, store.ID(id), patch)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	switch res.Status {
	case store.PurchaseUpdateNotFound:
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Purchase invoice not found.", Status: httpx.False()})
	case store.PurchaseUpdatePaidExceeds:
		invalid(w, fmt.Sprintf("This invoice already has ₹%s paid against it - delete that payment before reducing the total to ₹%s.",
			money(res.AmountPaid), money(res.NewTotal)))
	default:
		httpx.Write(w, httpx.Envelope{Code: 200, Message: "Purchase invoice updated.", Data: toDTO(inv)})
	}
}

// money is JS Number.toFixed(2) for the refusal message.
func money(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

// Delete is POST /purchase-invoice/delete: blocked while a supplier payment allocates to it.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	id := form.String("purchase_invoice_id")
	if id == "" {
		invalid(w, "Invalid request.")
		return
	}
	res, err := h.store.Delete(r.Context(), sess.UID, sess.CompanyID, store.ID(id))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	switch res {
	case store.PurchaseDeleteHasPayment:
		invalid(w, "A recorded payment is allocated to this invoice - delete the payment first (Bank Transfers > Paid).")
	case store.PurchaseDeleteNotFound:
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Purchase invoice not found.", Status: httpx.False()})
	default:
		httpx.Write(w, httpx.Envelope{Code: 200, Message: "Purchase invoice deleted.", Status: httpx.True()})
	}
}

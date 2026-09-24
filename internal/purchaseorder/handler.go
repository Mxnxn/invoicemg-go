// Package purchaseorder serves routes/PurchaseOrder.js. The number is generated (PO/<FY>/NNNNNN),
// the approval fingerprint (internal/pochanges) gates edits, and converting one mints a purchase
// invoice. Behind the purchase_orders feature. Send routes (/share, /confirm) are Meta-WhatsApp and
// not ported here.
package purchaseorder

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/pochanges"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type Handler struct{ store store.PurchaseOrders }

func New(s store.PurchaseOrders) *Handler { return &Handler{store: s} }

var poClock = func() time.Time { return time.Now().UTC() }

func actorOf(r *http.Request) store.NoteActor {
	sess := auth.MustFrom(r.Context())
	return store.NoteActor{Role: sess.Role, UID: sess.UID, PersonID: sess.PersonID}
}

// rawPORow is one submitted row before coercion; numeric fields accept a string or a number.
type rawPORow struct {
	Description, Material, Hsn, Unit, Length, Width string
	Gst, Rate, Qty, Discount, Charges              float64
	HasDimensions                                  bool
}

func (r *rawPORow) UnmarshalJSON(b []byte) error {
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
	boolOf := func(k string) bool {
		raw, ok := m[k]
		if !ok {
			return false
		}
		var b bool
		if json.Unmarshal(raw, &b) == nil {
			return b
		}
		var s string
		if json.Unmarshal(raw, &s) == nil {
			return s == "true"
		}
		return false
	}
	r.Description, r.Material, r.Hsn, r.Unit = str("description"), str("material"), str("hsn"), str("unit")
	r.Length, r.Width = str("length"), str("width")
	r.Gst, r.Rate, r.Qty, r.Discount, r.Charges = numOf("gst"), numOf("rate"), numOf("qty"), numOf("discount"), numOf("charges")
	r.HasDimensions = boolOf("hasDimensions")
	return nil
}

// parseRows coerces the `rows` form field (a JSON array) into store rows; a non-array is treated as
// no rows (Node's parseRows returns [] on parse failure).
func parseRows(raw string) []store.PORow {
	out := []store.PORow{}
	var parsed []rawPORow
	if json.Unmarshal([]byte(raw), &parsed) != nil {
		return out
	}
	for _, r := range parsed {
		out = append(out, store.PORow{
			Description: r.Description, Material: r.Material, Hsn: r.Hsn, Unit: r.Unit,
			Gst: r.Gst, HasDimensions: r.HasDimensions, Length: r.Length, Width: r.Width,
			Rate: r.Rate, Qty: r.Qty, Discount: r.Discount, Charges: r.Charges,
		})
	}
	return out
}

// dimFactor is Helpers/RowPricing.dimensionFactor for a purchase row: 1 by quantity, else L*W.
func dimFactor(r store.PORow) float64 {
	if !r.HasDimensions {
		return 1
	}
	l, w := 1.0, 1.0
	if r.Length != "" {
		if v, err := strconv.ParseFloat(r.Length, 64); err == nil {
			l = v
		}
	}
	if r.Width != "" {
		if v, err := strconv.ParseFloat(r.Width, 64); err == nil {
			w = v
		}
	}
	return l * w
}

// computeTotal is Helpers/PurchaseRowTotal.purchaseRowsTotal (shared with PurchaseInvoice on convert).
func computeTotal(rows []store.PORow) float64 {
	var sum float64
	for _, r := range rows {
		net := r.Qty*dimFactor(r)*r.Rate - r.Discount + r.Charges
		sum += net * (1 + r.Gst/100)
	}
	return sum
}

// toFingerprintPO builds the pochanges view of a PO from its store fields.
func toFingerprintPO(supplierID store.ID, date string, total float64, rows []store.PORow) pochanges.PO {
	po := pochanges.PO{SupplierID: string(supplierID), Date: date, Total: total}
	for _, r := range rows {
		po.Rows = append(po.Rows, pochanges.Row{
			Material: r.Material, Description: r.Description, Unit: r.Unit, Hsn: r.Hsn,
			Qty: r.Qty, Rate: r.Rate, Discount: r.Discount, Charges: r.Charges, Gst: r.Gst,
		})
	}
	return po
}

func toStoreChanges(cs []pochanges.Change) []store.Change {
	out := make([]store.Change, 0, len(cs))
	for _, c := range cs {
		out = append(out, store.Change{Field: c.Field, From: c.From, To: c.To})
	}
	return out
}

package quotation

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/docnumber"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

// now is injectable so the FY the next-number helper picks is deterministic in tests.
var now = func() time.Time { return time.Now().UTC() }

func invalid(w http.ResponseWriter, msg string) {
	httpx.Write(w, httpx.Envelope{Code: 422, Message: msg, Status: httpx.False()})
}

// rawRow mirrors one element of the submitted rows JSON, before coercion.
type rawRow struct {
	ID          string  `json:"_id"`
	Material    string  `json:"material"`
	Description string  `json:"description"`
	Length      string  `json:"length"`
	Width       string  `json:"width"`
	Qty         float64 `json:"qty"`
	Rate        float64 `json:"rate"`
	Cgst        float64 `json:"cgst"`
	Sgst        float64 `json:"sgst"`
	Discount    float64 `json:"discount"`
	Charges     float64 `json:"charges"`
}

// UnmarshalJSON coerces the numeric fields the way Node's parseRows does (Number(x) || 0), so a
// string "5", a number 5, or a missing field all behave as they do there.
func (r *rawRow) UnmarshalJSON(b []byte) error {
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
	r.ID = str("_id")
	r.Material = str("material")
	r.Description = str("description")
	r.Length = str("length")
	r.Width = str("width")
	r.Qty = numOf("qty")
	r.Rate = numOf("rate")
	r.Cgst = numOf("cgst")
	r.Sgst = numOf("sgst")
	r.Discount = numOf("discount")
	r.Charges = numOf("charges")
	return nil
}

// parseRows is Helpers parseRows: JSON array or nil (ok=false). length/width fall back to "1".
func parseRows(raw string) ([]store.QuotationRowInput, bool) {
	var rows []rawRow
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		return nil, false
	}
	out := make([]store.QuotationRowInput, 0, len(rows))
	for _, r := range rows {
		length := r.Length
		if length == "" {
			length = "1"
		}
		width := r.Width
		if width == "" {
			width = "1"
		}
		out = append(out, store.QuotationRowInput{
			ID: store.ID(r.ID), Material: r.Material, Description: r.Description,
			Length: length, Width: width, Qty: r.Qty, Rate: r.Rate,
			Cgst: r.Cgst, Sgst: r.Sgst, Discount: r.Discount, Charges: r.Charges,
		})
	}
	return out, true
}

// NextNumber is POST /quotation/next-quotation-number: the next free QT- number for this FY.
func (h *Handler) NextNumber(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	numbers, err := h.store.Numbers(r.Context(), sess.UID, sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	next := docnumber.Next(numbers, "QT-", now())
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: map[string]any{"quotationNumber": next}})
}

// Get is POST /quotation/get: one quotation, populated, owner+company scoped.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	id := form.String("quotation_id")
	if id == "" {
		invalid(w, "Invalid request.")
		return
	}
	q, found, err := h.store.Get(r.Context(), sess.UID, sess.CompanyID, store.ID(id))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Quotation not found.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: toDTO(q)})
}

// Create is POST /quotation/create.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	if !form.Has("client_id") || !form.Has("date") || !form.Has("quotationNumber") || !form.Has("rows") {
		invalid(w, "Invalid request.")
		return
	}
	rows, ok := parseRows(form.String("rows"))
	if !ok {
		invalid(w, "rows must be a JSON array.")
		return
	}
	q, dup, err := h.store.Create(r.Context(), sess.UID, sess.CompanyID, store.QuotationWrite{
		ClientID: store.ID(form.String("client_id")), QuotationNumber: form.String("quotationNumber"),
		Date: form.String("date"), Rows: rows,
	})
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if dup {
		invalid(w, "This quotation number is already in use.")
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Quotation created.", Data: toDTO(q)})
}

// Update is POST /quotation/update: a partial edit; rows, when sent, replace the set.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	id := form.String("quotation_id")
	if id == "" {
		invalid(w, "Invalid request.")
		return
	}
	var patch store.QuotationUpdate
	if form.Has("client_id") {
		cid := store.ID(form.String("client_id"))
		patch.ClientID = &cid
	}
	if form.Has("date") {
		v := form.String("date")
		patch.Date = &v
	}
	if form.Has("quotationNumber") {
		v := form.String("quotationNumber")
		patch.QuotationNumber = &v
	}
	if form.Has("rows") {
		rows, ok := parseRows(form.String("rows"))
		if !ok {
			invalid(w, "rows must be a JSON array.")
			return
		}
		patch.Rows = &rows
	}
	q, dup, found, err := h.store.Update(r.Context(), sess.UID, sess.CompanyID, store.ID(id), patch)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if dup {
		invalid(w, "This quotation number is already in use.")
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Quotation not found.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Quotation updated.", Data: toDTO(q)})
}

// Delete is POST /quotation/delete.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	id := form.String("quotation_id")
	if id == "" {
		invalid(w, "Invalid request.")
		return
	}
	found, err := h.store.Delete(r.Context(), sess.UID, sess.CompanyID, store.ID(id))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Quotation not found.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Quotation deleted.", Status: httpx.True()})
}

// RowDelete is POST /quotation/row/delete: remove one row and return the quotation.
func (h *Handler) RowDelete(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	id := form.String("quotation_id")
	rowID := form.String("row_id")
	if id == "" || rowID == "" {
		invalid(w, "Invalid request.")
		return
	}
	q, found, err := h.store.RowDelete(r.Context(), sess.UID, sess.CompanyID, store.ID(id), store.ID(rowID))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Quotation not found.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Row removed.", Data: toDTO(q)})
}

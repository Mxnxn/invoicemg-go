package material

import (
	"net/http"
	"regexp"
	"strconv"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

// writeDTO is the record /material/add and /material/update echo back - Node returns the raw
// Mongoose doc (no `borrowed`, no sharing). `unit` is carried so the row merges cleanly into the
// list the getall read populated, which does include it.
type writeDTO struct {
	ID           string            `json:"_id"`
	UID          string            `json:"uid"`
	CompanyID    *string           `json:"company_id"`
	MaterialName string            `json:"material_name"`
	MaterialRate float64           `json:"material_rate"`
	PurchaseRate float64           `json:"purchase_rate"`
	Unit         string            `json:"unit"`
	Hsn          string            `json:"hsn"`
	Tax          float64           `json:"tax"`
	PriceHistory []priceHistoryDTO `json:"priceHistory"`
	CreatedAt    httpx.Time        `json:"createdAt"`
	UpdatedAt    httpx.Time        `json:"updatedAt"`
	Version      int               `json:"__v"`
}

func toWriteDTO(m store.Material) writeDTO {
	hist := make([]priceHistoryDTO, 0, len(m.PriceHistory))
	for _, h := range m.PriceHistory {
		hist = append(hist, priceHistoryDTO{MaterialRate: h.MaterialRate, PurchaseRate: h.PurchaseRate, ChangedAt: httpx.NewTime(h.ChangedAt)})
	}
	return writeDTO{
		ID: string(m.ID), UID: string(m.UID), CompanyID: idPtr(m.CompanyID),
		MaterialName: m.MaterialName, MaterialRate: m.MaterialRate, PurchaseRate: m.PurchaseRate,
		Unit: m.Unit, Hsn: m.Hsn, Tax: m.Tax, PriceHistory: hist,
		CreatedAt: httpx.NewTime(m.CreatedAt), UpdatedAt: httpx.NewTime(m.UpdatedAt), Version: m.Version,
	}
}

// invalid is /material/*'s 422: status:false, "Invalid request." (with the period).
func invalid(w http.ResponseWriter) {
	httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
}

// readWrite pulls the writable fields. material_rate/purchase_rate are Number()-coerced; tax is
// Number(x)||0. Presence of the three required fields is checked by the caller as raw strings,
// matching Node's truthiness test (so "0" is present but numerically zero).
func readWrite(form *httpx.Form) (store.MaterialWrite, bool) {
	name := form.String("material_name")
	rateStr := form.String("material_rate")
	purStr := form.String("purchase_rate")
	present := name != "" && rateStr != "" && purStr != ""
	return store.MaterialWrite{
		MaterialName: name,
		MaterialRate: numOr(rateStr, 0),
		PurchaseRate: numOr(purStr, 0),
		Hsn:          form.String("hsn"),
		Tax:          numOr(form.String("tax"), 0),
	}, present
}

func numOr(s string, def float64) float64 {
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return def
}

// Add is POST /material/add: create a product owned by the acting user, company-scoped.
func (h *Handler) Add(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)
	form, _ := httpx.ReadForm(r)

	in, present := readWrite(form)
	if !present {
		invalid(w)
		return
	}
	created, err := h.store.Create(ctx, sess.CompanyID, sess.UID, in)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Material has added.", Data: toWriteDTO(created)})
}

// Update is POST /material/update: edit a product; a rate change appends a price-history entry.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)
	form, _ := httpx.ReadForm(r)

	materialID := form.String("material_id")
	in, present := readWrite(form)
	if materialID == "" || !present {
		invalid(w)
		return
	}
	updated, found, err := h.store.Update(ctx, sess.CompanyID, store.ID(materialID), in)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Material not found.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: toWriteDTO(updated)})
}

// Remove is POST /material/remove: hard-delete a product (company-scoped). Matching Node, a miss
// is NOT a 404 here - the response is 200 either way.
func (h *Handler) Remove(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)
	form, _ := httpx.ReadForm(r)

	materialID := form.String("material_id")
	if materialID == "" {
		invalid(w)
		return
	}
	if err := h.store.Delete(ctx, sess.CompanyID, store.ID(materialID)); err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Material has removed.", Status: httpx.True()})
}

// objectIDRe is Node's own guard, `/^[0-9a-fA-F]{24}$/`. /material/set-unit checks the id shape
// itself and answers 404 for a malformed one, rather than letting Mongoose's CastError surface
// as a 500 - the same guard, for the same reason, as routes/Alert.js.
var objectIDRe = regexp.MustCompile(`^[0-9a-fA-F]{24}$`)

// SetUnit is POST /material/set-unit: set a company-OWNED product's stocked unit. Behind
// requireCreate("products"). A missing id or unit is 422; a malformed id is 404 (guarded, not a
// 500); a product owned by another company (shared in) is 404 with a message that says so.
func (h *Handler) SetUnit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)
	form, _ := httpx.ReadForm(r)

	materialID := form.String("material_id")
	unit := form.String("unit")
	if materialID == "" || unit == "" {
		invalid(w)
		return
	}
	if !objectIDRe.MatchString(materialID) {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Product not found.", Status: httpx.False()})
		return
	}
	name, savedUnit, found, borrowed, err := h.store.SetUnit(ctx, sess.CompanyID, store.ID(materialID), unit)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		msg := "Product not found."
		if borrowed {
			msg = "That product belongs to another company. Set its unit from the company that owns it."
		}
		httpx.Write(w, httpx.Envelope{Code: 404, Message: msg, Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Unit set.", Status: httpx.True(), Data: setUnitDTO{
		ID: materialID, MaterialName: name, Unit: savedUnit,
	}})
}

// setUnitDTO is the projected doc Node returns from .select(["material_name","unit"]): _id plus
// the two fields, no __v (an inclusive projection drops it).
type setUnitDTO struct {
	ID           string `json:"_id"`
	MaterialName string `json:"material_name"`
	Unit         string `json:"unit"`
}

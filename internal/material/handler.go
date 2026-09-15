// Package material serves the product routes from routes/Material.js. /material/getall is the
// sharing-widened read (#1) - the whole product record plus a `borrowed` flag. The writes
// (add/update/remove) and the single-record /material/get are ported too, and each uses
// company-only scope, never this widened read.
package material

import (
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type Handler struct{ store store.Materials }

func New(s store.Materials) *Handler { return &Handler{store: s} }

// Getall is POST /material/getall: the product list for pickers and the Products tab, each row
// carrying the whole record (incl __v, priceHistory, timestamps) and a `borrowed` flag.
func (h *Handler) Getall(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	list, err := h.store.Visible(r.Context(), sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out := make([]materialDTO, 0, len(list))
	for _, m := range list {
		out = append(out, toDTO(m, sess.CompanyID))
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: out})
}

// Get is POST /material/get: one product by id, company-scoped (never the widened read). A
// miss answers 200 with data:null, exactly as Node's findOne returning null does.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	id := form.String("material_id")
	if id == "" {
		httpx.Invalid(w, "")
		return
	}
	m, found, err := h.store.Get(r.Context(), sess.CompanyID, store.ID(id))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: httpx.Null})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: toDTO(m, sess.CompanyID)})
}

// borrowed is isBorrowed: owned elsewhere but shared in - the same rule as clients.
func borrowed(m store.Material, companyID store.ID) bool {
	if m.CompanyID == companyID || m.Sharing == nil {
		return false
	}
	for _, id := range *m.Sharing {
		if id == companyID {
			return true
		}
	}
	return false
}

func toDTO(m store.Material, companyID store.ID) materialDTO {
	hist := make([]priceHistoryDTO, 0, len(m.PriceHistory))
	for _, h := range m.PriceHistory {
		hist = append(hist, priceHistoryDTO{
			MaterialRate: h.MaterialRate, PurchaseRate: h.PurchaseRate, ChangedAt: httpx.NewTime(h.ChangedAt),
		})
	}
	dto := materialDTO{
		ID: string(m.ID), UID: string(m.UID), CompanyID: idPtr(m.CompanyID),
		MaterialName: m.MaterialName, MaterialRate: m.MaterialRate, PurchaseRate: m.PurchaseRate,
		Unit: m.Unit, Hsn: m.Hsn, Tax: m.Tax, PriceHistory: hist,
		CreatedAt: httpx.NewTime(m.CreatedAt), UpdatedAt: httpx.NewTime(m.UpdatedAt),
		Version:  m.Version,
		Borrowed: borrowed(m, companyID),
	}
	if m.Sharing != nil {
		companies := make([]string, 0, len(*m.Sharing))
		for _, id := range *m.Sharing {
			companies = append(companies, string(id))
		}
		dto.Sharing = &sharingDTO{Companies: companies}
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

type materialDTO struct {
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
	Sharing      *sharingDTO       `json:"sharing,omitempty"`
	CreatedAt    httpx.Time        `json:"createdAt"`
	UpdatedAt    httpx.Time        `json:"updatedAt"`
	Version      int               `json:"__v"`
	Borrowed     bool              `json:"borrowed"`
}

type priceHistoryDTO struct {
	MaterialRate float64    `json:"material_rate"`
	PurchaseRate float64    `json:"purchase_rate"`
	ChangedAt    httpx.Time `json:"changed_at"`
}

type sharingDTO struct {
	Companies []string `json:"companies"`
}

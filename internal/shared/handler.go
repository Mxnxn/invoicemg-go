// Package shared serves the read-only cross-account lists from routes/Shared.js:
// /shared/customers and /shared/materials. Each spans every company the acting owner has WHEN the
// acting company's reportsAcrossCompanies toggle is on, and just the acting company otherwise -
// the widening decision lives entirely in Companies.Scope (Helpers/CompanyScope.scopeFor).
//
// DELIBERATELY separate from /client and /material (routes/Shared.js header): those lists feed
// pickers and duplicate guards on write flows, so widening them would let a user pull another
// company's customer into a document owned by this one. Nothing here is ever a picker's source -
// every route is a read that carries each row's owning company so a total spanning three
// businesses can never look like it spans one.
package shared

import (
	"net/http"
	"strings"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type Handler struct {
	companies store.Companies
	clients   store.Clients
	materials store.Materials
}

func New(co store.Companies, cl store.Clients, m store.Materials) *Handler {
	return &Handler{companies: co, clients: cl, materials: m}
}

// customerRow mirrors Node's withCompany(client) projection: the selected fields plus the owning
// company's label. No __v - the Mongoose inclusion select never carried it.
type customerRow struct {
	ID          string `json:"_id"`
	ClientName  string `json:"clientName"`
	ClientFirm  string `json:"clientFirm"`
	ClientPhone string `json:"clientPhone"`
	ClientGST   string `json:"clientGST"`
	CompanyID   string `json:"company_id"`
	CompanyName string `json:"companyName"`
}

// Customers is GET /shared/customers (requireFeature "customers"): every customer across the
// scoped companies, firm-sorted by the store, each tagged with its owning company and a tally of
// how many phone numbers occur in more than one company.
func (h *Handler) Customers(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	companyIDs, shared, labels, err := h.companies.Scope(r.Context(), sess.CompanyID, sess.UID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	list, err := h.clients.SharedList(r.Context(), companyIDs)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	rows := make([]customerRow, 0, len(list))
	phones := make([]string, 0, len(list))
	for _, c := range list {
		rows = append(rows, customerRow{
			ID: string(c.ID), ClientName: c.ClientName, ClientFirm: c.ClientFirm,
			ClientPhone: c.ClientPhone, ClientGST: c.ClientGST,
			CompanyID: string(c.CompanyID), CompanyName: labels[c.CompanyID],
		})
		phones = append(phones, c.ClientPhone)
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Status: httpx.True(), Data: map[string]any{
		"rows":         rows,
		"shared":       shared,
		"companyCount": len(companyIDs),
		"duplicates":   countDuplicates(phones),
	}})
}

// materialRow mirrors Node's withCompany(material) projection.
type materialRow struct {
	ID           string  `json:"_id"`
	MaterialName string  `json:"material_name"`
	MaterialRate float64 `json:"material_rate"`
	PurchaseRate float64 `json:"purchase_rate"`
	Hsn          string  `json:"hsn"`
	Unit         string  `json:"unit"`
	CompanyID    string  `json:"company_id"`
	CompanyName  string  `json:"companyName"`
}

// Materials is GET /shared/materials (requireFeature "products"): every product across the scoped
// companies, name-sorted by the store, with the duplicate tally taken on the product name.
func (h *Handler) Materials(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	companyIDs, shared, labels, err := h.companies.Scope(r.Context(), sess.CompanyID, sess.UID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	list, err := h.materials.SharedList(r.Context(), companyIDs)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	rows := make([]materialRow, 0, len(list))
	names := make([]string, 0, len(list))
	for _, m := range list {
		rows = append(rows, materialRow{
			ID: string(m.ID), MaterialName: m.MaterialName, MaterialRate: m.MaterialRate,
			PurchaseRate: m.PurchaseRate, Hsn: m.Hsn, Unit: m.Unit,
			CompanyID: string(m.CompanyID), CompanyName: labels[m.CompanyID],
		})
		names = append(names, m.MaterialName)
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Status: httpx.True(), Data: map[string]any{
		"rows":         rows,
		"shared":       shared,
		"companyCount": len(companyIDs),
		"duplicates":   countDuplicates(names),
	}})
}

// countDuplicates reports how many trimmed, lower-cased values occur in more than one row - the
// duplicate tally Node's Shared.js countDuplicates computes (blank values ignored). It answers
// "how much duplication is there", not "which rows", which is what the report's header needs.
func countDuplicates(values []string) int {
	seen := make(map[string]int, len(values))
	for _, v := range values {
		k := strings.ToLower(strings.TrimSpace(v))
		if k == "" {
			continue
		}
		seen[k]++
	}
	n := 0
	for _, c := range seen {
		if c > 1 {
			n++
		}
	}
	return n
}

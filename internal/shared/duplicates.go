package shared

import (
	"net/http"
	"sort"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/inventorymath"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

// dupCopy is one company's copy of a duplicated product, as routes/Shared.js emits it.
type dupCopy struct {
	ID           string  `json:"_id"`
	MaterialName string  `json:"material_name"`
	MaterialRate float64 `json:"material_rate"`
	PurchaseRate float64 `json:"purchase_rate"`
	Hsn          string  `json:"hsn"`
	Unit         string  `json:"unit"`
	CompanyID    string  `json:"company_id"`
	CompanyName  string  `json:"companyName"`
	SharedWith   int     `json:"sharedWith"`
}

// dupRow is one duplicated product grouped across the companies that hold it.
type dupRow struct {
	Key           string    `json:"key"`
	Name          string    `json:"name"`
	Copies        []dupCopy `json:"copies"`
	Companies     []string  `json:"companies"`
	PriceMismatch bool      `json:"priceMismatch"`
	LowestRate    float64   `json:"lowestRate"`
	HighestRate   float64   `json:"highestRate"`
}

// DuplicateMaterials is GET /shared/duplicate-materials (requireFeature "products"): the products
// entered separately in more than one of the owner's companies, grouped on the normalised name so
// "Art Card 300gsm" and "art card 300 gsm" collapse into one row. ALWAYS the owner's full company
// set, never the report toggle's scope - this report is read BEFORE deciding about sharing, so
// inheriting the toggle would hide duplicates exactly when they matter most (routes/Shared.js).
func (h *Handler) DuplicateMaterials(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	owned, err := h.companies.List(r.Context(), sess.UID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	companyIDs := make([]store.ID, 0, len(owned))
	labels := make(map[store.ID]string, len(owned))
	for _, c := range owned {
		companyIDs = append(companyIDs, c.ID)
		labels[c.ID] = companyLabel(c)
	}

	mats, err := h.materials.DuplicatesSource(r.Context(), companyIDs)
	if err != nil {
		httpx.Internal(w, err)
		return
	}

	// Group on the normalised name; blank keys are dropped, matching Node's `if (!key) continue`.
	groups := map[string][]dupCopy{}
	for _, m := range mats {
		key := inventorymath.NormaliseKey(m.MaterialName)
		if key == "" {
			continue
		}
		groups[key] = append(groups[key], dupCopy{
			ID: string(m.ID), MaterialName: m.MaterialName, MaterialRate: m.MaterialRate,
			PurchaseRate: m.PurchaseRate, Hsn: m.Hsn, Unit: m.Unit,
			CompanyID: string(m.CompanyID), CompanyName: labels[m.CompanyID], SharedWith: m.SharedWith,
		})
	}

	rows := make([]dupRow, 0)
	for key, copies := range groups {
		if len(copies) <= 1 {
			continue
		}
		// Copies read left-to-right by owning company, the way the table is worked through.
		sort.SliceStable(copies, func(i, j int) bool { return copies[i].CompanyName < copies[j].CompanyName })

		row := dupRow{
			Key:         key,
			Name:        longestName(copies),
			Copies:      copies,
			Companies:   distinctCompanyNames(copies),
			LowestRate:  copies[0].MaterialRate,
			HighestRate: copies[0].MaterialRate,
		}
		distinctRates := map[float64]struct{}{}
		for _, c := range copies {
			distinctRates[c.MaterialRate] = struct{}{}
			if c.MaterialRate < row.LowestRate {
				row.LowestRate = c.MaterialRate
			}
			if c.MaterialRate > row.HighestRate {
				row.HighestRate = c.MaterialRate
			}
		}
		// Flagged rather than averaged: two companies at two rates is the finding, and an average
		// would hide it.
		row.PriceMismatch = len(distinctRates) > 1
		rows = append(rows, row)
	}
	// The longest spelling reads best as the heading and the rows sort by it (Node localeCompare;
	// exact tie-break parity awaits a Node diff).
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })

	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Status: httpx.True(),
		Data: map[string]any{"rows": rows}})
}

// companyLabel is name || firm || "" - the label every shared row carries (Helpers/CompanyScope
// labelFor).
func companyLabel(c store.Company) string {
	if c.Name != "" {
		return c.Name
	}
	return c.Firm
}

// longestName picks the longest material_name in the group as its heading (the short one is
// usually the abbreviated retype); first-seen wins a length tie, matching Node's stable sort.
func longestName(copies []dupCopy) string {
	name := ""
	for _, c := range copies {
		if len(c.MaterialName) > len(name) {
			name = c.MaterialName
		}
	}
	return name
}

// distinctCompanyNames is the non-empty company labels in the group, de-duplicated in the copies'
// (company-name) order.
func distinctCompanyNames(copies []dupCopy) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(copies))
	for _, c := range copies {
		if c.CompanyName == "" {
			continue
		}
		if _, dup := seen[c.CompanyName]; dup {
			continue
		}
		seen[c.CompanyName] = struct{}{}
		out = append(out, c.CompanyName)
	}
	return out
}

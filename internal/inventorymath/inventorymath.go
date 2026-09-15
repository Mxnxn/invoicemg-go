// Package inventorymath ports Helpers/InventoryMath.js - the stock report's per-unit
// consumption rules and roll-up. Pure (no store), the same shape as jobmath/entrymath; the
// caller maps its rows in and gets stock lines + a "not counted" list back.
package inventorymath

import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// unitRule says which dimensions a unit reads and how it computes consumption.
type unitRule struct {
	reads []string
	of    func(l, w, q float64) float64
}

var unitRules = map[string]unitRule{
	"sq. ft": {[]string{"length", "width"}, func(l, w, q float64) float64 { return l * w * q }},
	"sq. in": {[]string{"length", "width"}, func(l, w, q float64) float64 { return l * w * q }},
	"qty":    {nil, func(l, w, q float64) float64 { return q }},
	"piece":  {nil, func(l, w, q float64) float64 { return q }},
	"mm":     {[]string{"length"}, func(l, w, q float64) float64 { return l * q }},
	"cm":     {[]string{"length"}, func(l, w, q float64) float64 { return l * q }},
	"in":     {[]string{"length"}, func(l, w, q float64) float64 { return l * q }},
	"feet":   {[]string{"length"}, func(l, w, q float64) float64 { return l * q }},
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// NormaliseKey lowercases, replaces punctuation with spaces, collapses, trims - the product
// match key, identical to the Node helper (digits kept).
func NormaliseKey(name string) string {
	return strings.TrimSpace(nonAlnum.ReplaceAllString(strings.ToLower(name), " "))
}

func ruleFor(unit string) (unitRule, bool) {
	r, ok := unitRules[strings.TrimSpace(strings.ToLower(unit))]
	return r, ok
}

// parses is Number.isFinite over a non-empty string value.
func parses(v string) bool {
	if v == "" {
		return false
	}
	f, err := strconv.ParseFloat(v, 64)
	return err == nil && !math.IsInf(f, 0) && !math.IsNaN(f)
}

func numStr(v string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil {
		return 0
	}
	return f
}

// Material is a product row. Unit drives the stock rule.
type Material struct {
	ID           string
	MaterialName string
	Unit         string
	PurchaseRate float64
}

// PurchaseRow / JobRow / WastageRow carry the fields the rules read plus the parent company.
type PurchaseRow struct {
	ID        string
	Material  string
	Qty       float64
	Rate      float64
	CompanyID string
}
type JobRow struct {
	ID            string
	Material      string
	Length        string
	Width         string
	Qty           float64
	HasDimensions *bool
	CompanyID     string
}
type WastageRow struct {
	ID           string
	MaterialName string
	Length       float64
	Height       float64
	CompanyID    string
}

// Line is one product's stock roll-up.
type Line struct {
	Material      string   `json:"material"`
	Unit          string   `json:"unit"`
	Purchased     float64  `json:"purchased"`
	Consumed      float64  `json:"consumed"`
	Wasted        float64  `json:"wasted"`
	PurchaseValue float64  `json:"purchaseValue"`
	InStock       float64  `json:"inStock"`
	Negative      bool     `json:"negative"`
	Companies     []string `json:"companies"`
}

// NotCounted is one row the report could not attribute.
type NotCounted struct {
	Source string `json:"source"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
	Ref    string `json:"ref"`
}

// Input bundles the four row sources and the company id->label map.
type Input struct {
	Materials    []Material
	PurchaseRows []PurchaseRow
	JobRows      []JobRow
	WastageRows  []WastageRow
	Labels       map[string]string
}

// Build is buildInventory: roll purchases/consumption/wastage into per-product stock lines.
func Build(in Input) ([]Line, []NotCounted) {
	type acc struct {
		Line
		companies map[string]bool
	}
	byKey := map[string]*acc{}
	var notCounted []NotCounted
	note := func(source, name, reason, ref string) {
		notCounted = append(notCounted, NotCounted{Source: source, Name: name, Reason: reason, Ref: ref})
	}

	for _, m := range in.Materials {
		key := NormaliseKey(m.MaterialName)
		if key == "" {
			continue
		}
		if _, ok := ruleFor(m.Unit); !ok {
			reason := "Product has no unit"
			if m.Unit != "" {
				reason = `Unit "` + m.Unit + `" has no stock rule`
			}
			note("product", m.MaterialName, reason, m.ID)
			continue
		}
		byKey[key] = &acc{Line: Line{Material: m.MaterialName, Unit: m.Unit}, companies: map[string]bool{}}
	}
	seen := func(key, companyID string) {
		if a := byKey[key]; a != nil {
			if label := in.Labels[companyID]; label != "" {
				a.companies[label] = true
			}
		}
	}

	for _, r := range in.PurchaseRows {
		key := NormaliseKey(r.Material)
		a := byKey[key]
		if a == nil {
			note("purchase", r.Material, "No matching product", r.ID)
			continue
		}
		qty := r.Qty
		a.Purchased += qty
		a.PurchaseValue += qty * r.Rate
		seen(key, r.CompanyID)
	}

	for _, r := range in.JobRows {
		key := NormaliseKey(r.Material)
		a := byKey[key]
		if a == nil {
			note("job", r.Material, "No matching product", r.ID)
			continue
		}
		rule, _ := ruleFor(a.Unit)
		if len(rule.reads) > 0 && !(r.HasDimensions == nil || *r.HasDimensions) {
			note("job", r.Material, "Row is priced by quantity, so it has no size to measure in "+a.Unit, r.ID)
			seen(key, r.CompanyID)
			continue
		}
		fields := map[string]string{"length": r.Length, "width": r.Width}
		for _, f := range rule.reads {
			if !parses(fields[f]) {
				note("job", r.Material, "Dimensions could not be read", r.ID)
				break
			}
		}
		// qty comes straight through: the DB column defaults to 1, so an "absent" qty is
		// already 1 here, matching Node's `r.qty ?? 1` without conflating a genuine 0.
		a.Consumed += rule.of(numStr(r.Length), numStr(r.Width), r.Qty)
		seen(key, r.CompanyID)
	}

	for _, r := range in.WastageRows {
		key := NormaliseKey(r.MaterialName)
		a := byKey[key]
		if a == nil {
			note("wastage", r.MaterialName, "No matching product", r.ID)
			continue
		}
		rule, _ := ruleFor(a.Unit)
		a.Wasted += rule.of(r.Length, r.Height, 1)
		seen(key, r.CompanyID)
	}

	rows := make([]Line, 0, len(byKey))
	for _, a := range byKey {
		a.InStock = a.Purchased - a.Consumed - a.Wasted
		a.Negative = a.InStock < 0
		a.Companies = make([]string, 0, len(a.companies))
		for c := range a.companies {
			a.Companies = append(a.Companies, c)
		}
		sort.Strings(a.Companies)
		rows = append(rows, a.Line)
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Material < rows[j].Material })
	if notCounted == nil {
		notCounted = []NotCounted{}
	}
	return rows, notCounted
}

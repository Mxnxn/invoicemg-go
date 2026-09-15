package store

import (
	"math"
	"sort"
)

// PayableRow is one purchase invoice's payable inputs (Helpers/PayablesMath.js).
type PayableRow struct {
	Total    float64
	Amount   float64
	Supplier string // "" -> "Unknown supplier"
}

// SumPayables totals outstanding payables and groups by supplier, dropping non-positive dues
// and rounding to the paisa - a direct port of Helpers/PayablesMath.sumPayables.
func SumPayables(rows []PayableRow) (float64, []SupplierDue) {
	bySupplier := map[string]float64{}
	order := []string{}
	var total float64
	for _, r := range rows {
		due := r.Total - r.Amount
		if due <= 0 {
			continue
		}
		total += due
		name := r.Supplier
		if name == "" {
			name = "Unknown supplier"
		}
		if _, ok := bySupplier[name]; !ok {
			order = append(order, name)
		}
		bySupplier[name] += due
	}
	out := make([]SupplierDue, 0, len(order))
	for _, name := range order {
		out = append(out, SupplierDue{Name: name, Due: round2(bySupplier[name])})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Due > out[j].Due })
	return round2(total), out
}

func round2(n float64) float64 { return math.Floor(n*100+0.5) / 100 }

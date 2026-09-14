// Package jobmath is the job/quotation/entry row money math, ported from the Node helpers
// Helpers/JobTotals.js and Helpers/RowPricing.js.
//
// It is a pure package with no store or driver dependency, the same shape as internal/textcase:
// the caller maps its rows into jobmath.Row and gets numbers back. The formula must agree with
// Node to the paisa, because the same job's value is printed on a customer's PDF here and on
// the invoice they are sent later - a second copy that drifts would make the two disagree.
package jobmath

import (
	"math"
	"strconv"
	"strings"
)

// Row is the pricing-relevant subset of a row. HasDimensions is a pointer because "absent"
// must mean by-dimension, not false: every row written before the field existed has no value,
// and those rows are dimensional (Helpers/RowPricing.js). A nil here is Node's `!== false`.
type Row struct {
	Qty           float64
	Rate          float64
	Cgst          float64
	Sgst          float64
	Igst          float64
	Discount      float64
	Charges       float64
	Length        string
	Width         string
	HasDimensions *bool
}

// HasDimensions is Node's `row.hasDimensions !== false`: true unless the row explicitly says
// false, so an absent value (a pre-existing row) prices by dimension.
func HasDimensions(r Row) bool {
	return r.HasDimensions == nil || *r.HasDimensions
}

// side is JavaScript's `Number(value) || blank`: an empty, unparseable, or zero side counts as
// blank. `Number("") || b` is b (0 is falsy), and so is `Number("abc") || b`, so a blank or
// junk dimension collapses to the caller's blank rather than to a literal 0 that would make
// every such line worth nothing by surprise.
func side(value string, blank float64) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || f == 0 {
		return blank
	}
	return f
}

// DimensionFactor is the area multiplier: 1 for a by-quantity row, length*width for a
// by-dimension one, which collapses the single formula qty*factor*rate back to qty*rate
// without a second code path.
//
// blank is what an empty side counts as, and callers disagree on purpose (Helpers/RowPricing.js):
// 0 for jobs and quotations (a blank there is a real zero), 1 for entries and invoice templates
// (older flat-rate lines legitimately carry "" and must not be re-priced to zero).
func DimensionFactor(r Row, blank float64) float64 {
	if !HasDimensions(r) {
		return 1
	}
	return side(r.Length, blank) * side(r.Width, blank)
}

// Round2 is JavaScript's `Math.round(n * 100) / 100`, to the paisa.
//
// It is math.Floor(x+0.5), NOT Go's math.Round: the two agree on positive values but differ on
// a negative half - Math.round(-250.5) is -250 (toward +Infinity) while math.Round(-250.5) is
// -251 (away from zero). A discounted row can go negative, so the difference is reachable, and
// the Node value is the one that must win. A NaN reads as 0, matching `Number(n) || 0`.
func Round2(n float64) float64 {
	if math.IsNaN(n) {
		n = 0
	}
	return math.Floor(n*100+0.5) / 100
}

// RowGrossTotal is one row's value: qty * area * rate, net of discount and charges, inclusive
// of tax. Ported verbatim from Helpers/JobTotals.js rowGrossTotal, including the per-row
// rounding - a line on an invoice is rounded, so the row is too.
func RowGrossTotal(r Row) float64 {
	amount := r.Qty * DimensionFactor(r, 0) * r.Rate
	netAmount := amount - r.Discount + r.Charges
	taxPct := (r.Cgst + r.Sgst + r.Igst) / 100
	return Round2(netAmount * (1 + taxPct))
}

// JobRowsTotal is the job's value: the sum of the already-rounded row totals, rounded again -
// the same double rounding Node applies, so the stored figure is the one a person would write
// down.
func JobRowsTotal(rows []Row) float64 {
	var sum float64
	for _, r := range rows {
		sum += RowGrossTotal(r)
	}
	return Round2(sum)
}

// Package entrymath is the invoice-entry money math, ported from Helpers/EntryTotals.js and the
// RoundOffWithAmount half of Helpers/RoundOff.js. Kept separate from jobmath: an Entry's total
// formula and the rupee-rounding rule are the invoice side of the books, and reproducing them
// to the paisa is what keeps a Go-served invoice list reconciling against the Node one.
package entrymath

import (
	"math"
	"strconv"
	"strings"
)

// Entry is the pricing subset of an Entry document.
type Entry struct {
	Amount   float64
	Discount float64
	Charges  float64
	Cgst     float64
	Sgst     float64
	Igst     float64
}

// TaxPct is cgst + sgst + igst (whole percent).
func TaxPct(e Entry) float64 { return e.Cgst + e.Sgst + e.Igst }

// NetAmount is amount - discount + charges, before tax.
func NetAmount(e Entry) float64 { return e.Amount - e.Discount + e.Charges }

// Total is what the customer is billed for this line, tax included - Helpers/EntryTotals.js
// entryTotal. Advance is deliberately NOT subtracted (the invoice total is the gross figure).
func Total(e Entry) float64 { return NetAmount(e) * (1 + TaxPct(e)/100) }

// RoundOffWithAmount is the rupee-rounding rule from Helpers/RoundOff.js: round to two
// decimals, then if the paise part is strictly greater than 50 round the rupees up, otherwise
// down - returning a whole-rupee figure. A non-finite input reads as 0, never NaN.
//
// It reproduces the Node function's exact steps (toFixed(2), split on ".", parseInt each half)
// rather than a cleaner equivalent, because the cleaner equivalent rounds .50 and negatives
// differently and this figure is reconciled against Node's.
func RoundOffWithAmount(amt float64) float64 {
	if math.IsNaN(amt) || math.IsInf(amt, 0) {
		amt = 0
	}
	rounded := math.Floor(amt*100+0.5) / 100 // Math.round(amt*100)/100
	s := strconv.FormatFloat(rounded, 'f', 2, 64)
	parts := strings.SplitN(s, ".", 2)
	whole, _ := strconv.Atoi(parts[0]) // parseInt(parts[0]) - keeps the sign
	paise, _ := strconv.Atoi(parts[1]) // parseInt(decimal)
	if paise > 50 {
		return float64(whole + 1)
	}
	return float64(whole)
}

package jobmath

import (
	"math"
	"testing"
)

// Every want in this file is what the real Node helpers printed for the same inputs
// (Helpers/JobTotals.js, run under node), not a hand-derived figure - the point of the port is
// that the two agree to the paisa, and only Node can say what Node computes. See the generator
// in the commit message.
//
// Comparison is to 1e-9: a real divergence in this math is at least a paisa (0.01), so this
// tolerates IEEE round-trip noise while still failing on any difference that would show on a
// document.
const eps = 1e-9

func near(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > eps {
		t.Errorf("%s = %v, want %v (Node)", name, got, want)
	}
}

func boolPtr(b bool) *bool { return &b }

func TestRowGrossTotal_MatchesNode(t *testing.T) {
	cases := []struct {
		name string
		row  Row
		want float64
	}{
		{"byDim_notax", Row{Qty: 2, Length: "3", Width: "4", Rate: 10}, 240},
		{"byDim_tax18", Row{Qty: 1, Length: "1", Width: "1", Rate: 100, Cgst: 9, Sgst: 9}, 118},
		// 10 * 1.18 is 11.799999999999999 in IEEE; rounded per row it is 11.8.
		{"taxResidue", Row{Qty: 1, Length: "1", Width: "1", Rate: 10, Cgst: 9, Sgst: 9}, 11.8},
		// by-quantity: dimensions ignored even though they are set.
		{"byQty", Row{Qty: 5, Rate: 3, HasDimensions: boolPtr(false), Length: "9", Width: "9"}, 15},
		// a blank or zero side collapses to blank (0) for a job row, so the line is worth 0.
		{"blankSide", Row{Qty: 2, Length: "", Width: "5", Rate: 10}, 0},
		{"zeroSide", Row{Qty: 2, Length: "0", Width: "5", Rate: 10}, 0},
		{"discountCharges", Row{Qty: 1, Length: "1", Width: "1", Rate: 100, Discount: 20, Charges: 5, Igst: 18}, 100.3},
		// no HasDimensions (nil) prices by dimension - the pre-existing-row case.
		{"hasDimUndefined", Row{Qty: 2, Length: "3", Width: "4", Rate: 10}, 240},
		// 10 - 12.505 is not the literal -2.505 in float, and the difference reaches the paisa:
		// Node rounds this to -2.51 even though round2(-2.505) alone is -2.5.
		{"negativeNet", Row{Qty: 1, Length: "1", Width: "1", Rate: 10, Discount: 12.505}, -2.51},
	}
	for _, c := range cases {
		near(t, "RowGrossTotal/"+c.name, RowGrossTotal(c.row), c.want)
	}
}

func TestJobRowsTotal_MatchesNode(t *testing.T) {
	rows := []Row{
		{Qty: 1, Length: "1", Width: "1", Rate: 10, Cgst: 9, Sgst: 9}, // 11.8
		{Qty: 5, Rate: 3, HasDimensions: boolPtr(false)},              // 15
	}
	near(t, "JobRowsTotal", JobRowsTotal(rows), 26.8)
}

// Round2 is Math.round(n*100)/100. Both values are what Node printed.
func TestRound2_MatchesNode(t *testing.T) {
	// The multiplication 2.675*100 lands at or above 267.5 in IEEE, so this rounds UP to 2.68 -
	// the same value Node produces, and the reason floor(x+0.5) must use the identical ops.
	near(t, "Round2(2.675)", Round2(2.675), 2.68)
	// -2.505*100 lands just above -250.5, so Math.round gives -250 -> -2.5. Go's math.Round
	// would give -2.51 for an exact -250.5, which is why Round2 is floor(x+0.5), not math.Round.
	near(t, "Round2(-2.505)", Round2(-2.505), -2.5)
}

func TestHasDimensions(t *testing.T) {
	if !HasDimensions(Row{}) {
		t.Error("nil HasDimensions must be true (by dimension)")
	}
	if !HasDimensions(Row{HasDimensions: boolPtr(true)}) {
		t.Error("true must be true")
	}
	if HasDimensions(Row{HasDimensions: boolPtr(false)}) {
		t.Error("false must be false")
	}
}

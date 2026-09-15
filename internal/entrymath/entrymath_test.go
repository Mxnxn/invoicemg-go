package entrymath

import (
	"math"
	"testing"
)

// Values captured from the real Node helpers (Helpers/EntryTotals.js, Helpers/RoundOff.js).
func near(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s = %v, want %v (Node)", name, got, want)
	}
}

func TestTotal_MatchesNode(t *testing.T) {
	near(t, "basic", Total(Entry{Amount: 100, Cgst: 9, Sgst: 9}), 118)
	near(t, "disc", Total(Entry{Amount: 100, Discount: 20, Charges: 5, Igst: 18}), 100.3)
	near(t, "notax", Total(Entry{Amount: 250}), 250)
}

func TestRoundOffWithAmount_MatchesNode(t *testing.T) {
	near(t, "2670.40", RoundOffWithAmount(2670.40), 2670) // paise 40, not > 50 -> down
	near(t, "2670.60", RoundOffWithAmount(2670.60), 2671) // paise 60 > 50 -> up
	near(t, "118", RoundOffWithAmount(118), 118)
	near(t, "NaN", RoundOffWithAmount(math.NaN()), 0)
}

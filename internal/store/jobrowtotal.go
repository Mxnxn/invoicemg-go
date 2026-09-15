package store

import (
	"strconv"
	"strings"
)

// JobRowPricing is the pricing-relevant subset of a job row for the total formula.
type JobRowPricing struct {
	Length   string
	Width    string
	Qty      float64
	Rate     float64
	Cgst     float64
	Sgst     float64
	Igst     float64
	Discount float64
	Charges  float64
}

// JobRowGrossTotal is routes/Lifecycle.rowGrossTotal: qty·length·width·rate, net of
// discount/charges, inclusive of tax. Job rows always price by dimension (length/width default
// to "1"), so a by-quantity row (L=W=1) collapses to qty·rate.
func JobRowGrossTotal(r JobRowPricing) float64 {
	amount := r.Qty * jobNum(r.Length) * jobNum(r.Width) * r.Rate
	net := amount - r.Discount + r.Charges
	tax := (r.Cgst + r.Sgst + r.Igst) / 100
	return net * (1 + tax)
}

func jobNum(s string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return f
}

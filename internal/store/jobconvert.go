package store

import "math"

// ConvertedEntry is the Entry produced from one job row (routes/Lifecycle convert-to-entries).
// amount is the pre-tax base (qty·L·W·rate); advance/total split the GROSS (tax- and
// charge-inclusive) by the job's paid ratio.
type ConvertedEntry struct {
	Description string
	Material    string
	Hsn         string
	// Unit is the product's unit of measure, snapshotted from the Material at conversion the same
	// way and at the same point as Hsn. The invoice's optional Unit column reads it.
	Unit        string
	Rate        float64
	Qty         float64
	Length      string
	Width       string
	Cgst        float64
	Sgst        float64
	Igst        float64
	Discount    float64
	Charges     float64
	Amount      float64
	Advance     float64
	Total       float64
}

// ConvertJobRow is one job row's inputs for conversion.
type ConvertJobRow struct {
	Material    string
	Description string
	Rate        float64
	Qty         float64
	Length      string
	Width       string
	Cgst        float64
	Sgst        float64
	Igst        float64
	Discount    float64
	Charges     float64
}

// ConvertRow computes the Entry fields for one row, given the owning job's paid ratio and the
// resolved hsn / display fallbacks. It ports the amount/gross/advance/total math exactly.
func ConvertRow(r ConvertJobRow, jobTotal, jobAdvance float64, hsn, unit, fallbackDesc string) ConvertedEntry {
	amount := r.Qty * jobNum(r.Length) * jobNum(r.Width) * r.Rate
	net := amount - r.Discount + r.Charges
	tax := (r.Cgst + r.Sgst + r.Igst) / 100
	gross := net * (1 + tax)
	paidRatio := 0.0
	if jobTotal > 0 {
		paidRatio = math.Min(1, math.Max(0, jobAdvance/jobTotal))
	}
	advance := math.Max(0, convRound2(gross*paidRatio))
	total := math.Max(0, convRound2(gross-advance))
	material := r.Material
	if material == "" {
		material = r.Description
	}
	if material == "" {
		material = fallbackDesc
	}
	description := r.Description
	if description == "" {
		description = r.Material
	}
	if description == "" {
		description = fallbackDesc
	}
	length := r.Length
	if length == "" {
		length = "0"
	}
	width := r.Width
	if width == "" {
		width = "0"
	}
	return ConvertedEntry{
		Description: description, Material: material, Hsn: hsn, Unit: unit, Rate: r.Rate, Qty: r.Qty,
		Length: length, Width: width, Cgst: r.Cgst, Sgst: r.Sgst, Igst: r.Igst,
		Discount: r.Discount, Charges: r.Charges, Amount: amount, Advance: advance, Total: total,
	}
}

func convRound2(n float64) float64 { return math.Floor(n*100+0.5) / 100 }

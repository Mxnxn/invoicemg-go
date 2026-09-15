// Package cashflow is the cash-and-profitability math for the Analytics cashflow dashboard,
// ported from Helpers/CashflowMetrics.js. It is a pure package with no store or driver
// dependency: the caller maps its rows in and gets numbers back.
//
// Two notions of "cost" live here on purpose, because they answer different questions and
// mixing them is wrong for both:
//
//	purchases - what suppliers billed this period (PurchaseInvoice.total). Real spend, ties to
//	            the books, but lumpy.
//	cogs      - the cost of the goods actually SOLD this period (purchase_rate x qty on the
//	            invoiced entries). Matches cost to the revenue it produced, so margin is stable.
//
// Cash movement (in/out/net) is measured from money that actually moved - receipts, transfers,
// supplier payments - never from invoiced amounts. Profit is measured from revenue and cogs,
// never from cash: an invoice raised is revenue, not cash until someone pays it.
package cashflow

import "math"

// roundMoney is Helpers/CashflowMetrics.roundMoney: Math.round((Number(x)||0)*100)/100, which
// floor(x*100+0.5)/100 reproduces (JS Math.round rounds a half toward +Inf).
func roundMoney(v float64) float64 { return math.Floor(v*100+0.5) / 100 }

// pct guards a zero denominator - a month with no revenue is normal and must not be NaN%.
func pct(numerator, denominator float64) float64 {
	if denominator > 0 {
		return roundMoney(numerator / denominator * 100)
	}
	return 0
}

// Amount is a single money row: only the one relevant value is read.
type Amount struct{ Value float64 }

// SoldEntry is an invoiced entry's cost basis. A product whose purchase rate could not be
// resolved (renamed or deleted since it was sold) contributes no cost - 0*qty - rather than
// voiding the report; the handler counts that coverage separately (cogsCoverage) so the gap
// is visible rather than silently inflating margin.
type SoldEntry struct {
	Qty          float64
	PurchaseRate float64
}

// Wastage carries both the selling total and the cost basis; Total is used when CostTotal is
// not positive (older rows have no cost basis, and an unset 0 is not treated as free).
type Wastage struct {
	Total     float64
	CostTotal float64
}

// Invoice contributes both the billed total (revenue) and the collected amount.
type Invoice struct {
	TotalAmount float64
	Amount      float64
}

// Input is one period's rows (already date-filtered by the caller).
type Input struct {
	Invoices         []Invoice
	Received         []Amount
	BatchReceives    []Amount
	SupplierPayments []Amount
	PurchaseInvoices []Amount
	SoldEntries      []SoldEntry
	Wastages         []Wastage
}

// Totals is the computed metric set, field names matching the Node wire shape.
type Totals struct {
	CashIn          float64 `json:"cashIn"`
	CashOut         float64 `json:"cashOut"`
	NetCashflow     float64 `json:"netCashflow"`
	Revenue         float64 `json:"revenue"`
	Collected       float64 `json:"collected"`
	Invoiced        float64 `json:"invoiced"`
	Purchases       float64 `json:"purchases"`
	Cogs            float64 `json:"cogs"`
	Wastage         float64 `json:"wastage"`
	GrossProfit     float64 `json:"grossProfit"`
	OperatingProfit float64 `json:"operatingProfit"`
	GrossMargin     float64 `json:"grossMargin"`
	OperatingMargin float64 `json:"operatingMargin"`
	CollectionRate  float64 `json:"collectionRate"`
}

func sum(rows []Amount) float64 {
	var t float64
	for _, r := range rows {
		t += r.Value
	}
	return t
}

// Compute ports computeCashflow.
func Compute(in Input) Totals {
	cashIn := roundMoney(sum(in.Received) + sum(in.BatchReceives))
	cashOut := roundMoney(sum(in.SupplierPayments))
	netCashflow := roundMoney(cashIn - cashOut)

	var revenueRaw, collectedRaw float64
	for _, iv := range in.Invoices {
		revenueRaw += iv.TotalAmount
		collectedRaw += iv.Amount
	}
	revenue := roundMoney(revenueRaw)
	collected := roundMoney(collectedRaw)
	purchases := roundMoney(sum(in.PurchaseInvoices))

	var cogsRaw float64
	for _, e := range in.SoldEntries {
		cogsRaw += e.Qty * e.PurchaseRate
	}
	cogs := roundMoney(cogsRaw)

	var wastageRaw float64
	for _, w := range in.Wastages {
		if w.CostTotal > 0 {
			wastageRaw += w.CostTotal
		} else {
			wastageRaw += w.Total
		}
	}
	wastage := roundMoney(wastageRaw)

	grossProfit := roundMoney(revenue - cogs)
	operatingProfit := roundMoney(revenue - cogs - wastage)

	return Totals{
		CashIn:          cashIn,
		CashOut:         cashOut,
		NetCashflow:     netCashflow,
		Revenue:         revenue,
		Collected:       collected,
		Invoiced:        revenue,
		Purchases:       purchases,
		Cogs:            cogs,
		Wastage:         wastage,
		GrossProfit:     grossProfit,
		OperatingProfit: operatingProfit,
		GrossMargin:     pct(grossProfit, revenue),
		OperatingMargin: pct(operatingProfit, revenue),
		CollectionRate:  pct(collected, revenue),
	}
}

// Material is one product's buy/sell rates for the margin table.
type Material struct {
	Name         string
	MaterialRate float64 // sell
	PurchaseRate float64 // buy
}

// Margin is one product's buy-vs-sell margin.
type Margin struct {
	Name          string  `json:"name"`
	SellRate      float64 `json:"sellRate"`
	BuyRate       float64 `json:"buyRate"`
	MarginPerUnit float64 `json:"marginPerUnit"`
	MarginPct     float64 `json:"marginPct"`
}

// ProductMargins ports computeProductMargins: richest margin first, products with no sell rate
// skipped (they have no meaningful margin and would divide by zero). A stable sort preserves
// input order among equal margins, matching V8's Array.sort.
func ProductMargins(materials []Material) []Margin {
	out := make([]Margin, 0, len(materials))
	for _, m := range materials {
		if m.MaterialRate <= 0 {
			continue
		}
		sell, buy := m.MaterialRate, m.PurchaseRate
		out = append(out, Margin{
			Name:          m.Name,
			SellRate:      roundMoney(sell),
			BuyRate:       roundMoney(buy),
			MarginPerUnit: roundMoney(sell - buy),
			MarginPct:     pct(sell-buy, sell),
		})
	}
	stableSortByMarginDesc(out)
	return out
}

// stableSortByMarginDesc is insertion sort - stable, and these lists are short (one per product).
func stableSortByMarginDesc(m []Margin) {
	for i := 1; i < len(m); i++ {
		for j := i; j > 0 && m[j-1].MarginPct < m[j].MarginPct; j-- {
			m[j-1], m[j] = m[j], m[j-1]
		}
	}
}

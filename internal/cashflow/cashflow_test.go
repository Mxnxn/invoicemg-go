package cashflow

import (
	"reflect"
	"testing"
)

// Values captured from Helpers/CashflowMetrics.test.js so the two can't drift.
func TestCompute_FullPeriod(t *testing.T) {
	got := Compute(Input{
		Invoices:         []Invoice{{TotalAmount: 10000, Amount: 10000}, {TotalAmount: 5000, Amount: 0}},
		Received:         []Amount{{8000}},
		BatchReceives:    []Amount{{2000}},
		SupplierPayments: []Amount{{3000}},
		PurchaseInvoices: []Amount{{4000}},
		SoldEntries:      []SoldEntry{{Qty: 10, PurchaseRate: 100}, {Qty: 5, PurchaseRate: 200}},
		Wastages:         []Wastage{{Total: 500}},
	})
	checks := map[string]struct{ got, want float64 }{
		"cashIn":          {got.CashIn, 10000},
		"cashOut":         {got.CashOut, 3000},
		"netCashflow":     {got.NetCashflow, 7000},
		"purchases":       {got.Purchases, 4000},
		"revenue":         {got.Revenue, 15000},
		"cogs":            {got.Cogs, 2000},
		"wastage":         {got.Wastage, 500},
		"grossProfit":     {got.GrossProfit, 13000},
		"operatingMargin": {got.OperatingMargin, 83.33},
		"operatingProfit": {got.OperatingProfit, 12500},
		"invoiced":        {got.Invoiced, 15000},
		"collected":       {got.Collected, 10000},
		"collectionRate":  {got.CollectionRate, 66.67},
	}
	for name, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", name, c.got, c.want)
		}
	}
}

func TestCompute_EmptyIsZeroNotNaN(t *testing.T) {
	got := Compute(Input{})
	for name, v := range map[string]float64{
		"revenue": got.Revenue, "operatingMargin": got.OperatingMargin, "collectionRate": got.CollectionRate,
		"netCashflow": got.NetCashflow, "grossProfit": got.GrossProfit,
	} {
		if v != 0 {
			t.Errorf("%s = %v, want 0", name, v)
		}
	}
}

func TestCompute_NegativeNet(t *testing.T) {
	got := Compute(Input{Received: []Amount{{1000}}, SupplierPayments: []Amount{{4000}}})
	if got.NetCashflow != -3000 {
		t.Errorf("netCashflow = %v, want -3000", got.NetCashflow)
	}
}

func TestCompute_LossIsNegativeMargin(t *testing.T) {
	got := Compute(Input{Invoices: []Invoice{{TotalAmount: 1000}}, SoldEntries: []SoldEntry{{Qty: 1, PurchaseRate: 1500}}})
	if got.GrossProfit != -500 {
		t.Errorf("grossProfit = %v, want -500", got.GrossProfit)
	}
	if got.OperatingMargin >= 0 {
		t.Errorf("a loss must show as a negative margin, got %v", got.OperatingMargin)
	}
}

func TestCompute_MissingRateNoCost(t *testing.T) {
	got := Compute(Input{
		Invoices:    []Invoice{{TotalAmount: 1000}},
		SoldEntries: []SoldEntry{{Qty: 2, PurchaseRate: 100}, {Qty: 3, PurchaseRate: 0}},
	})
	if got.Cogs != 200 {
		t.Errorf("cogs = %v, want 200", got.Cogs)
	}
}

func TestCompute_FloatDrift(t *testing.T) {
	got := Compute(Input{Received: []Amount{{0.1}, {0.2}}})
	if got.CashIn != 0.3 {
		t.Errorf("cashIn = %v, want 0.3", got.CashIn)
	}
}

func TestCompute_WastageCostBasis(t *testing.T) {
	if w := Compute(Input{Wastages: []Wastage{{Total: 900, CostTotal: 400}}}).Wastage; w != 400 {
		t.Errorf("cost basis should win: got %v, want 400", w)
	}
	if w := Compute(Input{Wastages: []Wastage{{Total: 900}}}).Wastage; w != 900 {
		t.Errorf("legacy fallback to total: got %v, want 900", w)
	}
	if w := Compute(Input{Wastages: []Wastage{{Total: 900, CostTotal: 0}}}).Wastage; w != 900 {
		t.Errorf("unset cost_total is not free: got %v, want 900", w)
	}
}

func TestProductMargins(t *testing.T) {
	rows := ProductMargins([]Material{
		{Name: "Vinyl", MaterialRate: 100, PurchaseRate: 60},
		{Name: "Flex", MaterialRate: 50, PurchaseRate: 45},
		{Name: "Board", MaterialRate: 200, PurchaseRate: 80},
	})
	if len(rows) != 3 {
		t.Fatalf("want 3, got %d", len(rows))
	}
	if rows[0].Name != "Board" || rows[0].MarginPct != 60 || rows[0].MarginPerUnit != 120 {
		t.Errorf("row0 = %+v", rows[0])
	}
	if rows[2].Name != "Flex" || rows[2].MarginPct != 10 {
		t.Errorf("row2 = %+v", rows[2])
	}
}

func TestProductMargins_SkipsNoSellRate(t *testing.T) {
	rows := ProductMargins([]Material{
		{Name: "Unpriced", MaterialRate: 0, PurchaseRate: 50},
		{Name: "Priced", MaterialRate: 100, PurchaseRate: 50},
	})
	if len(rows) != 1 || rows[0].Name != "Priced" {
		t.Errorf("want only Priced, got %+v", rows)
	}
}

func TestProductMargins_BelowCost(t *testing.T) {
	rows := ProductMargins([]Material{{Name: "Loss", MaterialRate: 100, PurchaseRate: 150}})
	if rows[0].MarginPct != -50 || rows[0].MarginPerUnit != -50 {
		t.Errorf("row = %+v", rows[0])
	}
}

func TestProductMargins_Empty(t *testing.T) {
	if got := ProductMargins(nil); !reflect.DeepEqual(got, []Margin{}) {
		t.Errorf("nil -> %+v, want empty slice", got)
	}
}

func TestRoundMoney(t *testing.T) {
	if roundMoney(0.1+0.2) != 0.3 {
		t.Errorf("0.1+0.2 -> %v", roundMoney(0.1+0.2))
	}
	if roundMoney(12.345) != 12.35 {
		t.Errorf("12.345 -> %v, want 12.35", roundMoney(12.345))
	}
}

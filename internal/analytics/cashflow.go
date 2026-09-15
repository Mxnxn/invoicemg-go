package analytics

import (
	"math"
	"net/http"
	"sort"
	"strings"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/cashflow"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

// Cashflow is POST /analytics/cashflow - the Cashflow tab: period totals, a monthly series,
// COGS coverage, and the best/worst product margins. Defaults to a trailing year when no
// [from,to] is given. All the date filtering, month bucketing and cost matching happens here;
// the store just hands over the company's raw rows and the pure cashflow package does the math.
func (h *Handler) Cashflow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)
	form, _ := httpx.ReadForm(r)

	now := h.now()
	to := form.String("to")
	if to == "" {
		to = now.Format("2006-01-02")
	}
	from := form.String("from")
	if from == "" {
		from = now.AddDate(0, 0, -365).Format("2006-01-02")
	}

	data, err := h.store.Cashflow(ctx, sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}

	// Cost basis is looked up by product name (entries carry only the sell rate). A renamed or
	// deleted product won't match; cogsCoverage reports how much of the sold volume we could cost.
	normKey := func(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
	costRate := map[string]float64{}
	for _, m := range data.Materials {
		costRate[normKey(m.Name)] = m.PurchaseRate
	}

	inRange := func(d string) bool { return d != "" && d >= from && d <= to }
	// soldInRange keeps the entry's raw fields so it can be re-bucketed by month; matched counts
	// the ones whose product we could find a cost for.
	type sold struct {
		date  string
		month string
		qty   float64
		rate  float64
	}
	var soldInRange []sold
	matched := 0
	for _, e := range data.SoldEntries {
		if !inRange(e.Date) {
			continue
		}
		rate, ok := costRate[normKey(e.Material)]
		if ok {
			matched++
		}
		soldInRange = append(soldInRange, sold{date: e.Date, month: monthOf(e.Date), qty: e.Qty, rate: rate})
	}

	// build assembles a cashflow.Input from the rows whose date passes keep().
	build := func(keep func(string) bool, entries []sold) cashflow.Input {
		in := cashflow.Input{}
		for _, iv := range data.Invoices {
			if keep(iv.Date) {
				in.Invoices = append(in.Invoices, cashflow.Invoice{TotalAmount: iv.TotalAmount, Amount: iv.Amount})
			}
		}
		in.Received = amountsWhere(data.Received, keep)
		in.BatchReceives = amountsWhere(data.BatchReceives, keep)
		in.SupplierPayments = amountsWhere(data.SupplierPayments, keep)
		in.PurchaseInvoices = amountsWhere(data.PurchaseInvoices, keep)
		for _, w := range data.Wastages {
			if keep(w.Date) {
				in.Wastages = append(in.Wastages, cashflow.Wastage{Total: w.Total, CostTotal: w.CostTotal})
			}
		}
		for _, e := range entries {
			in.SoldEntries = append(in.SoldEntries, cashflow.SoldEntry{Qty: e.qty, PurchaseRate: e.rate})
		}
		return in
	}

	totals := cashflow.Compute(build(inRange, soldInRange))

	// Monthly series, over the months any cash/invoice event falls in, within the window.
	monthSet := map[string]struct{}{}
	for _, iv := range data.Invoices {
		addMonth(monthSet, iv.Date, from, to)
	}
	for _, rows := range [][]store.CashflowRow{data.Received, data.BatchReceives, data.SupplierPayments} {
		for _, r := range rows {
			addMonth(monthSet, r.Date, from, to)
		}
	}
	months := make([]string, 0, len(monthSet))
	for m := range monthSet {
		months = append(months, m)
	}
	sort.Strings(months)

	series := make([]map[string]any, 0, len(months))
	for _, month := range months {
		inMonth := func(d string) bool { return monthOf(d) == month }
		var monthEntries []sold
		for _, e := range soldInRange {
			if e.month == month {
				monthEntries = append(monthEntries, e)
			}
		}
		m := cashflow.Compute(build(inMonth, monthEntries))
		series = append(series, map[string]any{
			"month": month, "cashIn": m.CashIn, "cashOut": m.CashOut,
			"netCashflow": m.NetCashflow, "revenue": m.Revenue, "operatingProfit": m.OperatingProfit,
		})
	}

	materials := make([]cashflow.Material, 0, len(data.Materials))
	for _, m := range data.Materials {
		materials = append(materials, cashflow.Material{Name: m.Name, MaterialRate: m.MaterialRate, PurchaseRate: m.PurchaseRate})
	}
	margins := cashflow.ProductMargins(materials)

	coverage := 100
	if len(soldInRange) > 0 {
		// Node's Math.round to the nearest whole percent (half rounds toward +Inf).
		coverage = int(math.Floor(float64(matched)/float64(len(soldInRange))*100 + 0.5))
	}

	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: map[string]any{
		"from": from, "to": to, "totals": totals, "series": series,
		"cogsCoverage": coverage,
		"bestMargins":  firstN(margins, 5),
		"worstMargins": lastNReversed(margins, 5),
	}})
}

// monthOf is JS `date.slice(0, 7)`: the YYYY-MM prefix, or the whole string if shorter.
func monthOf(d string) string {
	if len(d) >= 7 {
		return d[:7]
	}
	return d
}

func addMonth(set map[string]struct{}, date, from, to string) {
	m := monthOf(date)
	if m != "" && m >= monthOf(from) && m <= monthOf(to) {
		set[m] = struct{}{}
	}
}

func amountsWhere(rows []store.CashflowRow, keep func(string) bool) []cashflow.Amount {
	var out []cashflow.Amount
	for _, r := range rows {
		if keep(r.Date) {
			out = append(out, cashflow.Amount{Value: r.Amount})
		}
	}
	return out
}

func firstN(m []cashflow.Margin, n int) []cashflow.Margin {
	if len(m) < n {
		n = len(m)
	}
	return m[:n]
}

// lastNReversed is JS `arr.slice(-n).reverse()`: the last n, thinnest-margin first.
func lastNReversed(m []cashflow.Margin, n int) []cashflow.Margin {
	if len(m) < n {
		n = len(m)
	}
	tail := m[len(m)-n:]
	out := make([]cashflow.Margin, n)
	for i := range tail {
		out[n-1-i] = tail[i]
	}
	return out
}

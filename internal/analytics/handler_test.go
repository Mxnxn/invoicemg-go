package analytics

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type stubAnalytics struct {
	billed, collected []store.DatedAmount
	gotSource         string
	cashflow          store.CashflowData
}

func (s *stubAnalytics) Cashflow(_ context.Context, _ store.ID) (store.CashflowData, error) {
	return s.cashflow, nil
}

func (s *stubAnalytics) TopSales(_ context.Context, _ store.ID) ([]store.ClientRank, error) {
	return nil, nil
}
func (s *stubAnalytics) TopCredits(_ context.Context, _ store.ID) ([]store.ClientRank, error) {
	return nil, nil
}
func (s *stubAnalytics) TopPaid(_ context.Context, _ store.ID) ([]store.ClientRank, error) {
	return nil, nil
}

func (s *stubAnalytics) OutstandingInvoices(_ context.Context, _ store.ID) ([]store.DatedAmount, error) {
	return nil, nil
}
func (s *stubAnalytics) UnbilledEntries(_ context.Context, _ store.ID) ([]store.UnbilledEntry, error) {
	return nil, nil
}
func (s *stubAnalytics) Reviews(_ context.Context, _ store.ID, _, _ string) ([]store.ReviewRow, error) {
	return nil, nil
}
func (s *stubAnalytics) GstSales(_ context.Context, _ store.ID) ([]store.GstDoc, error) {
	return nil, nil
}
func (s *stubAnalytics) GstPurchases(_ context.Context, _ store.ID) ([]store.GstDoc, error) {
	return nil, nil
}
func (s *stubAnalytics) Receipts(_ context.Context, _ store.ID) ([]store.DatedAmount, error) {
	return nil, nil
}
func (s *stubAnalytics) PaymentGaps(_ context.Context, _ store.ID) ([]float64, error) {
	return nil, nil
}
func (s *stubAnalytics) PendingSince(_ context.Context, _ store.ID) ([]time.Time, error) {
	return nil, nil
}
func (s *stubAnalytics) Payables(_ context.Context, _ store.ID) (float64, []store.SupplierDue, error) {
	return 0, nil, nil
}
func (s *stubAnalytics) RevenueSeries(_ context.Context, _ store.ID, source string) ([]store.DatedAmount, []store.DatedAmount, error) {
	s.gotSource = source
	return s.billed, s.collected, nil
}

func post(t *testing.T, h *Handler, fields map[string]string) (int, map[string]any) {
	t.Helper()
	vals := url.Values{}
	for k, v := range fields {
		vals.Set(k, v)
	}
	r := httptest.NewRequest("POST", "/", strings.NewReader(vals.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	h.Revenue(rec, r)
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	return rec.Code, body
}

// A fixed clock so bucket labels are deterministic.
func fixedNow() *Handler {
	h := New(&stubAnalytics{})
	h.now = func() time.Time { return time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC) }
	return h
}

func TestRevenue_RequiresPeriod(t *testing.T) {
	code, body := post(t, fixedNow(), map[string]string{})
	if body["code"] != float64(422) {
		t.Errorf("code = %v, want 422 (missing period)", body["code"])
	}
	_ = code
}

func TestRevenue_MonthlyBucketsAndSums(t *testing.T) {
	sep := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	h := New(&stubAnalytics{
		billed:    []store.DatedAmount{{Date: sep, Amount: 11800}},
		collected: []store.DatedAmount{{Date: sep, Amount: 5000}},
	})
	h.now = func() time.Time { return time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC) }

	_, body := post(t, h, map[string]string{"period": "monthly"})
	if body["code"] != float64(200) {
		t.Fatalf("code = %v", body["code"])
	}
	data := body["data"].([]any)
	if len(data) != 36 {
		t.Fatalf("monthly buckets = %d, want 36", len(data))
	}
	last := data[len(data)-1].(map[string]any)
	if last["label"] != "Sep 2026" || last["billed"] != float64(11800) || last["collected"] != float64(5000) {
		t.Errorf("Sep bucket wrong: %v", last)
	}
	// yearAgo present for the trailing-window case
	if _, ok := body["yearAgo"].(map[string]any); !ok {
		t.Errorf("yearAgo should be present, got %v", body["yearAgo"])
	}
}

func TestRevenue_SourceDefaultsInvoiced(t *testing.T) {
	s := &stubAnalytics{}
	h := New(s)
	h.now = func() time.Time { return time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC) }
	post(t, h, map[string]string{"period": "monthly"})
	if s.gotSource != "invoiced" {
		t.Errorf("default source = %q, want invoiced", s.gotSource)
	}
	post(t, h, map[string]string{"period": "monthly", "source": "all"})
	if s.gotSource != "all" {
		t.Errorf("source = %q, want all", s.gotSource)
	}
}

func cashflowHandler(data store.CashflowData) *Handler {
	h := New(&stubAnalytics{cashflow: data})
	h.now = func() time.Time { return time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC) }
	return h
}

func postCashflow(t *testing.T, h *Handler, fields map[string]string) map[string]any {
	t.Helper()
	vals := url.Values{}
	for k, v := range fields {
		vals.Set(k, v)
	}
	r := httptest.NewRequest("POST", "/", strings.NewReader(vals.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	h.Cashflow(rec, r)
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("not json: %v\n%s", err, rec.Body.String())
	}
	return body
}

// The Node CashflowMetrics reference scenario, dated inside an explicit window, plus a materials
// table so cogs matching and margins are exercised end to end.
func TestCashflow_TotalsAndCoverage(t *testing.T) {
	data := store.CashflowData{
		Invoices: []store.CashflowInvoice{
			{Date: "2026-03-10", TotalAmount: 10000, Amount: 10000},
			{Date: "2026-03-20", TotalAmount: 5000, Amount: 0},
		},
		Received:         []store.CashflowRow{{Date: "2026-03-11", Amount: 8000}},
		BatchReceives:    []store.CashflowRow{{Date: "2026-03-12", Amount: 2000}},
		SupplierPayments: []store.CashflowRow{{Date: "2026-03-15", Amount: 3000}},
		PurchaseInvoices: []store.CashflowRow{{Date: "2026-03-05", Amount: 4000}},
		Wastages:         []store.CashflowWastage{{Date: "2026-03-01", Total: 500}},
		SoldEntries: []store.CashflowSoldEntry{
			{Date: "2026-03-10", Material: "Vinyl", Qty: 10}, // cost 100 -> 1000
			{Date: "2026-03-10", Material: "Board", Qty: 5},  // cost 200 -> 1000
			{Date: "2026-03-10", Material: "Ghost", Qty: 9},  // no material -> uncosted
		},
		Materials: []store.CashflowMaterial{
			{Name: "Vinyl", MaterialRate: 100, PurchaseRate: 100},
			{Name: "Board", MaterialRate: 500, PurchaseRate: 200},
		},
	}
	body := postCashflow(t, cashflowHandler(data), map[string]string{"from": "2026-01-01", "to": "2026-12-31"})
	if body["code"] != float64(200) {
		t.Fatalf("envelope: %v", body)
	}
	d := body["data"].(map[string]any)
	totals := d["totals"].(map[string]any)
	if totals["cashIn"] != float64(10000) || totals["cashOut"] != float64(3000) || totals["netCashflow"] != float64(7000) {
		t.Errorf("cash: %v", totals)
	}
	if totals["revenue"] != float64(15000) || totals["cogs"] != float64(2000) || totals["wastage"] != float64(500) {
		t.Errorf("revenue/cogs/wastage: %v", totals)
	}
	if totals["operatingProfit"] != float64(12500) || totals["operatingMargin"] != 83.33 {
		t.Errorf("operating: %v", totals)
	}
	// 2 of 3 sold entries could be costed -> round(66.66..) = 67
	if d["cogsCoverage"] != float64(67) {
		t.Errorf("cogsCoverage = %v, want 67", d["cogsCoverage"])
	}
	best := d["bestMargins"].([]any)
	if len(best) != 2 || best[0].(map[string]any)["name"] != "Board" {
		t.Errorf("bestMargins richest first: %v", best)
	}
	series := d["series"].([]any)
	if len(series) != 1 {
		t.Fatalf("all events in one month -> 1 series point, got %d: %v", len(series), series)
	}
	s0 := series[0].(map[string]any)
	if s0["month"] != "2026-03" || s0["revenue"] != float64(15000) || s0["netCashflow"] != float64(7000) {
		t.Errorf("series point: %v", s0)
	}
}

// No rows and no window: defaults to a trailing year off the fixed clock, empty totals, and
// coverage falls back to 100 rather than dividing by zero.
func TestCashflow_DefaultsAndEmpty(t *testing.T) {
	body := postCashflow(t, cashflowHandler(store.CashflowData{}), map[string]string{})
	d := body["data"].(map[string]any)
	if d["from"] != "2025-09-15" || d["to"] != "2026-09-15" {
		t.Errorf("default window = %v..%v, want 2025-09-15..2026-09-15", d["from"], d["to"])
	}
	if d["cogsCoverage"] != float64(100) {
		t.Errorf("empty coverage = %v, want 100", d["cogsCoverage"])
	}
	if len(d["series"].([]any)) != 0 {
		t.Errorf("no events -> empty series, got %v", d["series"])
	}
	if len(d["bestMargins"].([]any)) != 0 {
		t.Errorf("no materials -> empty margins, got %v", d["bestMargins"])
	}
}

// A month outside the window is dropped from the series even though it has events.
func TestCashflow_WindowExcludesOutOfRangeMonths(t *testing.T) {
	data := store.CashflowData{
		Received: []store.CashflowRow{
			{Date: "2026-02-10", Amount: 100}, // in
			{Date: "2026-08-10", Amount: 200}, // out of window
		},
	}
	body := postCashflow(t, cashflowHandler(data), map[string]string{"from": "2026-01-01", "to": "2026-03-31"})
	d := body["data"].(map[string]any)
	series := d["series"].([]any)
	if len(series) != 1 || series[0].(map[string]any)["month"] != "2026-02" {
		t.Errorf("only in-window month kept: %v", series)
	}
	if d["totals"].(map[string]any)["cashIn"] != float64(100) {
		t.Errorf("out-of-window receipt must not count: %v", d["totals"])
	}
}

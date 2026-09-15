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

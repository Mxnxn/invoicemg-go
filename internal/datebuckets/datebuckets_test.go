package datebuckets

import (
	"testing"
	"time"
)

func labels(bs []Bucket) []string {
	out := make([]string, len(bs))
	for i, b := range bs {
		out[i] = b.Label
	}
	return out
}

func eq(t *testing.T, name string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: len %d != %d (%v)", name, len(got), len(want), got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("%s[%d] = %q, want %q", name, i, got[i], want[i])
		}
	}
}

// Deterministic values captured from the real Node DateBuckets.js.
func TestMonthsForYear(t *testing.T) {
	eq(t, "monthsForYear", labels(MonthsForYear(2026)), []string{
		"Jan 2026", "Feb 2026", "Mar 2026", "Apr 2026", "May 2026", "Jun 2026",
		"Jul 2026", "Aug 2026", "Sep 2026", "Oct 2026", "Nov 2026", "Dec 2026",
	})
}

func TestWeeksForMonth(t *testing.T) {
	w := WeeksForMonth(2026, 8) // September, 0-indexed
	eq(t, "weeksForMonth", labels(w), []string{
		"Aug 31 - Sep 6", "Sep 7 - Sep 13", "Sep 14 - Sep 20", "Sep 21 - Sep 27", "Sep 28 - Oct 4",
	})
	if !w[0].Start.Equal(time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("first week start = %v", w[0].Start)
	}
	if !w[0].End.Equal(time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("first week end = %v", w[0].End)
	}
}

func TestSumIntoBuckets(t *testing.T) {
	buckets := MonthsForYear(2026)
	mk := func(s string) time.Time { d, _ := time.Parse("2006-01-02", s); return d.UTC() }
	items := []Dated{
		{Date: mk("2026-09-10"), Amount: 100},
		{Date: mk("2026-09-20"), Amount: 50},
		{Date: mk("2026-01-05"), Amount: 7},
		{Date: mk("2025-12-31"), Amount: 999}, // outside 2026 -> ignored
	}
	sums := SumIntoBuckets(items, buckets)
	want := []float64{7, 0, 0, 0, 0, 0, 0, 0, 150, 0, 0, 0}
	for i := range want {
		if sums[i] != want[i] {
			t.Errorf("sums[%d] = %v, want %v", i, sums[i], want[i])
		}
	}
}

func TestEffectiveDate(t *testing.T) {
	created := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	// createdAt wins over date
	if d, ok := EffectiveDate(&created, "2020-01-01"); !ok || !d.Equal(created) {
		t.Errorf("createdAt should win: %v %v", d, ok)
	}
	// date fallback
	if d, ok := EffectiveDate(nil, "2026-09-10"); !ok || !d.Equal(time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("date fallback wrong: %v %v", d, ok)
	}
	// neither
	if _, ok := EffectiveDate(nil, ""); ok {
		t.Error("no date should be not-ok")
	}
}

// The now-relative builders: structural checks (count, contiguity, span) plus the last bucket
// containing now - the same shape as the Node test, which cannot pin labels either.
func TestWeeklyBuckets_Structure(t *testing.T) {
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	bs := WeeklyBuckets(now, 8)
	if len(bs) != 8 {
		t.Fatalf("len = %d, want 8", len(bs))
	}
	for i, b := range bs {
		if b.End.Sub(b.Start) != week {
			t.Errorf("bucket %d not 7 days", i)
		}
		if i > 0 && !bs[i-1].End.Equal(b.Start) {
			t.Errorf("buckets %d/%d not contiguous", i-1, i)
		}
	}
	last := bs[len(bs)-1]
	if now.Before(last.Start) || !now.Before(last.End) {
		t.Errorf("now not in last weekly bucket")
	}
}

func TestMonthlyBuckets_LastIsNowMonth(t *testing.T) {
	now := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	bs := MonthlyBuckets(now, 36)
	if len(bs) != 36 {
		t.Fatalf("len = %d, want 36", len(bs))
	}
	if bs[len(bs)-1].Label != "Sep 2026" {
		t.Errorf("last monthly label = %q, want Sep 2026", bs[len(bs)-1].Label)
	}
}

func TestYearlyBuckets(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	bs := YearlyBuckets(now, 3, 0)
	eq(t, "yearly", labels(bs), []string{"2024", "2025", "2026"})
}

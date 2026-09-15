// Package datebuckets ports Helpers/DateBuckets.js - the weekly/monthly/yearly bucketing that
// Analytics and Statistics both read. All arithmetic is UTC, matching the Node helper (which
// uses Date.UTC throughout), so a server in another timezone cannot shift a bucket boundary.
//
// The now-relative builders take an explicit `now` rather than calling time.Now, so a test can
// pin the clock and assert exact labels; the Node original reads new Date() internally.
package datebuckets

import (
	"fmt"
	"time"
)

var monthNames = []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

// Bucket is a half-open date range [Start, End) with a display label.
type Bucket struct {
	Start time.Time
	End   time.Time
	Label string
}

func utcDay(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// FormatShortDate is "Jan 2" - month abbreviation + day of month, UTC.
func FormatShortDate(t time.Time) string {
	t = t.UTC()
	return fmt.Sprintf("%s %d", monthNames[int(t.Month())-1], t.Day())
}

// StartOfISOWeek returns the Monday of t's ISO week, at UTC midnight.
func StartOfISOWeek(t time.Time) time.Time {
	d := utcDay(t)
	day := int(d.Weekday()) // Sunday=0
	if day == 0 {
		day = 7
	}
	if day != 1 {
		d = d.AddDate(0, 0, -day+1)
	}
	return d
}

const week = 7 * 24 * time.Hour

func weekBucket(start time.Time) Bucket {
	end := start.Add(week)
	last := end.Add(-24 * time.Hour)
	return Bucket{Start: start, End: end, Label: FormatShortDate(start) + " - " + FormatShortDate(last)}
}

// WeeklyBuckets returns `count` contiguous ISO-week buckets ending with the week containing now.
func WeeklyBuckets(now time.Time, count int) []Bucket {
	currentStart := StartOfISOWeek(now)
	out := make([]Bucket, 0, count)
	for i := count - 1; i >= 0; i-- {
		out = append(out, weekBucket(currentStart.Add(-time.Duration(i)*week)))
	}
	return out
}

func monthStart(year int, month time.Month) time.Time {
	return time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
}

// MonthlyBuckets returns `count` month buckets ending with now's month.
func MonthlyBuckets(now time.Time, count int) []Bucket {
	now = now.UTC()
	out := make([]Bucket, 0, count)
	for i := count - 1; i >= 0; i-- {
		start := monthStart(now.Year(), now.Month()).AddDate(0, -i, 0)
		end := start.AddDate(0, 1, 0)
		out = append(out, Bucket{Start: start, End: end, Label: fmt.Sprintf("%s %d", monthNames[int(start.Month())-1], start.Year())})
	}
	return out
}

// YearlyBuckets returns `count` year buckets ending with (now.year - offset).
func YearlyBuckets(now time.Time, count, offset int) []Bucket {
	currentYear := now.UTC().Year() - offset
	out := make([]Bucket, 0, count)
	for i := count - 1; i >= 0; i-- {
		year := currentYear - i
		out = append(out, Bucket{
			Start: time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC),
			End:   time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC),
			Label: fmt.Sprintf("%d", year),
		})
	}
	return out
}

// MonthsForYear returns the twelve month buckets of a year.
func MonthsForYear(year int) []Bucket {
	out := make([]Bucket, 0, 12)
	for m := 1; m <= 12; m++ {
		start := monthStart(year, time.Month(m))
		out = append(out, Bucket{Start: start, End: start.AddDate(0, 1, 0), Label: fmt.Sprintf("%s %d", monthNames[m-1], year)})
	}
	return out
}

// WeeksForMonth returns every ISO week overlapping a month. month is 0-indexed, as in the Node
// helper (0 = January), so callers pass the same value they would in JavaScript.
func WeeksForMonth(year, month int) []Bucket {
	monthStartT := time.Date(year, time.Month(month+1), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := time.Date(year, time.Month(month+2), 1, 0, 0, 0, 0, time.UTC)
	out := []Bucket{}
	for weekStart := StartOfISOWeek(monthStartT); weekStart.Before(monthEnd); weekStart = weekStart.Add(week) {
		out = append(out, weekBucket(weekStart))
	}
	return out
}

// EffectiveDate is getEffectiveDate: createdAt wins, then date, then not-ok. createdAt is the
// stamped timestamp (a pointer, nil when unset); date is the typed YYYY-MM-DD string.
func EffectiveDate(createdAt *time.Time, date string) (time.Time, bool) {
	if createdAt != nil && !createdAt.IsZero() {
		return createdAt.UTC(), true
	}
	if date != "" {
		if d, err := time.Parse("2006-01-02", date); err == nil {
			return d.UTC(), true
		}
		// Node's new Date(str) also accepts full ISO; accept it too.
		if d, err := time.Parse(time.RFC3339, date); err == nil {
			return d.UTC(), true
		}
	}
	return time.Time{}, false
}

// Dated is one amount with its effective date already resolved, for SumIntoBuckets.
type Dated struct {
	Date   time.Time
	Amount float64
}

// SumIntoBuckets totals amounts into the bucket whose [start,end) contains each date.
func SumIntoBuckets(items []Dated, buckets []Bucket) []float64 {
	sums := make([]float64, len(buckets))
	for _, it := range items {
		t := it.Date.UnixNano()
		for i, b := range buckets {
			if t >= b.Start.UnixNano() && t < b.End.UnixNano() {
				sums[i] += it.Amount
				break
			}
		}
	}
	return sums
}

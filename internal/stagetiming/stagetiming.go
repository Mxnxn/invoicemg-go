// Package stagetiming ports the pure parts of Helpers/StageTiming.js used by the production
// analytics: parsing "Queue advanced" history details, folding stage names to the reporting
// vocabulary, age bucketing, and the median/percentile roll-ups. Pure - no store, no clock it is
// not handed - the same shape as datebuckets/jobmath.
package stagetiming

import (
	"math"
	"regexp"
	"sort"
	"strings"
)

// DefaultStages is the four-stage default pipeline; anything else folds to OtherStage.
var DefaultStages = []string{"Created", "Printing", "Ready-to-Pickup", "Done"}

const OtherStage = "Other stages"

// AgeBuckets are the WIP dwell buckets, in order.
var AgeBuckets = []string{"0-2d", "3-7d", "8-14d", "15+d"}

// MsPerDay matches Node's MS_PER_DAY (used against millisecond deltas).
const MsPerDay = float64(24 * 60 * 60 * 1000)

var (
	// "Row <id>: <from> → <to>" - the per-row detail flavour.
	rowDetail = regexp.MustCompile(`^Row\s+(.+?):\s*(.+?)\s*→\s*(.+?)\s*$`)
	// "<from> → <to>" - the job-level flavour and everything before per-row tracking.
	jobDetail = regexp.MustCompile(`^\s*(.+?)\s*→\s*(.+?)\s*$`)
)

// QueueAdvance is a parsed "Queue advanced" detail. RowKey is opaque (an invoice-style id or a raw
// ObjectId) - only a stable per-job grouping key. Ok is false when the detail is not parseable.
type QueueAdvance struct {
	RowKey string
	From   string
	To     string
	Ok     bool
}

// ParseQueueAdvance parses a "Queue advanced" detail string. A string without the arrow is not one.
func ParseQueueAdvance(detail string) QueueAdvance {
	if !strings.Contains(detail, "→") {
		return QueueAdvance{}
	}
	if m := rowDetail.FindStringSubmatch(detail); m != nil {
		return QueueAdvance{RowKey: m[1], From: m[2], To: m[3], Ok: true}
	}
	if m := jobDetail.FindStringSubmatch(detail); m != nil {
		return QueueAdvance{RowKey: "", From: m[1], To: m[2], Ok: true}
	}
	return QueueAdvance{}
}

// FoldStage collapses a stage name to the reporting vocabulary (case-insensitive match against the
// defaults), or OtherStage. Custom and legacy stages are counted, never dropped.
func FoldStage(name string) string {
	trimmed := strings.TrimSpace(name)
	for _, s := range DefaultStages {
		if strings.EqualFold(s, trimmed) {
			return s
		}
	}
	return OtherStage
}

// AgeBucket picks the dwell bucket for a day count.
func AgeBucket(days float64) string {
	switch {
	case days < 3:
		return AgeBuckets[0]
	case days < 8:
		return AgeBuckets[1]
	case days < 15:
		return AgeBuckets[2]
	default:
		return AgeBuckets[3]
	}
}

// Round1 is Math.round(n*10)/10 (half up; production values are non-negative).
func Round1(n float64) float64 { return math.Floor(n*10+0.5) / 10 }

// Percentile is the linear-interpolation percentile over finite values; nil (JS null) when empty.
func Percentile(nums []float64, p float64) *float64 {
	sorted := make([]float64, 0, len(nums))
	for _, n := range nums {
		if !math.IsInf(n, 0) && !math.IsNaN(n) {
			sorted = append(sorted, n)
		}
	}
	if len(sorted) == 0 {
		return nil
	}
	sort.Float64s(sorted)
	if len(sorted) == 1 {
		return &sorted[0]
	}
	rank := (p / 100) * float64(len(sorted)-1)
	low := int(math.Floor(rank))
	high := int(math.Ceil(rank))
	if low == high {
		v := sorted[low]
		return &v
	}
	v := Round1(sorted[low] + (sorted[high]-sorted[low])*(rank-float64(low)))
	return &v
}

// Median is the 50th percentile.
func Median(nums []float64) *float64 { return Percentile(nums, 50) }

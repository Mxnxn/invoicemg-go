package alerts

import (
	"math"
	"strconv"

	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

// reviewDimensions and the score bounds are Helpers/ReviewScores.js, in its order - the order
// matters because the first missing dimension is the one named in the error, and a test pins
// that quality is checked first.
var reviewDimensions = []string{"quality", "speed", "communication", "satisfaction", "overall"}

const (
	maxComment = 1000
	minScore   = 1
	maxScore   = 5
)

// parseScore is Node's parseScore: a whole number from 1 to 5, refusing anything else - a blank
// field, a decimal, and 0 in particular, because 0 is what an untouched star row submits and it
// is not a rating. Number.isInteger("4.5") is false, so a decimal is refused; the float parse
// plus the Trunc check reproduces that.
func parseScore(raw string) (int, bool) {
	if raw == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil || f != math.Trunc(f) {
		return 0, false
	}
	n := int(f)
	if n < minScore || n > maxScore {
		return 0, false
	}
	return n, true
}

// parseScores validates a submitted review, returning the scores and comment, or ok=false with
// the message Node sends. All five dimensions are required: a review missing one is an
// unfinished form, and storing it would drag that dimension's average down by a rating nobody
// gave. The comment is capped, not rejected.
func parseScores(form *httpx.Form) (store.ReviewScores, string, bool, string) {
	got := map[string]int{}
	for _, dim := range reviewDimensions {
		n, ok := parseScore(form.String(dim))
		if !ok {
			return store.ReviewScores{}, "", false, "Please give a rating from 1 to 5 for " + dim + "."
		}
		got[dim] = n
	}
	return store.ReviewScores{
		Quality:       got["quality"],
		Speed:         got["speed"],
		Communication: got["communication"],
		Satisfaction:  got["satisfaction"],
		Overall:       got["overall"],
	}, truncateRunes(form.String("comment"), maxComment), true, ""
}

// truncateRunes caps a string's length. Node slices by UTF-16 code unit and this by rune, which
// differ only past the BMP (an emoji), on a field already capped rather than rejected - close
// enough that a divergence cannot change whether a review is accepted.
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

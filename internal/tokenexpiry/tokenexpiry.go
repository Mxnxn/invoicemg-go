// Package tokenexpiry ports Helpers/TokenExpiry.js: resolve a registration token's expiry from an
// optional yyyymmdd string, defaulting to 24 hours. The token expires at the END of the chosen day.
package tokenexpiry

import (
	"regexp"
	"strconv"
	"time"
)

const (
	defaultTTL = 24 * time.Hour
	maxDays    = 90
)

var yyyymmdd = regexp.MustCompile(`^\d{8}$`)

// Resolve returns the expiry time (ok=true) or a reason (ok=false). raw is a yyyymmdd string, or
// empty for the 24-hour default.
func Resolve(raw string, now time.Time) (time.Time, bool, string) {
	if raw == "" {
		return now.Add(defaultTTL), true, ""
	}
	if !yyyymmdd.MatchString(raw) {
		return time.Time{}, false, "Expiry must be yyyymmdd, for example 20260910."
	}
	y, _ := strconv.Atoi(raw[0:4])
	m, _ := strconv.Atoi(raw[4:6])
	d, _ := strconv.Atoi(raw[6:8])
	end := time.Date(y, time.Month(m), d, 23, 59, 59, int(999*time.Millisecond), time.Local)
	// time.Date normalises out-of-range dates (Feb 30 -> Mar 2), so a mismatch means it did not exist.
	if end.Year() != y || int(end.Month()) != m || end.Day() != d {
		return time.Time{}, false, "That date does not exist."
	}
	if !end.After(now) {
		return time.Time{}, false, "Expiry must be in the future."
	}
	if end.After(now.Add(maxDays * 24 * time.Hour)) {
		return time.Time{}, false, "Expiry cannot be more than 90 days away."
	}
	return end, true, ""
}

package httpx

import (
	"strings"
	"time"
)

// Time is a time.Time that serialises the way JavaScript's Date.toJSON does.
//
// This exists because Go and JavaScript disagree, and the disagreement is invisible until it
// is not:
//
//	Node   {"createdAt":"2026-09-14T12:34:56.000Z"}
//	Go     {"createdAt":"2026-09-14T12:34:56Z"}
//
// Date.prototype.toJSON always emits exactly three decimal places. Go's time.Time marshals as
// RFC3339Nano, which TRIMS trailing zeros - so the two agree whenever the millisecond part is
// non-zero and differ whenever it is zero. That is roughly one timestamp in a thousand by
// chance, and 100% of the time for any date stored as midnight: every `receivedDate` promoted
// to a Date, every day boundary, every date an admin typed without a time.
//
// A client that merely displays the string will not care. One that compares it, uses it as a
// key, sorts it lexically, or round-trips it back to the API will, and the failure surfaces
// far away from the cause. There are 18 Date fields across 10 models in the Node schema, so
// this needs to be systematic rather than remembered per handler.
//
// Use this type for every field that comes back from a Date column, never time.Time directly.
type Time time.Time

// jsLayout is RFC3339 with milliseconds forced to three places - what Date.toJSON emits.
// The zone is written as Z rather than +00:00 because JavaScript always serialises UTC.
const jsLayout = "2006-01-02T15:04:05.000Z"

func (t Time) MarshalJSON() ([]byte, error) {
	// The zero time serialises as null, matching a Mongoose field that was never set: Node
	// sends null there, and "0001-01-01T00:00:00.000Z" would be read by a client as a real
	// date in the year 1.
	if time.Time(t).IsZero() {
		return []byte("null"), nil
	}
	// UTC first: Mongo stores UTC, and a server running in another zone would otherwise
	// serialise a different wall clock than Node does from the same document.
	return []byte(`"` + time.Time(t).UTC().Format(jsLayout) + `"`), nil
}

func (t *Time) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "null" || s == "" {
		*t = Time(time.Time{})
		return nil
	}
	// Parsed with RFC3339 rather than jsLayout, because an incoming value may legitimately
	// carry no fractional part or a non-UTC offset - we are strict about what we SEND and
	// liberal about what we accept.
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return err
	}
	*t = Time(parsed)
	return nil
}

func (t Time) Time() time.Time { return time.Time(t) }

// NewTime wraps a time.Time, and NewTimePtr keeps a nil pointer nil so an unset optional field
// stays absent rather than becoming the zero time.
func NewTime(t time.Time) Time { return Time(t) }

func NewTimePtr(t *time.Time) *Time {
	if t == nil {
		return nil
	}
	v := Time(*t)
	return &v
}

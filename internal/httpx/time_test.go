package httpx

import (
	"encoding/json"
	"testing"
	"time"
)

// The expected strings here are not guesses - they are what `node -e` prints for the same
// instants, captured when this type was written:
//
//	> JSON.stringify({t: new Date(Date.UTC(2026,8,14,12,34,56,0))})
//	'{"t":"2026-09-14T12:34:56.000Z"}'
//	> JSON.stringify({t: new Date(Date.UTC(2026,8,14,12,34,56,789))})
//	'{"t":"2026-09-14T12:34:56.789Z"}'
func TestMatchesJavaScriptDateToJSON(t *testing.T) {
	cases := []struct {
		name string
		in   time.Time
		want string
	}{
		{
			// The one that matters. Plain time.Time marshals this as "2026-09-14T12:34:56Z",
			// with no fractional part, because RFC3339Nano trims trailing zeros.
			name: "zero milliseconds still writes .000",
			in:   time.Date(2026, 9, 14, 12, 34, 56, 0, time.UTC),
			want: `"2026-09-14T12:34:56.000Z"`,
		},
		{
			name: "non-zero milliseconds",
			in:   time.Date(2026, 9, 14, 12, 34, 56, 789000000, time.UTC),
			want: `"2026-09-14T12:34:56.789Z"`,
		},
		{
			// Midnight is the common case, not an edge one: every date an admin typed with no
			// time lands here, so a plain time.Time would differ from Node on most of them.
			name: "midnight",
			in:   time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC),
			want: `"2026-09-14T00:00:00.000Z"`,
		},
		{
			// Sub-millisecond precision is truncated, not rounded - JavaScript has no
			// nanoseconds, so anything finer cannot survive the trip either way.
			name: "nanoseconds truncate to milliseconds",
			in:   time.Date(2026, 9, 14, 12, 34, 56, 789999999, time.UTC),
			want: `"2026-09-14T12:34:56.789Z"`,
		},
		{
			// A server in another zone must still send what Mongo holds, or the same document
			// serialises differently depending on where the container runs.
			name: "non-UTC input is converted",
			in:   time.Date(2026, 9, 14, 12, 34, 56, 0, time.FixedZone("IST", 5*3600+1800)),
			want: `"2026-09-14T07:04:56.000Z"`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := json.Marshal(Time(c.in))
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(got) != c.want {
				t.Fatalf("got %s, want %s", got, c.want)
			}
		})
	}
}

// An unset Mongoose date comes back as null. The zero time would otherwise serialise as a
// real date in the year 1, which a client would happily render.
func TestZeroTimeIsNull(t *testing.T) {
	got, err := json.Marshal(Time(time.Time{}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(got) != "null" {
		t.Fatalf("got %s, want null", got)
	}
}

func TestNewTimePtrKeepsNilNil(t *testing.T) {
	if NewTimePtr(nil) != nil {
		t.Fatal("a nil date became non-nil, so an unset field would serialise as a value")
	}
	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	if NewTimePtr(&now) == nil {
		t.Fatal("a set date became nil")
	}
}

// Strict about what we send, liberal about what we accept: an incoming value may carry no
// fractional part or a real offset.
func TestUnmarshalAcceptsBothForms(t *testing.T) {
	for _, in := range []string{
		`"2026-09-14T12:34:56.000Z"`,
		`"2026-09-14T12:34:56Z"`,
		`"2026-09-14T18:04:56+05:30"`,
	} {
		var got Time
		if err := json.Unmarshal([]byte(in), &got); err != nil {
			t.Fatalf("unmarshal %s: %v", in, err)
		}
		if !got.Time().UTC().Equal(time.Date(2026, 9, 14, 12, 34, 56, 0, time.UTC)) {
			t.Fatalf("%s parsed to %v", in, got.Time())
		}
	}
}

// Round-tripping must be stable, or a value read from the API and sent back changes shape.
func TestRoundTrip(t *testing.T) {
	original := Time(time.Date(2026, 9, 14, 12, 34, 56, 0, time.UTC))
	b, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back Time
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	again, err := json.Marshal(back)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if string(b) != string(again) {
		t.Fatalf("round trip changed %s into %s", b, again)
	}
}

package alerts

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/mxnxn/invoicemg-go/internal/httpx"
)

func scoreForm(t *testing.T, fields map[string]string) *httpx.Form {
	t.Helper()
	vals := url.Values{}
	for k, v := range fields {
		vals.Set(k, v)
	}
	req := httptest.NewRequest("POST", "/", strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	f, err := httpx.ReadForm(req)
	if err != nil {
		t.Fatalf("ReadForm: %v", err)
	}
	return f
}

// Every case is what the real Helpers/ReviewScores.js parseScores returned for the same input.
func TestParseScores(t *testing.T) {
	base := map[string]string{"quality": "5", "speed": "4", "communication": "5", "satisfaction": "4", "overall": "5"}

	with := func(m map[string]string, k, v string) map[string]string {
		out := map[string]string{}
		for kk, vv := range m {
			out[kk] = vv
		}
		out[k] = v
		return out
	}

	t.Run("valid trims comment", func(t *testing.T) {
		s, comment, ok, msg := parseScores(scoreForm(t, with(base, "comment", "  Great work  ")))
		if !ok {
			t.Fatalf("ok=false msg=%q", msg)
		}
		if s.Quality != 5 || s.Speed != 4 || s.Communication != 5 || s.Satisfaction != 4 || s.Overall != 5 {
			t.Errorf("scores wrong: %+v", s)
		}
		if comment != "Great work" {
			t.Errorf("comment = %q, want trimmed", comment)
		}
	})

	t.Run("whitespace score accepted", func(t *testing.T) {
		s, _, ok, _ := parseScores(scoreForm(t, with(base, "quality", " 3 ")))
		if !ok || s.Quality != 3 {
			t.Errorf("expected quality 3, got %+v ok=%v", s, ok)
		}
	})

	rejects := []struct {
		name, field, value, wantDim string
	}{
		{"missing", "speed", "", "speed"},
		{"zero", "communication", "0", "communication"},
		{"half", "satisfaction", "4.5", "satisfaction"},
		{"six", "overall", "6", "overall"},
	}
	for _, r := range rejects {
		t.Run(r.name, func(t *testing.T) {
			_, _, ok, msg := parseScores(scoreForm(t, with(base, r.field, r.value)))
			if ok {
				t.Fatalf("expected rejection")
			}
			want := "Please give a rating from 1 to 5 for " + r.wantDim + "."
			if msg != want {
				t.Errorf("message = %q, want %q", msg, want)
			}
		})
	}

	t.Run("empty body names quality first", func(t *testing.T) {
		_, _, ok, msg := parseScores(scoreForm(t, map[string]string{}))
		if ok || msg != "Please give a rating from 1 to 5 for quality." {
			t.Errorf("ok=%v msg=%q", ok, msg)
		}
	})
}

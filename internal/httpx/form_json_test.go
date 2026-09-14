package httpx

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// formWith builds a *Form from urlencoded fields, the way ReadForm sees a real request.
// urlencoding round-trips through ParseForm, so whitespace and JSON punctuation survive.
func formWith(t *testing.T, fields map[string]string) *Form {
	t.Helper()
	vals := url.Values{}
	for k, v := range fields {
		vals.Set(k, v)
	}
	req := httptest.NewRequest("POST", "/", strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	f, err := ReadForm(req)
	if err != nil {
		t.Fatalf("ReadForm: %v", err)
	}
	return f
}

type row struct {
	N int `json:"n"`
}

// JSON is the strict form: mirrors JSON.parse(req.body.name). Absent, empty and malformed all
// throw in JavaScript, so all three must error here.
func TestFormJSON_Strict(t *testing.T) {
	t.Run("valid array", func(t *testing.T) {
		f := formWith(t, map[string]string{"rows": `[{"n":1},{"n":2}]`})
		var rows []row
		if err := f.JSON("rows", &rows); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rows) != 2 || rows[0].N != 1 || rows[1].N != 2 {
			t.Fatalf("got %+v", rows)
		}
	})

	t.Run("absent errors (JSON.parse(undefined) throws)", func(t *testing.T) {
		f := formWith(t, map[string]string{})
		var rows []row
		if err := f.JSON("rows", &rows); err == nil {
			t.Fatal("absent field must error")
		}
	})

	t.Run("empty errors (JSON.parse(\"\") throws)", func(t *testing.T) {
		f := formWith(t, map[string]string{"rows": ""})
		var rows []row
		if err := f.JSON("rows", &rows); err == nil {
			t.Fatal("empty field must error")
		}
	})

	t.Run("malformed errors", func(t *testing.T) {
		f := formWith(t, map[string]string{"rows": "[{"})
		var rows []row
		if err := f.JSON("rows", &rows); err == nil {
			t.Fatal("malformed field must error")
		}
	})
}

// JSONOr is the lenient form: mirrors JSON.parse(req.body.name || "[]"). Absent and empty fall
// back and do not error; a non-empty malformed value still errors, because a non-empty string
// is truthy in JavaScript and reaches JSON.parse.
func TestFormJSON_Or(t *testing.T) {
	t.Run("absent falls back to []", func(t *testing.T) {
		f := formWith(t, map[string]string{})
		var alloc []int
		if err := f.JSONOr("allocations", "[]", &alloc); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(alloc) != 0 {
			t.Fatalf("expected empty, got %v", alloc)
		}
	})

	t.Run("empty falls back to []", func(t *testing.T) {
		f := formWith(t, map[string]string{"allocations": ""})
		var alloc []int
		if err := f.JSONOr("allocations", "[]", &alloc); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(alloc) != 0 {
			t.Fatalf("expected empty, got %v", alloc)
		}
	})

	t.Run("valid parses", func(t *testing.T) {
		f := formWith(t, map[string]string{"allocations": "[1,2,3]"})
		var alloc []int
		if err := f.JSONOr("allocations", "[]", &alloc); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(alloc) != 3 || alloc[2] != 3 {
			t.Fatalf("got %v", alloc)
		}
	})

	t.Run("non-empty malformed still errors", func(t *testing.T) {
		f := formWith(t, map[string]string{"allocations": "nonsense"})
		var alloc []int
		if err := f.JSONOr("allocations", "[]", &alloc); err == nil {
			t.Fatal("non-empty malformed value must error")
		}
	})

	t.Run("whitespace-only errors, not treated as empty", func(t *testing.T) {
		// "  " is truthy in JS, so JSON.parse("  ") throws - it must NOT be trimmed to "" and
		// swallowed by the fallback.
		f := formWith(t, map[string]string{"allocations": "  "})
		var alloc []int
		if err := f.JSONOr("allocations", "[]", &alloc); err == nil {
			t.Fatal("whitespace-only value must error (raw, not trimmed)")
		}
	})
}

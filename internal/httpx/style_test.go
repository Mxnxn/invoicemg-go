package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func restore(t *testing.T) {
	t.Helper()
	previous := Style()
	t.Cleanup(func() { SetStyle(previous) })
}

// The legacy contract, which exists because the React client reads data.code and never
// res.status. A real 401 here would be read by it as a successful empty response.
func TestLegacyStyleAlwaysAnswers200(t *testing.T) {
	restore(t)
	SetStyle(StyleLegacy)

	for _, code := range []int{200, 401, 403, 404, 409, 422, 500} {
		rec := httptest.NewRecorder()
		Write(rec, Envelope{Code: code, Message: "x"})
		if rec.Code != http.StatusOK {
			t.Errorf("code %d answered HTTP %d, want 200 under legacy", code, rec.Code)
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("not json: %v", err)
		}
		if body["code"] != float64(code) {
			t.Errorf("body code = %v, want %d", body["code"], code)
		}
	}
}

// The correct surface. The status line carries the outcome AND the body keeps `code`, so a
// client can be moved across one call at a time rather than all at once.
func TestRESTStyleUsesTheStatusLine(t *testing.T) {
	restore(t)
	SetStyle(StyleREST)

	for _, code := range []int{200, 401, 403, 404, 409, 422, 500} {
		rec := httptest.NewRecorder()
		Write(rec, Envelope{Code: code, Message: "x"})
		if rec.Code != code {
			t.Errorf("code %d answered HTTP %d, want %d under rest", code, rec.Code, code)
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("not json: %v", err)
		}
		if body["code"] != float64(code) {
			t.Errorf("body lost its code under rest: %v", body["code"])
		}
	}
}

// A code nobody recognises becomes 500 rather than being passed through, so a typo in a
// handler cannot invent an HTTP status.
func TestUnknownCodeBecomes500(t *testing.T) {
	restore(t)
	SetStyle(StyleREST)

	rec := httptest.NewRecorder()
	Write(rec, Envelope{Code: 799, Message: "typo"})
	if rec.Code != 500 {
		t.Fatalf("HTTP %d, want 500", rec.Code)
	}
}

// Anything that is not exactly "rest" is legacy. The default has to be the safe one: a
// mistyped API_STYLE must not silently break the live client.
func TestUnknownStyleFallsBackToLegacy(t *testing.T) {
	restore(t)
	SetStyle("REST")
	if Style() != StyleLegacy {
		t.Fatalf("style = %q, want legacy for an unrecognised value", Style())
	}
	SetStyle("")
	if Style() != StyleLegacy {
		t.Fatalf("style = %q, want legacy for an empty value", Style())
	}
}

func TestHelpersCarryTheirCodes(t *testing.T) {
	restore(t)
	SetStyle(StyleREST)

	cases := []struct {
		name string
		call func(w http.ResponseWriter)
		want int
	}{
		{"unauthorized", func(w http.ResponseWriter) { Unauthorized(w, "") }, 401},
		{"forbidden", func(w http.ResponseWriter) { Forbidden(w) }, 403},
		{"invalid", func(w http.ResponseWriter) { Invalid(w, "") }, 422},
		{"internal", func(w http.ResponseWriter) { Internal(w, nil) }, 500},
		{"ok", func(w http.ResponseWriter) { OK(w, []string{}) }, 200},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c.call(rec)
			if rec.Code != c.want {
				t.Fatalf("HTTP %d, want %d", rec.Code, c.want)
			}
		})
	}
}

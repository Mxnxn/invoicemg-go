package httpx

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

// Raw sends the body and content type verbatim with the given status, whatever the style.
func TestRaw(t *testing.T) {
	rec := httptest.NewRecorder()
	Raw(rec, 200, "text/html; charset=utf-8", []byte("CHALLENGE_123"))

	if rec.Code != 200 {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != "CHALLENGE_123" {
		t.Errorf("body = %q, want the bare challenge", got)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("content-type = %q", ct)
	}
}

// Status is res.sendStatus: the code plus its standard message as text, the shape the webhook
// POST uses to ack (200) and reject (403/500).
func TestStatus(t *testing.T) {
	cases := map[int]string{200: "OK", 403: "Forbidden", 500: "Internal Server Error"}
	for code, want := range cases {
		rec := httptest.NewRecorder()
		Status(rec, code)
		if rec.Code != code {
			t.Errorf("Status(%d): code = %d", code, rec.Code)
		}
		if got := rec.Body.String(); got != want {
			t.Errorf("Status(%d): body = %q, want %q", code, got, want)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
			t.Errorf("Status(%d): content-type = %q", code, ct)
		}
	}
}

// WriteStatus forces the HTTP status even under legacy, where Write would answer 200. The body
// is still the envelope, so the client that reads data.code sees 401 and a proxy sees 401 too.
func TestWriteStatus_ForcesStatusUnderLegacy(t *testing.T) {
	restore(t)
	SetStyle(StyleLegacy)

	rec := httptest.NewRecorder()
	WriteStatus(rec, 401, Envelope{Code: 401, Message: "Unauthorized.", Status: False()})

	if rec.Code != 401 {
		t.Fatalf("HTTP status = %d, want 401 (legacy Write would have said 200)", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("not json: %v", err)
	}
	if body["code"] != float64(401) {
		t.Errorf("body code = %v, want 401", body["code"])
	}
}

package whatsapp

import (
	"net/http/httptest"
	"testing"
)

func serve(t *testing.T, token, query string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", "/whatsapp/webhook?"+query, nil)
	rec := httptest.NewRecorder()
	New(token, nil).Verify(rec, req)
	return rec
}

// A matching token echoes the challenge verbatim with HTTP 200 - a real status and a bare body,
// not the envelope (#23).
func TestVerify_Handshake(t *testing.T) {
	rec := serve(t, "secret", "hub.mode=subscribe&hub.verify_token=secret&hub.challenge=CHALLENGE_42")
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "CHALLENGE_42" {
		t.Errorf("body = %q, want the bare challenge", rec.Body.String())
	}
}

// Wrong token, or not a subscribe, is a real 403.
func TestVerify_Rejects(t *testing.T) {
	if c := serve(t, "secret", "hub.mode=subscribe&hub.verify_token=wrong&hub.challenge=x").Code; c != 403 {
		t.Errorf("wrong token: status = %d, want 403", c)
	}
	if c := serve(t, "secret", "hub.mode=unsubscribe&hub.verify_token=secret").Code; c != 403 {
		t.Errorf("wrong mode: status = %d, want 403", c)
	}
}

// An unset verify token fails closed with a real 500.
func TestVerify_NoToken(t *testing.T) {
	if c := serve(t, "", "hub.mode=subscribe&hub.verify_token=&hub.challenge=x").Code; c != 500 {
		t.Errorf("status = %d, want 500 when token unset", c)
	}
}

package totp

import (
	"testing"
	"time"
)

// RFC 6238 test vector: SHA1 secret "12345678901234567890" (base32 below); at T=59s the 8-digit
// code is 94287082, so the 6-digit code is 287082.
const rfcSecret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

func TestVerify_RFCVector(t *testing.T) {
	at := time.Unix(59, 0)
	if !Verify(rfcSecret, "287082", at) {
		t.Error("RFC vector code should verify at T=59")
	}
	if Verify(rfcSecret, "000000", at) {
		t.Error("a wrong code must not verify")
	}
	if Verify(rfcSecret, "28708", at) {
		t.Error("a 5-digit code is invalid")
	}
	if Verify("not base32 !!!", "287082", at) {
		t.Error("a bad secret must not verify")
	}
}

func TestVerify_Window(t *testing.T) {
	// 287082 is step 1 (T=59). It should also verify one step earlier/later (±1 window).
	if !Verify(rfcSecret, "287082", time.Unix(59+30, 0)) {
		t.Error("code should still verify one step later (window +1)")
	}
	if Verify(rfcSecret, "287082", time.Unix(59+90, 0)) {
		t.Error("code should NOT verify two steps later (outside window)")
	}
}

func TestGenerateSecret_RoundTrips(t *testing.T) {
	s, err := GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 32 {
		t.Errorf("secret len = %d, want 32 (20 bytes base32)", len(s))
	}
	if _, err := decode(s); err != nil {
		t.Errorf("generated secret should decode: %v", err)
	}
}

func TestOtpauthURI(t *testing.T) {
	uri := OtpauthURI("ABC234", "a@b.co")
	for _, want := range []string{"otpauth://totp/", "secret=ABC234", "issuer=InvoiceMG", "digits=6", "period=30", "algorithm=SHA1"} {
		if !contains(uri, want) {
			t.Errorf("uri %q missing %q", uri, want)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

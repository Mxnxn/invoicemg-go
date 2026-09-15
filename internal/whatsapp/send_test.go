package whatsapp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

func TestNormalizePhone(t *testing.T) {
	cases := map[string]string{"9374629956": "919374629956", "+91 93746 29956": "919374629956", "": "", "12345678901": "12345678901"}
	for in, want := range cases {
		if got := normalizePhone(in); got != want {
			t.Errorf("normalizePhone(%q) = %q, want %q", in, got, want)
		}
	}
}

func sendHandler(graphBase string, c store.Companies) *Handler {
	return &Handler{companies: c, graphBase: graphBase, httpClient: &http.Client{Timeout: 5 * time.Second}}
}

func doSend(t *testing.T, h *Handler, form string) map[string]any {
	t.Helper()
	r := httptest.NewRequest("POST", "/", strings.NewReader(form))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	h.Send(rec, r)
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("not json: %v (%s)", err, rec.Body.String())
	}
	return body
}

func configured() *stubCompanies {
	return &stubCompanies{found: true, company: store.Company{WaAPIToken: "tok", WaPhoneNumberID: "PID"}}
}

func TestSendValidation(t *testing.T) {
	// missing fields
	if b := doSend(t, sendHandler("", configured()), "to=919000000001"); b["code"] != float64(422) || b["message"] != "Invalid request." {
		t.Errorf("missing message: %v", b)
	}
	// unconfigured
	unconf := &stubCompanies{found: true, company: store.Company{}}
	if b := doSend(t, sendHandler("", unconf), "to=919000000001&message=hi"); b["code"] != float64(422) || !strings.Contains(b["message"].(string), "isn't configured") {
		t.Errorf("unconfigured: %v", b)
	}
	// no phone
	if b := doSend(t, sendHandler("", configured()), "to=abc&message=hi"); b["code"] != float64(422) || !strings.Contains(b["message"].(string), "no phone number") {
		t.Errorf("no phone: %v", b)
	}
}

func TestSendSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/PID/messages") || r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("bad request: %s %s", r.URL.Path, r.Header.Get("Authorization"))
		}
		json.NewEncoder(w).Encode(map[string]any{"messages": []any{map[string]any{"id": "wamid.1"}}})
	}))
	defer srv.Close()
	b := doSend(t, sendHandler(srv.URL, configured()), "to=9374629956&message=hi")
	if b["code"] != float64(200) || b["message"] != "Message sent." {
		t.Fatalf("send: %v", b)
	}
	if b["data"].(map[string]any)["messages"] == nil {
		t.Errorf("meta data not passed through: %v", b["data"])
	}
}

func TestSendMetaErrors(t *testing.T) {
	// expired token (190)
	srv190 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": 190, "message": "Auth", "error_data": map[string]any{"details": "expired"}}})
	}))
	defer srv190.Close()
	b := doSend(t, sendHandler(srv190.URL, configured()), "to=9374629956&message=hi")
	if b["code"] != float64(502) || !strings.Contains(b["message"].(string), "token expired") || !strings.Contains(b["message"].(string), "expired") {
		t.Errorf("190: %v", b)
	}
	// generic meta error surfaces its message
	srvGen := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": 131, "message": "Number not on WhatsApp"}})
	}))
	defer srvGen.Close()
	b = doSend(t, sendHandler(srvGen.URL, configured()), "to=9374629956&message=hi")
	if b["code"] != float64(502) || b["message"] != "Number not on WhatsApp" {
		t.Errorf("generic: %v", b)
	}
}

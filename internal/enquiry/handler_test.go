package enquiry

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/store"
)

type stubEnq struct {
	id     store.ID
	got    store.EnquiryWrite
	called bool
}

func (s *stubEnq) Create(_ context.Context, in store.EnquiryWrite) (store.ID, error) {
	s.called = true
	s.got = in
	return s.id, nil
}
func (s *stubEnq) List(context.Context) ([]store.Enquiry, error)      { return nil, nil }
func (s *stubEnq) SetHandled(context.Context, store.ID, bool) error { return nil }

func newH(s store.Enquiries, token string) *Handler {
	return &Handler{store: s, token: token, attempts: map[string][]time.Time{}, now: func() time.Time { return time.Unix(1700000000, 0) }}
}

func post(h *Handler, vals url.Values, headers map[string]string) map[string]any {
	r := httptest.NewRequest("POST", "/", strings.NewReader(vals.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.Create(rec, r)
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	return body
}

var valid = url.Values{"name": {"Asha"}, "email": {"a@b.co"}, "phone": {"+911234567890"}}

func TestHoneypot(t *testing.T) {
	s := &stubEnq{}
	v := url.Values{"website": {"bot"}, "name": {"Asha"}, "email": {"a@b.co"}, "phone": {"+911234567890"}}
	body := post(newH(s, ""), v, nil)
	if body["code"] != float64(200) {
		t.Errorf("honeypot -> %v, want 200 (fake success)", body["code"])
	}
	if s.called {
		t.Error("honeypot must not record an enquiry")
	}
}

func TestToken(t *testing.T) {
	s := &stubEnq{id: "e1"}
	if body := post(newH(s, "secret"), valid, nil); body["code"] != float64(401) {
		t.Errorf("no token -> %v, want 401", body["code"])
	}
	if body := post(newH(s, "secret"), valid, map[string]string{"x-enquiry-token": "secret"}); body["code"] != float64(200) {
		t.Errorf("right token -> %v, want 200", body["code"])
	}
}

func TestValidation(t *testing.T) {
	cases := []struct {
		v    url.Values
		want float64
	}{
		{url.Values{"email": {"a@b.co"}, "phone": {"+911234567890"}}, 400},          // no name
		{url.Values{"name": {"A"}, "email": {"nope"}, "phone": {"+911234567890"}}, 400}, // bad email
		{url.Values{"name": {"A"}, "email": {"a@b.co"}, "phone": {"12345"}}, 400},    // bad phone
	}
	for _, c := range cases {
		if body := post(newH(&stubEnq{}, ""), c.v, nil); body["code"] != c.want {
			t.Errorf("%v -> %v, want %v", c.v, body["code"], c.want)
		}
	}
}

func TestSuccess(t *testing.T) {
	s := &stubEnq{id: "e9"}
	body := post(newH(s, ""), valid, nil)
	if body["code"] != float64(200) || body["data"].(map[string]any)["_id"] != "e9" {
		t.Errorf("success -> %v", body)
	}
	if s.got.Email != "a@b.co" || s.got.Name != "Asha" {
		t.Errorf("write = %+v", s.got)
	}
	if s.got.Source != "landing" {
		t.Errorf("source default = %q, want landing", s.got.Source)
	}
}

func TestRateLimit(t *testing.T) {
	h := newH(&stubEnq{}, "")
	for i := 0; i < rateLimit; i++ {
		if body := post(h, valid, nil); body["code"] != float64(200) {
			t.Fatalf("attempt %d -> %v, want 200", i, body["code"])
		}
	}
	if body := post(h, valid, nil); body["code"] != float64(429) {
		t.Errorf("over the limit -> %v, want 429", body["code"])
	}
}

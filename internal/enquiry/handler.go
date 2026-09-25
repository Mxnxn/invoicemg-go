// Package enquiry serves POST /enquiry from routes/Enquiry.js - the landing page's demo/pricing
// form, the one unauthenticated write in the app. Its guards (honeypot, shared token, per-IP rate
// limit) turn away drive-by bots; the rate limit is what actually bounds the damage.
package enquiry

import (
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

const (
	rateLimit   = 5
	rateWindow  = 10 * time.Minute
	noteMax     = 2000
)

type Handler struct {
	store store.Enquiries
	token string
	now   func() time.Time

	mu       sync.Mutex
	attempts map[string][]time.Time
}

func New(s store.Enquiries) *Handler {
	return &Handler{
		store:    s,
		token:    strings.TrimSpace(os.Getenv("ENQUIRY_TOKEN")),
		now:      func() time.Time { return time.Now() },
		attempts: map[string][]time.Time{},
	}
}

var (
	emailRE = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]{2,}$`)
	phoneRE = regexp.MustCompile(`^\+\d{8,15}$`)
	spaceRE = regexp.MustCompile(`[\s-]`)
)

// checkRate is the per-IP fixed window (Helpers-free port of Enquiry.checkRate): prune expired,
// refuse at the limit, else record this attempt.
func (h *Handler) checkRate(ip string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := h.now()
	for seen, list := range h.attempts {
		kept := list[:0]
		for _, at := range list {
			if now.Sub(at) < rateWindow {
				kept = append(kept, at)
			}
		}
		if len(kept) == 0 {
			delete(h.attempts, seen)
		} else {
			h.attempts[seen] = kept
		}
	}
	mine := h.attempts[ip]
	fresh := mine[:0]
	for _, at := range mine {
		if now.Sub(at) < rateWindow {
			fresh = append(fresh, at)
		}
	}
	if len(fresh) >= rateLimit {
		h.attempts[ip] = fresh
		return false
	}
	h.attempts[ip] = append(fresh, now)
	return true
}

// tokenAccepted mirrors Node: an unset ENQUIRY_TOKEN means "no barrier configured", not "reject".
func (h *Handler) tokenAccepted(r *http.Request) bool {
	if h.token == "" {
		return true
	}
	return strings.TrimSpace(r.Header.Get("x-enquiry-token")) == h.token
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("x-forwarded-for"); xff != "" {
		if ip := strings.TrimSpace(strings.Split(xff, ",")[0]); ip != "" {
			return ip
		}
	}
	if r.RemoteAddr != "" {
		if i := strings.LastIndex(r.RemoteAddr, ":"); i >= 0 {
			return r.RemoteAddr[:i]
		}
		return r.RemoteAddr
	}
	return "unknown"
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	form, _ := httpx.ReadForm(r)

	// Honeypot: a field never shown to a person. Anything in it is a bot; answer as if it worked.
	if strings.TrimSpace(form.String("website")) != "" {
		httpx.Write(w, httpx.Envelope{Code: 200, Status: httpx.True(), Message: "Thanks - we will be in touch."})
		return
	}
	if !h.tokenAccepted(r) {
		httpx.Write(w, httpx.Envelope{Code: 401, Status: httpx.False(), Message: "Enquiry rejected."})
		return
	}
	if !h.checkRate(clientIP(r)) {
		httpx.Write(w, httpx.Envelope{Code: 429, Status: httpx.False(), Message: "Too many enquiries from here. Please try again a little later."})
		return
	}

	name := strings.TrimSpace(form.String("name"))
	email := strings.TrimSpace(form.String("email"))
	phone := strings.TrimSpace(form.String("phone"))
	company := strings.TrimSpace(form.String("companyName"))
	note := strings.TrimSpace(form.String("note"))
	if len(note) > noteMax {
		note = note[:noteMax]
	}

	if name == "" {
		httpx.Write(w, httpx.Envelope{Code: 400, Status: httpx.False(), Message: "Name is required."})
		return
	}
	if !emailRE.MatchString(email) {
		httpx.Write(w, httpx.Envelope{Code: 400, Status: httpx.False(), Message: "A valid email is required."})
		return
	}
	if !phoneRE.MatchString(spaceRE.ReplaceAllString(phone, "")) {
		httpx.Write(w, httpx.Envelope{Code: 400, Status: httpx.False(), Message: "A valid phone number is required."})
		return
	}

	source := form.String("source")
	if source == "" {
		source = "landing"
	}
	if len(source) > 40 {
		source = source[:40]
	}
	ua := r.Header.Get("user-agent")
	if len(ua) > 300 {
		ua = ua[:300]
	}
	id, err := h.store.Create(r.Context(), store.EnquiryWrite{
		Name: name, Email: email, Phone: phone, CompanyName: company, Note: note, Source: source, UserAgent: ua,
	})
	if err != nil {
		httpx.Write(w, httpx.Envelope{Code: 500, Status: httpx.False(), Message: "Could not record that enquiry."})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Status: httpx.True(), Message: "Thanks - we will be in touch.", Data: map[string]any{"_id": string(id)}})
}

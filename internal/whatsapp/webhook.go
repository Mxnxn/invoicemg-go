// Package whatsapp serves the WhatsApp webhook from routes/WhatsAppWebhook.js. Only Meta's
// GET verification handshake is ported so far - the POST callback (HMAC over the raw body, plus
// message-status processing) is not.
//
// It is unauthenticated on purpose: Meta carries no session, and for the POST the HMAC
// signature is the authentication. This is also the one corner of the app that answers with
// REAL HTTP status codes and a bare body rather than the {code,message,status} envelope, which
// is why it uses httpx.Raw / httpx.Status (#23).
package whatsapp

import (
	"net/http"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type Handler struct {
	// verifyToken is captured at construction from WHATSAPP_VERIFY_TOKEN. Node reads it per
	// request, but the token is set once at deploy, so a boot-time value matches in practice.
	verifyToken string
	companies   store.Companies
	// graphBase and httpClient front the WhatsApp Cloud API; overridable in tests.
	graphBase  string
	httpClient *http.Client
}

func New(verifyToken string, companies store.Companies) *Handler {
	return &Handler{
		verifyToken: verifyToken, companies: companies,
		graphBase:  "https://graph.facebook.com",
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// Verify is GET /whatsapp/webhook: Meta's one-time subscription handshake. When the token
// matches, echo hub.challenge back as a bare string with HTTP 200 (Express's res.send(string)
// defaults to text/html); an unset secret is a real 500, a mismatch a real 403 - all via
// sendStatus, so Meta reads the status line.
func (h *Handler) Verify(w http.ResponseWriter, r *http.Request) {
	if h.verifyToken == "" {
		// Fail closed: without the token the check cannot be performed, and Meta must not be
		// told the endpoint is fine.
		httpx.Status(w, http.StatusInternalServerError)
		return
	}
	q := r.URL.Query()
	if q.Get("hub.mode") == "subscribe" && q.Get("hub.verify_token") == h.verifyToken {
		httpx.Raw(w, http.StatusOK, "text/html; charset=utf-8", []byte(q.Get("hub.challenge")))
		return
	}
	httpx.Status(w, http.StatusForbidden)
}

// The WhatsApp settings routes from routes/WhatsApp.js: read (/config) and save (/config/update)
// the Company's Cloud API credentials. The api token is write-only over the wire - only whether
// one is set is ever echoed back, so a blank submit can't wipe a working token by accident.
package whatsapp

import (
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

func publicConfig(c store.Company) map[string]any {
	return map[string]any{
		"phoneNumberId":     c.WaPhoneNumberID,
		"businessAccountId": c.WaBusinessAccountID,
		"hasApiToken":       c.WaAPIToken != "",
		"configured":        c.WaAPIToken != "" && c.WaPhoneNumberID != "",
	}
}

// Config is POST /whatsapp/config: the acting company's public WhatsApp settings. 404 when the
// company is missing.
func (h *Handler) Config(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	company, found, err := h.companies.FindActive(r.Context(), sess.UID, sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Company not found.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: publicConfig(company)})
}

// ConfigUpdate is POST /whatsapp/config/update (admin): save the phone/business ids, and the api
// token only when a non-blank one is submitted. 404 when the company is missing.
func (h *Handler) ConfigUpdate(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	var patch store.CompanyPatch
	if form.Present("phoneNumberId") {
		v := form.String("phoneNumberId")
		patch.WaPhoneNumberID = &v
	}
	if form.Present("businessAccountId") {
		v := form.String("businessAccountId")
		patch.WaBusinessAccountID = &v
	}
	if form.String("apiToken") != "" {
		v := form.String("apiToken")
		patch.WaAPIToken = &v
	}
	company, found, err := h.companies.Update(r.Context(), sess.UID, sess.CompanyID, patch)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Company not found.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "WhatsApp settings saved.", Data: publicConfig(company)})
}

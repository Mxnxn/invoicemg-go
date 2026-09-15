package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
)

const graphVersion = "v19.0"

// normalizePhone turns a free-text local number into the country-code-prefixed digits the Cloud
// API's `to` field wants. Every number in this data is India-based, so a bare 10-digit number is
// assumed to be missing its "91" (routes/WhatsApp.js normalizePhone).
func normalizePhone(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	digits := b.String()
	if digits == "" {
		return ""
	}
	if len(digits) == 10 {
		return "91" + digits
	}
	return digits
}

// Send is POST /whatsapp/send: forward a text message to a client over the company's WhatsApp
// Cloud API credentials. The pre-call validation (missing fields, unconfigured, no phone) is
// reproduced exactly; a Cloud API failure is surfaced as Node does, with the expired-token
// (code 190) case called out specifically.
func (h *Handler) Send(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	to, message := form.String("to"), form.String("message")
	if to == "" || message == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	company, found, err := h.companies.FindActive(r.Context(), sess.UID, sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found || company.WaAPIToken == "" || company.WaPhoneNumberID == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "WhatsApp isn't configured yet - set it up in Configure > WhatsApp.", Status: httpx.False()})
		return
	}
	dest := normalizePhone(to)
	if dest == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "This client has no phone number on file.", Status: httpx.False()})
		return
	}

	data, metaErr, err := h.sendGraph(r.Context(), company.WaPhoneNumberID, company.WaAPIToken, dest, message)
	if err != nil {
		httpx.Write(w, httpx.Envelope{Code: 502, Message: "Couldn't reach WhatsApp. Check the API token and phone number ID in Configure > WhatsApp.", Status: httpx.False()})
		return
	}
	if metaErr != nil {
		if metaErr.Code == 190 {
			detail := metaErr.ErrorData.Details
			if detail == "" {
				detail = metaErr.Message
			}
			msg := "WhatsApp token expired or invalid - generate a new one and save it in Configure > WhatsApp."
			if detail != "" {
				msg += fmt.Sprintf(" (%s)", detail)
			}
			httpx.Write(w, httpx.Envelope{Code: 502, Message: msg, Status: httpx.False()})
			return
		}
		msg := metaErr.Message
		if msg == "" {
			msg = "Couldn't reach WhatsApp. Check the API token and phone number ID in Configure > WhatsApp."
		}
		httpx.Write(w, httpx.Envelope{Code: 502, Message: msg, Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Message sent.", Data: data})
}

type metaError struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	ErrorData struct {
		Details string `json:"details"`
	} `json:"error_data"`
}

// sendGraph posts the message to the Cloud API. It returns the decoded success body, or the
// Meta error object when the API rejected it, or a transport error.
func (h *Handler) sendGraph(ctx context.Context, phoneNumberID, apiToken, to, message string) (any, *metaError, error) {
	body, _ := json.Marshal(map[string]any{
		"messaging_product": "whatsapp",
		"to":                to,
		"type":              "text",
		"text":              map[string]any{"body": message},
	})
	url := fmt.Sprintf("%s/%s/%s/messages", h.graphBase, graphVersion, phoneNumberID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	var decoded struct {
		Error *metaError `json:"error"`
	}
	raw := json.NewDecoder(resp.Body)
	var full map[string]any
	if err := raw.Decode(&full); err != nil {
		return nil, nil, err
	}
	if e, ok := full["error"]; ok && e != nil {
		b, _ := json.Marshal(e)
		_ = json.Unmarshal(b, &decoded.Error)
		return nil, decoded.Error, nil
	}
	return full, nil, nil
}

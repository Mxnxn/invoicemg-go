package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
)

// bodyVarRe counts the {{n}} placeholders in a template body, matching routes/WhatsApp.js's
// /{{\s*\d+\s*}}/g so the picker can show how many variables a send must supply.
var bodyVarRe = regexp.MustCompile(`{{\s*\d+\s*}}`)

// Templates is POST /whatsapp/templates: the message templates approved on this company's
// WhatsApp Business Account, read LIVE from Meta (approval state changes there without telling
// us, so a cached list would offer a template that has since been rejected). Only the picker's
// fields are returned - never the token, never the raw Graph payload. Missing token/business id
// -> 422; any Meta or transport failure -> 502 (Node catches both the same way).
func (h *Handler) Templates(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())

	company, found, err := h.companies.FindActive(r.Context(), sess.UID, sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found || company.WaAPIToken == "" || company.WaBusinessAccountID == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Add the API token and WhatsApp Business Account ID first, then the templates will load.", Status: httpx.False()})
		return
	}

	templates, metaErr, err := h.fetchTemplates(r.Context(), company.WaBusinessAccountID, company.WaAPIToken)
	if err != nil {
		httpx.Write(w, httpx.Envelope{Code: 502, Message: "Couldn't load templates from WhatsApp.", Status: httpx.False()})
		return
	}
	if metaErr != nil {
		msg := metaErr.Message
		if msg == "" {
			msg = "Couldn't load templates from WhatsApp."
		}
		httpx.Write(w, httpx.Envelope{Code: 502, Message: msg, Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Status: httpx.True(), Data: templates})
}

// templateDTO is the trimmed shape the picker needs - never the token, never Meta's account
// metadata. Field names and derivations match routes/WhatsApp.js exactly.
type templateDTO struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	Language             string `json:"language"`
	Status               string `json:"status"`
	Category             string `json:"category"`
	HeaderText           string `json:"headerText"`
	BodyText             string `json:"bodyText"`
	BodyVariables        int    `json:"bodyVariables"`
	HasURLButton         bool   `json:"hasUrlButton"`
	URLButtonHasVariable bool   `json:"urlButtonHasVariable"`
}

// graphTemplate mirrors the parts of Meta's message_templates payload we read.
type graphTemplate struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Language   string `json:"language"`
	Status     string `json:"status"`
	Category   string `json:"category"`
	Components []struct {
		Type    string `json:"type"`
		Format  string `json:"format"`
		Text    string `json:"text"`
		Buttons []struct {
			Type string `json:"type"`
			URL  string `json:"url"`
		} `json:"buttons"`
	} `json:"components"`
}

// fetchTemplates GETs the account's templates from the Graph API and maps them. It returns the
// Meta error object when the payload carried one, or a transport error otherwise.
func (h *Handler) fetchTemplates(ctx context.Context, businessAccountID, apiToken string) ([]templateDTO, *metaError, error) {
	url := fmt.Sprintf("%s/%s/%s/message_templates?limit=200", h.graphBase, graphVersion, businessAccountID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiToken)
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	var decoded struct {
		Data  []graphTemplate `json:"data"`
		Error *metaError      `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, nil, err
	}
	if decoded.Error != nil {
		return nil, decoded.Error, nil
	}

	out := make([]templateDTO, 0, len(decoded.Data))
	for _, t := range decoded.Data {
		dto := templateDTO{ID: t.ID, Name: t.Name, Language: t.Language, Status: t.Status, Category: t.Category}
		for _, c := range t.Components {
			switch c.Type {
			case "HEADER":
				if c.Format == "TEXT" {
					dto.HeaderText = c.Text
				}
			case "BODY":
				dto.BodyText = c.Text
				dto.BodyVariables = len(bodyVarRe.FindAllString(c.Text, -1))
			case "BUTTONS":
				for _, b := range c.Buttons {
					if b.Type == "URL" {
						dto.HasURLButton = true
						if strings.Contains(b.URL, "{{") {
							dto.URLButtonHasVariable = true
						}
					}
				}
			}
		}
		out = append(out, dto)
	}
	return out, nil, nil
}

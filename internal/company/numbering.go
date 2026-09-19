package company

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/docnumber"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
)

// numberingClock is injectable so a preview's financial year is deterministic in tests.
var numberingClock = func() time.Time { return time.Now().UTC() }

// numberingKinds is the four document types the read route shows (routes/Company.js). The update
// route additionally accepts "purchaseOrder", which lives in docnumber.DefaultFormats.
var numberingKinds = []string{"invoice", "job", "quotation", "purchase"}

// Numbering is POST /company/numbering: the acting company's per-type numbering formats, each
// filled in from DEFAULT_FORMATS where unset, plus a preview of what the next document would look
// like. Session-only (not admin). Never 404s - a company with nothing stored gets the defaults.
func (h *Handler) Numbering(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	stored, err := h.companies.Numbering(r.Context(), sess.CompanyID, sess.UID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	now := numberingClock()
	numbering := make(map[string]docnumber.Format, len(numberingKinds))
	preview := make(map[string]string, len(numberingKinds))
	for _, kind := range numberingKinds {
		f := docnumber.NormalizeFormat(rawFormat(stored[kind]), docnumber.DefaultFormats[kind])
		numbering[kind] = f
		preview[kind] = docnumber.FormatDocumentNumber(f, 1, now)
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Status: httpx.True(), Data: map[string]any{
		"numbering": numbering, "preview": preview,
	}})
}

// NumberingUpdate is POST /company/numbering/update (admin): set one document type's format. An
// unknown kind is 422, a normalised format with no prefix is 422 (a prefix is what tells one
// series from another), a missing company is 404.
func (h *Handler) NumberingUpdate(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)

	kind := form.String("kind")
	fallback, ok := docnumber.DefaultFormats[kind]
	if !ok {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Unknown document type.", Status: httpx.False()})
		return
	}
	var raw docnumber.RawFormat
	if form.Present("prefix") {
		v := form.String("prefix")
		raw.Prefix = &v
	}
	if form.Present("year") {
		v := form.String("year")
		raw.Year = &v
	}
	if form.Present("pad") {
		v := form.String("pad")
		raw.Pad = &v
	}
	if form.Present("separator") {
		v := form.String("separator")
		raw.Separator = &v
	}
	format := docnumber.NormalizeFormat(raw, fallback)
	if format.Prefix == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "A prefix is required - it is what tells one series from another.", Status: httpx.False()})
		return
	}
	blob, err := json.Marshal(format)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	found, err := h.companies.SetNumbering(r.Context(), sess.CompanyID, sess.UID, kind, blob)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Company not found.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{
		Code: 200, Status: httpx.True(),
		Message: "Numbering updated. Documents already issued keep the numbers they have.",
		Data:    map[string]any{"format": format, "preview": docnumber.FormatDocumentNumber(format, 1, numberingClock())},
	})
}

// rawFormat turns a stored format blob into a docnumber.RawFormat. The stored pad is a JSON
// number, so it is read through json.Number and handed on as the string docnumber.NormalizeFormat
// coerces. A blank blob yields an all-nil raw, which normalises to the fallback.
func rawFormat(blob json.RawMessage) docnumber.RawFormat {
	if len(blob) == 0 {
		return docnumber.RawFormat{}
	}
	var p struct {
		Prefix    *string      `json:"prefix"`
		Year      *string      `json:"year"`
		Pad       *json.Number `json:"pad"`
		Separator *string      `json:"separator"`
	}
	_ = json.Unmarshal(blob, &p)
	rf := docnumber.RawFormat{Prefix: p.Prefix, Year: p.Year, Separator: p.Separator}
	if p.Pad != nil {
		s := p.Pad.String()
		rf.Pad = &s
	}
	return rf
}

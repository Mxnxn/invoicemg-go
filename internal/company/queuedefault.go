package company

import (
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

// QueueDefault is POST /lifecycle/jobs/queue/default (admin, lifecycle feature): save the acting
// company's default job queue pipeline. The order is normalised (First/Last pinned, de-duped)
// before it is stored, so a stale tab or a hand-built request still yields a usable pipeline.
// A missing field is 422 "Invalid request."; a non-array/garbage-but-valid JSON normalises to the
// fallback rather than erroring (matching normalizeQueueOrder); only invalid JSON is a parse 422.
func (h *Handler) QueueDefault(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)

	if !form.Has("queueOrder") {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	var parsed any
	if err := form.JSON("queueOrder", &parsed); err != nil {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "queueOrder must be a JSON array of strings.", Status: httpx.False()})
		return
	}
	// Node runs JSON.parse then normalizeQueueOrder, which coerces a non-array to [] and skips
	// non-string elements - so a valid-JSON non-array is not an error, it is the fallback.
	raw := []string{}
	if arr, ok := parsed.([]any); ok {
		for _, e := range arr {
			if s, ok := e.(string); ok {
				raw = append(raw, s)
			}
		}
	}
	order := store.NormalizeQueueOrder(raw)

	stored, found, err := h.companies.SetQueueOrder(r.Context(), sess.CompanyID, order)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Company not found.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Saved as the default for new jobs.", Status: httpx.True(),
		Data: map[string]any{"queueOrder": nonNilStrings(stored)}})
}

// nonNilStrings makes a nil slice serialise as [] rather than null.
func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

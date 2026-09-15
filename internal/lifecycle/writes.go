package lifecycle

import (
	"net/http"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/docnumber"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

// now is injectable so the FY the next-challan helper picks is deterministic in tests.
var now = func() time.Time { return time.Now().UTC() }

// NextChallan is POST /lifecycle/jobs/next-challan-number. Jobs use the empty document code.
func (h *Handler) NextChallan(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	numbers, err := h.store.ChallanNumbers(r.Context(), sess.UID, sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	next := docnumber.Next(numbers, "", now())
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: map[string]any{"challanNumber": next}})
}

// ByEntry is POST /lifecycle/jobs/by-entry: the populated job that owns an entry, or data:null.
func (h *Handler) ByEntry(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	entryID := form.String("entry_id")
	if entryID == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	job, found, err := h.store.ByEntry(r.Context(), sess.UID, sess.CompanyID, store.ID(entryID))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: httpx.Null})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: build(job)})
}

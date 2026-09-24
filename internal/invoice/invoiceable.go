package invoice

import (
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type invoiceableJobDTO struct {
	ID              string   `json:"_id"`
	ChallanNumber   string   `json:"challanNumber"`
	ReceivedDate    string   `json:"receivedDate"`
	Queue           string   `json:"queue"`
	Total           float64  `json:"total"`
	RowCount        int      `json:"rowCount"`
	EntryIDs        []string `json:"entryIds"`
	Amount          float64  `json:"amount"`
	InvoicedRows    int      `json:"invoicedRows"`
	UnconvertedRows int      `json:"unconvertedRows"`
	ConvertibleRows int      `json:"convertibleRows"`
	PendingRows     int      `json:"pendingRows"`
	HasIgst         bool     `json:"hasIgst"`
}

// InvoiceableJobs is POST /invoice/invoiceable-jobs: a client's jobs the invoice run can pick up.
// A job-id is offered only when NOTHING on it is still in production (pendingRows == 0) and it has
// something to bill - either unissued entries or Done rows still to convert. The per-row math
// mirrors routes/Invoice.js exactly.
func (h *Handler) InvoiceableJobs(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	clientID := form.String("client_id")
	if clientID == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "client_id is required.", Status: httpx.False()})
		return
	}

	jobs, err := h.jobs.InvoiceableJobs(r.Context(), sess.CompanyID, store.ID(clientID))
	if err != nil {
		httpx.Internal(w, err)
		return
	}

	out := make([]invoiceableJobDTO, 0, len(jobs))
	for _, job := range jobs {
		available := []string{}
		invoiced, unconverted, convertible := 0, 0, 0
		amount := 0.0
		hasIgst := false
		for _, row := range job.Rows {
			if row.Igst > 0 {
				hasIgst = true
			}
			if !row.HasEntry {
				unconverted++
				// Only a Done row can be converted (/jobs/convert-to-entries refuses the rest).
				if row.Queue == "Done" {
					convertible++
				}
				continue
			}
			if row.HasIssued {
				invoiced++
				continue
			}
			available = append(available, string(row.EntryID))
			amount += row.Amount
		}
		// Rows neither billed nor ready to bill: still in production.
		pending := unconverted - convertible

		// A job-id is invoiceable only when everything on it is done and there is something to bill.
		if pending != 0 || (len(available) == 0 && convertible == 0) {
			continue
		}
		out = append(out, invoiceableJobDTO{
			ID: string(job.ID), ChallanNumber: job.ChallanNumber, ReceivedDate: job.ReceivedDate,
			Queue: job.Queue, Total: job.Total, RowCount: len(job.Rows),
			EntryIDs: available, Amount: round2(amount), InvoicedRows: invoiced,
			UnconvertedRows: unconverted, ConvertibleRows: convertible, PendingRows: pending, HasIgst: hasIgst,
		})
	}

	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Status: httpx.True(), Data: out})
}

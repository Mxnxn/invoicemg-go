package invoice

import (
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/entrymath"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type historyEntryDTO struct {
	EntryID     string     `json:"entry_id"`
	Date        string     `json:"date"`
	Material    string     `json:"material"`
	Description string     `json:"description"`
	Qty         float64    `json:"qty"`
	Rate        float64    `json:"rate"`
	Amount      float64    `json:"amount"`
	Total       float64    `json:"total"`
	Advance     float64    `json:"advance"`
	CreatedAt   httpx.Time `json:"createdAt"`
	UpdatedAt   httpx.Time `json:"updatedAt"`
}

type historyJobDTO struct {
	JobID         string `json:"job_id"`
	ChallanNumber string `json:"challanNumber"`
}

type historyTrailDTO struct {
	At            httpx.Time `json:"at"`
	Action        string     `json:"action"`
	Detail        string     `json:"detail"`
	ActorName     string     `json:"actorName"`
	ChallanNumber string     `json:"challanNumber"`
}

// History is POST /invoice/history: an invoice's entries plus the jobs they came from and those
// jobs' audit trail (routes/Invoice.js). The display total is recomputed with entrymath, matching
// Node's entryTotal.
func (h *Handler) History(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	id := form.String("invoice_id")
	if id == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "invoice_id is required.", Status: httpx.False()})
		return
	}

	hist, found, err := h.invoices.History(r.Context(), sess.CompanyID, store.ID(id))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "No such invoice.", Status: httpx.False()})
		return
	}

	entries := make([]historyEntryDTO, 0, len(hist.Entries))
	for _, e := range hist.Entries {
		entries = append(entries, historyEntryDTO{
			EntryID: string(e.EntryID), Date: e.Date, Material: e.Material, Description: e.Description,
			Qty: e.Qty, Rate: e.Rate, Amount: e.Amount,
			Total:   entrymath.Total(entrymath.Entry{Amount: e.Amount, Discount: e.Discount, Charges: e.Charges, Cgst: e.Cgst, Sgst: e.Sgst, Igst: e.Igst}),
			Advance: e.Advance, CreatedAt: httpx.NewTime(e.CreatedAt), UpdatedAt: httpx.NewTime(e.UpdatedAt),
		})
	}

	jobs := make([]historyJobDTO, 0, len(hist.Jobs))
	challanByJob := map[store.ID]string{}
	for _, j := range hist.Jobs {
		jobs = append(jobs, historyJobDTO{JobID: string(j.JobID), ChallanNumber: j.ChallanNumber})
		challanByJob[j.JobID] = j.ChallanNumber
	}

	trail := make([]historyTrailDTO, 0, len(hist.Trail))
	for _, t := range hist.Trail {
		trail = append(trail, historyTrailDTO{
			At: httpx.NewTime(t.At), Action: t.Action, Detail: t.Detail, ActorName: t.ActorName,
			ChallanNumber: challanByJob[t.JobID],
		})
	}

	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: map[string]any{
		"invoiceId": hist.InvoiceNumber,
		"jobs":      jobs,
		"entries":   entries,
		"trail":     trail,
	}})
}

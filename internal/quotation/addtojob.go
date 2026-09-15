package quotation

import (
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

// AddRowToJob is POST /quotation/row/add-to-job: convert a quotation row into a job row,
// starting a new job or appending to the one already built from this quotation. Returns the
// re-populated quotation and the job. The system note + history are written by the store.
func (h *Handler) AddRowToJob(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	quotationID, rowID := form.String("quotation_id"), form.String("row_id")
	if quotationID == "" || rowID == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	res, err := h.store.AddRowToJob(r.Context(), sess.UID, sess.CompanyID, store.ID(quotationID), store.ID(rowID))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	switch res.Status {
	case store.QRJQuotationNotFound:
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Quotation not found.", Status: httpx.False()})
		return
	case store.QRJRowNotFound:
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Row not found.", Status: httpx.False()})
		return
	case store.QRJAlreadyAdded:
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "This row has already been added to a job.", Status: httpx.False()})
		return
	case store.QRJDupChallan:
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Job number collision - try again.", Status: httpx.False()})
		return
	}
	message := "Row added to the quotation's existing job."
	if res.IsNew {
		message = "Job created from quotation row."
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: message, Data: map[string]any{
		"quotation": toDTO(res.Quotation),
		"job":       jobJSON(res.Job),
	}})
}

// jobJSON is the converted job in a compact shape (this endpoint has no frontend consumer that
// reads the deep job populate; the identity, totals and rows are what matters).
func jobJSON(j store.Job) map[string]any {
	rows := make([]map[string]any, 0, len(j.Rows))
	for _, r := range j.Rows {
		rows = append(rows, map[string]any{
			"_id": string(r.ID), "rowId": r.RowID, "material": r.Material, "description": r.Description,
			"qty": r.Qty, "rate": r.Rate, "cgst": r.Cgst, "sgst": r.Sgst, "queue": r.Queue, "progress": r.Progress,
		})
	}
	m := map[string]any{
		"_id": string(j.ID), "challanNumber": j.ChallanNumber, "total": j.Total,
		"queue": j.Queue, "progress": j.Progress, "rows": rows,
	}
	if j.Client != nil {
		m["client_id"] = map[string]any{"_id": string(j.Client.ID), "clientName": j.Client.ClientName, "clientFirm": j.Client.ClientFirm}
	} else {
		m["client_id"] = nil
	}
	return m
}

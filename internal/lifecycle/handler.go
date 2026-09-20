// Package lifecycle serves POST /lifecycle/jobs/list from routes/Lifecycle.js - the Jobs board.
// Each job carries its six populations (client, employee, vendor, and per-row quotation,
// employee, entry) and the state withInvoiceState derives: invoiceState, invoiceNumbers,
// readyForInvoice, lock, alert and per-channel alerts, plus a per-row `invoiced` flag. Behind
// the lifecycle feature. The create/update, the eight state-machine transitions, notes,
// history and convert-to-entries are ported across the sibling files; only jobs/delete (Trash)
// is left out.
package lifecycle

import (
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/joblifecycle"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type Handler struct {
	store store.Jobs
	notes store.JobNotes
	// materials backs the Job Report's per-line HSN lookup (job rows do not store HSN).
	materials store.Materials
}

func New(s store.Jobs, n store.JobNotes, m store.Materials) *Handler {
	return &Handler{store: s, notes: n, materials: m}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)

	list, err := h.store.List(r.Context(), sess.UID, sess.CompanyID, store.ID(form.String("client_id")))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out := make([]jobDTO, 0, len(list))
	for _, j := range list {
		out = append(out, build(j))
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: out})
}

func build(j store.Job) jobDTO {
	// Map to the pure derivation input, and build the issued set from the rows the store already
	// resolved as invoiced.
	lrows := make([]joblifecycle.Row, 0, len(j.Rows))
	issued := map[string]bool{}
	for _, r := range j.Rows {
		entryID := ""
		if r.Entry != nil {
			entryID = string(r.Entry.ID)
		}
		if r.EntryIssued && entryID != "" {
			issued[entryID] = true
		}
		lrows = append(lrows, joblifecycle.Row{
			ID: string(r.ID), RowID: r.RowID, Material: r.Material, Description: r.Description,
			Qty: r.Qty, HasDimensions: r.HasDimensions, Length: r.Length, Width: r.Width, Rate: r.Rate,
			Cgst: r.Cgst, Sgst: r.Sgst, Igst: r.Igst, Discount: r.Discount, Queue: r.Queue, EntryID: entryID,
		})
	}
	ljob := joblifecycle.Job{
		Unlocked: j.Unlocked, Rows: lrows,
		Created: joblifecycle.Channel{Status: j.CreatedAlert.Status, Error: j.CreatedAlert.Error, Count: j.CreatedAlert.Count, RowIDs: j.CreatedAlert.RowIDs, Signature: j.CreatedAlert.Signature},
		Done:    joblifecycle.Channel{Status: j.DoneAlert.Status, Error: j.DoneAlert.Error, Count: j.DoneAlert.Count, RowIDs: j.DoneAlert.RowIDs, Signature: j.DoneAlert.Signature},
	}

	inv := joblifecycle.DeriveInvoiceState(lrows, issued)
	invoiced := inv.State != "none"
	lock := joblifecycle.LockState(j.Unlocked, invoiced)

	// invoiceNumbers: distinct, in row order, from the covering invoice the store found per row.
	seen := map[string]bool{}
	invoiceNumbers := []string{}
	for _, r := range j.Rows {
		if r.InvoiceNumber != "" && !seen[r.InvoiceNumber] {
			seen[r.InvoiceNumber] = true
			invoiceNumbers = append(invoiceNumbers, r.InvoiceNumber)
		}
	}

	dto := jobDTO{
		ID: string(j.ID), ChallanNumber: j.ChallanNumber, ReceivedDate: j.ReceivedDate,
		Total: j.Total, Advance: j.Advance, Queue: j.Queue, Progress: j.Progress,
		QueueOrder: nonNil(j.QueueOrder), Unlocked: j.Unlocked,
		Rows:      rowsDTO(j.Rows, issued),
		CreatedAt: httpx.NewTime(j.CreatedAt), UpdatedAt: httpx.NewTime(j.UpdatedAt), Version: j.Version,
		InvoiceState:    inv.State,
		InvoicedRows:    inv.InvoicedRows,
		InvoiceNumbers:  invoiceNumbers,
		ReadyForInvoice: joblifecycle.IsReadyForInvoice(lrows, inv.State),
		Lock:            lockDTO(lock),
		Alert:           doneAlertLegacy(j),
		Alerts: alertsDTO{
			Created: channelDTO(joblifecycle.Alert(ljob, "created"), j.CreatedAlert),
			Done:    channelDTO(joblifecycle.Alert(ljob, "done"), j.DoneAlert),
		},
	}
	if j.Client != nil {
		dto.ClientID = &jobClientDTO{
			ID: string(j.Client.ID), ClientName: j.Client.ClientName, ClientFirm: j.Client.ClientFirm,
			ClientPhone: j.Client.ClientPhone, ClientAddress: j.Client.ClientAddress,
			NotifyOnCreate: j.Client.NotifyOnCreate, NotifyOnUpdate: j.Client.NotifyOnUpdate,
		}
	}
	dto.EmployeeID = personPtr(j.Employee)
	dto.VendorID = personPtr(j.Vendor)
	return dto
}

func rowsDTO(rows []store.JobRow, issued map[string]bool) []rowDTO {
	out := make([]rowDTO, 0, len(rows))
	for _, r := range rows {
		hasDim := true
		if r.HasDimensions != nil {
			hasDim = *r.HasDimensions
		}
		row := rowDTO{
			ID: string(r.ID), RowID: r.RowID, Material: r.Material, Description: r.Description,
			Qty: r.Qty, HasDimensions: hasDim, Length: r.Length, Width: r.Width, Rate: r.Rate,
			Cgst: r.Cgst, Sgst: r.Sgst, Igst: r.Igst, Discount: r.Discount, Charges: r.Charges,
			Queue: r.Queue, Progress: r.Progress, QueueOrder: nonNil(r.QueueOrder),
			CreatedAt: httpx.NewTime(r.CreatedAt), UpdatedAt: httpx.NewTime(r.UpdatedAt),
			EmployeeID: personPtr(r.Employee),
			Invoiced:   r.EntryIssued,
		}
		if r.Quotation != nil {
			row.QuotationID = &quotationRefDTO{ID: string(r.Quotation.ID), QuotationNumber: r.Quotation.QuotationNumber}
		}
		if r.Entry != nil {
			row.EntryID = &entryRefDTO{ID: string(r.Entry.ID), HasIssued: r.Entry.HasIssued, Total: r.Entry.Total, Advance: r.Entry.Advance}
		}
		out = append(out, row)
	}
	return out
}

// doneAlertLegacy is JobDoneAlert.alertState (the singular `alert`): built from the done
// channel's top-level fields. Kept distinct from the per-channel `alerts.done` because the
// frontend reads both.
func doneAlertLegacy(j store.Job) legacyAlertDTO {
	done := []string{}
	for _, r := range j.Rows {
		if r.Queue == "Done" {
			done = append(done, string(r.ID))
		}
	}
	told := map[string]bool{}
	for _, id := range j.DoneAlert.RowIDs {
		told[id] = true
	}
	pending := 0
	for _, id := range done {
		if !told[id] {
			pending++
		}
	}
	everyDone := len(j.Rows) > 0 && len(done) == len(j.Rows)
	sentBefore := j.DoneAlert.Count > 0
	return legacyAlertDTO{
		SentBefore: sentBefore, SentAt: httpx.NewTimePtr(j.DoneAlert.SentAt), AlertCount: j.DoneAlert.Count,
		PendingRows: pending, CanSend: everyDone && pending > 0, IsUpdate: sentBefore,
	}
}

func channelDTO(a joblifecycle.AlertState, ch store.JobAlertChannel) channelStateDTO {
	var status *string
	if a.Status != "" {
		s := a.Status
		status = &s
	}
	return channelStateDTO{
		SentBefore: a.SentBefore, SentAt: httpx.NewTimePtr(ch.SentAt), Count: a.Count,
		Status: status, StatusAt: httpx.NewTimePtr(ch.StatusAt), Error: a.Error,
		CanSend: a.CanSend, IsUpdate: a.IsUpdate, Changed: a.Changed, Signature: a.Signature,
		PendingRows: a.PendingRows, DoneRowIDs: a.DoneRowIDs,
	}
}

func lockDTO(l joblifecycle.Lock) lockStateDTO {
	return lockStateDTO{
		Invoiced: l.Invoiced, Unlocked: l.Unlocked, CanEditValues: l.CanEditValues,
		CanEditQueue: l.CanEditQueue, CanDeleteJob: l.CanDeleteJob, CanDeleteRow: l.CanDeleteRow,
	}
}

func personPtr(p *store.JobPerson) *personDTO {
	if p == nil {
		return nil
	}
	return &personDTO{ID: string(p.ID), Name: p.Name}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

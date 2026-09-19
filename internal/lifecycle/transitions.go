package lifecycle

import (
	"fmt"
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

// txResult maps a store transition outcome onto the wire, returning the populated job on OK.
func txResult(w http.ResponseWriter, job store.Job, status store.JobTxStatus, err error, needsMsg string) {
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	switch status {
	case store.JobTxJobNotFound:
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Job not found.", Status: httpx.False()})
	case store.JobTxRowNotFound:
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Row not found.", Status: httpx.False()})
	case store.JobTxNeedsAssignee:
		httpx.Write(w, httpx.Envelope{Code: 422, Message: needsMsg, Status: httpx.False()})
	default:
		httpx.Write(w, httpx.Envelope{Code: 200, Message: "Job updated.", Data: build(job)})
	}
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// Assign is POST /lifecycle/jobs/assign.
func (h *Handler) Assign(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	jobID, kind := form.String("job_id"), form.String("type")
	if jobID == "" || kind == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	if kind != "employee" && kind != "vendor" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "type must be 'employee' or 'vendor'.", Status: httpx.False()})
		return
	}
	job, status, err := h.store.Assign(r.Context(), sess.UID, sess.CompanyID, store.ID(jobID), actorOf(sess), kind, store.ID(form.String("person_id")))
	txResult(w, job, status, err, "")
}

// Progress is POST /lifecycle/jobs/progress.
func (h *Handler) Progress(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	jobID, progress := form.String("job_id"), form.String("progress")
	if jobID == "" || progress == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	job, status, err := h.store.Progress(r.Context(), sess.UID, sess.CompanyID, store.ID(jobID), actorOf(sess), progress)
	txResult(w, job, status, err, "Assign an employee or vendor before changing progress.")
}

// Unlock is POST /lifecycle/jobs/unlock: toggle a job's edit lock. `unlocked` defaults to true
// when absent. Returns the populated job - "No change." when already at the wanted state, else
// "Job unlocked." / "Job locked.". Unlike the other transitions this route DOES send status:true,
// matching routes/Lifecycle.js.
func (h *Handler) Unlock(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	jobID := form.String("job_id")
	if jobID == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	wanted := true
	if form.Present("unlocked") {
		wanted = form.String("unlocked") == "true"
	}
	job, changed, status, err := h.store.Unlock(r.Context(), sess.UID, sess.CompanyID, store.ID(jobID), actorOf(sess), wanted)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if status == store.JobTxJobNotFound {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Job not found.", Status: httpx.False()})
		return
	}
	msg := "No change."
	if changed {
		msg = "Job locked."
		if wanted {
			msg = "Job unlocked."
		}
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: msg, Status: httpx.True(), Data: build(job)})
}

// RowsCompleteAll is POST /lifecycle/jobs/rows/complete-all: mark every not-Done row Done in one
// go. Refused with 403 when the invoice lock forbids queue changes; "Every row was already done."
// when nothing moved; else "N row(s) marked done.". Sends status:true and the populated job.
func (h *Handler) RowsCompleteAll(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	jobID := form.String("job_id")
	if jobID == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	job, moved, status, err := h.store.RowsCompleteAll(r.Context(), sess.UID, sess.CompanyID, store.ID(jobID), actorOf(sess))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	switch status {
	case store.JobTxJobNotFound:
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Job not found.", Status: httpx.False()})
		return
	case store.JobTxLocked:
		httpx.Write(w, httpx.Envelope{Code: 403, Message: "This job is invoiced and locked. Unlock it to change production stages.", Status: httpx.False()})
		return
	}
	msg := "Every row was already done."
	if moved > 0 {
		unit := "rows"
		if moved == 1 {
			unit = "row"
		}
		msg = fmt.Sprintf("%d %s marked done.", moved, unit)
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: msg, Status: httpx.True(), Data: build(job)})
}

// SetQueue is POST /lifecycle/jobs/set-queue.
func (h *Handler) SetQueue(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	jobID, queue := form.String("job_id"), form.String("queue")
	if jobID == "" || queue == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	job, status, err := h.store.SetQueue(r.Context(), sess.UID, sess.CompanyID, store.ID(jobID), actorOf(sess), queue)
	txResult(w, job, status, err, "")
}

// Queue is POST /lifecycle/jobs/queue (persist the drag-reordered stage list).
func (h *Handler) Queue(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	jobID := form.String("job_id")
	if jobID == "" || !form.Has("queueOrder") {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	var order []string
	if err := form.JSON("queueOrder", &order); err != nil {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "queueOrder must be a JSON array of strings.", Status: httpx.False()})
		return
	}
	job, status, err := h.store.QueueOrder(r.Context(), sess.UID, sess.CompanyID, store.ID(jobID), actorOf(sess), order)
	txResult(w, job, status, err, "")
}

// RowAssign is POST /lifecycle/jobs/rows/assign.
func (h *Handler) RowAssign(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	jobID, rowID := form.String("job_id"), form.String("row_id")
	if jobID == "" || rowID == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	job, status, err := h.store.RowAssign(r.Context(), sess.UID, sess.CompanyID, store.ID(jobID), store.ID(rowID), actorOf(sess), store.ID(form.String("employee_id")))
	txResult(w, job, status, err, "")
}

// RowQueue is POST /lifecycle/jobs/rows/queue.
func (h *Handler) RowQueue(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	jobID, rowID, queue := form.String("job_id"), form.String("row_id"), form.String("queue")
	if jobID == "" || rowID == "" || queue == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	job, status, err := h.store.RowSetQueue(r.Context(), sess.UID, sess.CompanyID, store.ID(jobID), store.ID(rowID), actorOf(sess), queue)
	txResult(w, job, status, err, "")
}

// RowQueueOrder is POST /lifecycle/jobs/rows/queue-order.
func (h *Handler) RowQueueOrder(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	jobID, rowID := form.String("job_id"), form.String("row_id")
	if jobID == "" || rowID == "" || !form.Has("queueOrder") {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	var order []string
	if err := form.JSON("queueOrder", &order); err != nil {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "queueOrder must be a JSON array of strings.", Status: httpx.False()})
		return
	}
	job, status, err := h.store.RowQueueOrder(r.Context(), sess.UID, sess.CompanyID, store.ID(jobID), store.ID(rowID), actorOf(sess), order)
	txResult(w, job, status, err, "")
}

// RowProgress is POST /lifecycle/jobs/rows/progress.
func (h *Handler) RowProgress(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	jobID, rowID, progress := form.String("job_id"), form.String("row_id"), form.String("progress")
	if jobID == "" || rowID == "" || progress == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	if !contains(store.RowProgressStates, progress) {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid progress value.", Status: httpx.False()})
		return
	}
	job, status, err := h.store.RowProgress(r.Context(), sess.UID, sess.CompanyID, store.ID(jobID), store.ID(rowID), actorOf(sess), progress)
	txResult(w, job, status, err, "Assign an employee before changing progress.")
}

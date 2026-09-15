package lifecycle

import (
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

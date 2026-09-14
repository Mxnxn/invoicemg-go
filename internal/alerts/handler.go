// Package alerts serves the public customer link from routes/Alert.js: the page a customer
// opens from a WhatsApp message to see their own job. It is unauthenticated - the recipient is
// a customer, not a user - and what protects it is that both ids in the path must resolve to
// the same job. The payload is ONE job and nothing else.
package alerts

import (
	"errors"
	"net/http"
	"regexp"
	"sync"

	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/jobmath"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

// objectIDRe is Node's own guard, `/^[0-9a-fA-F]{24}$/`. RE2-safe, so it ports as-is (#32).
var objectIDRe = regexp.MustCompile(`^[0-9a-fA-F]{24}$`)

type Handler struct{ store store.Alerts }

func New(s store.Alerts) *Handler { return &Handler{store: s} }

// Detail is GET /alert/{job_id}/job/{jobcard_id}.
func (h *Handler) Detail(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("job_id")
	jobcardID := r.PathValue("jobcard_id")

	// #30/#32: guard the id shape and answer 404, exactly as routes/Alert.js does with the
	// same regex, so a malformed link reads as a dead link rather than a 500. This route
	// deliberately GUARDS to 404 - unlike the ~88 routes that let a Mongoose CastError become
	// a 500 (store.ErrBadID). Whether to guard is a per-route decision, and this one does.
	if !objectIDRe.MatchString(jobID) {
		notValid(w)
		return
	}

	ctx := r.Context()
	job, err := h.store.Job(ctx, store.ID(jobID))
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrBadID) {
		notValid(w)
		return
	}
	if err != nil {
		httpx.Internal(w, err)
		return
	}

	row, ok := findRow(job, jobcardID)
	if !ok {
		notValid(w)
		return
	}

	// #20: the client and the company are fetched in parallel, as Node does with Promise.all -
	// but each goroutine writes its OWN variable, so there is no shared slice to append into
	// and the client can never land in the company's place regardless of which finishes first.
	// (errgroup would read the same; this keeps it to the stdlib.)
	var (
		client     store.AlertClient
		clientErr  error
		company    store.AlertCompany
		companyErr error
	)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); client, clientErr = h.store.Client(ctx, job.ClientID) }()
	go func() { defer wg.Done(); company, companyErr = h.store.Company(ctx, job.CompanyID) }()
	wg.Wait()

	// A missing client or company is blank, not an error - Node reads them through optional
	// chaining (`client?.clientName || ""`). Only a real failure is a 500.
	if clientErr != nil && !errors.Is(clientErr, store.ErrNotFound) {
		httpx.Internal(w, clientErr)
		return
	}
	if companyErr != nil && !errors.Is(companyErr, store.ErrNotFound) {
		httpx.Internal(w, companyErr)
		return
	}

	// The same createdAt fallback Model/Job.js's toJSON applies: an older job with a blank
	// receivedDate shows the day it was raised rather than nothing. toISOString is UTC.
	receivedDate := job.ReceivedDate
	if receivedDate == "" && job.CreatedAt != nil {
		receivedDate = job.CreatedAt.UTC().Format("2006-01-02")
	}

	rows := make([]rowDTO, 0, len(job.Rows))
	mathRows := make([]jobmath.Row, 0, len(job.Rows))
	for _, r := range job.Rows {
		rows = append(rows, publicRow(r))
		mathRows = append(mathRows, mathRow(r))
	}

	// company display name is firm, falling back to name - the same order every document prints.
	companyName := company.Firm
	if companyName == "" {
		companyName = company.Name
	}

	httpx.Write(w, httpx.Envelope{
		Code:    200,
		Message: "Operation successful.",
		Status:  httpx.True(),
		Data: detailDTO{
			JobID:         string(job.ID),
			ChallanNumber: job.ChallanNumber,
			ReceivedDate:  receivedDate,
			JobQueue:      job.Queue,
			Jobcard:       publicRow(row),
			Rows:          rows,
			Total:         jobmath.JobRowsTotal(mathRows),
			Client:        clientDTO{Name: client.Name, Firm: client.Firm},
			Company: companyDTO{
				Name:    companyName,
				Phone:   company.Phone,
				Logo:    company.URL,
				Address: company.Address,
				Gst:     company.Gst,
			},
		},
	})
}

// Review is GET /alert/{job_id}/job/{jobcard_id}/review: whether this job has been reviewed,
// so the page shows what the customer said instead of an empty form they would fill twice.
func (h *Handler) Review(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("job_id")
	// Same id guard as Detail (#30/#32) - Node checks it here too before touching the store.
	if !objectIDRe.MatchString(jobID) {
		notValid(w)
		return
	}

	rev, err := h.store.Review(r.Context(), store.ID(jobID))
	if errors.Is(err, store.ErrNotFound) {
		// #22: not reviewed sends `review: null`, not an omitted key. Here the null is a nil
		// *reviewDTO field inside data (plain encoding/json), so it needs no httpx.Null - that
		// sentinel is only for the envelope's own data.
		httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Status: httpx.True(),
			Data: reviewEnvelope{Reviewed: false, Review: nil}})
		return
	}
	if err != nil {
		httpx.Internal(w, err)
		return
	}

	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Status: httpx.True(),
		Data: reviewEnvelope{
			Reviewed: true,
			Review: &reviewDTO{
				ID: string(rev.ID),
				Scores: scoresDTO{
					Quality:       rev.Scores.Quality,
					Speed:         rev.Scores.Speed,
					Communication: rev.Scores.Communication,
					Satisfaction:  rev.Scores.Satisfaction,
					Overall:       rev.Scores.Overall,
				},
				Comment: rev.Comment,
				// #5: httpx.Time, never time.Time - Node's Date.toJSON always writes three
				// decimal places (…:56.000Z) where Go's default trims them.
				CreatedAt: httpx.NewTime(rev.CreatedAt),
			},
		}})
}

// findRow matches by _id first (the common case) then by rowId, so a link built from either
// identifier resolves - Node's findRow.
func findRow(job store.AlertJob, jobcardID string) (store.AlertRow, bool) {
	for _, r := range job.Rows {
		if string(r.ID) == jobcardID {
			return r, true
		}
	}
	for _, r := range job.Rows {
		if r.RowID != "" && r.RowID == jobcardID {
			return r, true
		}
	}
	return store.AlertRow{}, false
}

func mathRow(r store.AlertRow) jobmath.Row {
	return jobmath.Row{
		Qty: r.Qty, Rate: r.Rate,
		Cgst: r.Cgst, Sgst: r.Sgst, Igst: r.Igst,
		Discount: r.Discount, Charges: r.Charges,
		Length: r.Length, Width: r.Width, HasDimensions: r.HasDimensions,
	}
}

// publicRow is Node's publicRow: the row's own fields plus a server-computed total, so the PDF
// the customer keeps cannot disagree with the invoice they are later sent. length/width fall
// back to "1" for DISPLAY only - the total uses the raw values via mathRow, as Node does.
func publicRow(r store.AlertRow) rowDTO {
	length := r.Length
	if length == "" {
		length = "1"
	}
	width := r.Width
	if width == "" {
		width = "1"
	}
	return rowDTO{
		JobcardID:     string(r.ID),
		RowID:         r.RowID,
		Description:   r.Description,
		Material:      r.Material,
		Qty:           r.Qty,
		HasDimensions: jobmath.HasDimensions(mathRow(r)),
		Length:        length,
		Width:         width,
		Rate:          r.Rate,
		Total:         jobmath.RowGrossTotal(mathRow(r)),
		Queue:         r.Queue,
		Progress:      r.Progress,
	}
}

func notValid(w http.ResponseWriter) {
	httpx.Write(w, httpx.Envelope{Code: 404, Message: "This link is no longer valid.", Status: httpx.False()})
}

type detailDTO struct {
	JobID         string     `json:"job_id"`
	ChallanNumber string     `json:"challanNumber"`
	ReceivedDate  string     `json:"receivedDate"`
	JobQueue      string     `json:"jobQueue"`
	Jobcard       rowDTO     `json:"jobcard"`
	Rows          []rowDTO   `json:"rows"`
	Total         float64    `json:"total"`
	Client        clientDTO  `json:"client"`
	Company       companyDTO `json:"company"`
}

type rowDTO struct {
	JobcardID     string  `json:"jobcard_id"`
	RowID         string  `json:"rowId"`
	Description   string  `json:"description"`
	Material      string  `json:"material"`
	Qty           float64 `json:"qty"`
	HasDimensions bool    `json:"hasDimensions"`
	Length        string  `json:"length"`
	Width         string  `json:"width"`
	Rate          float64 `json:"rate"`
	Total         float64 `json:"total"`
	Queue         string  `json:"queue"`
	Progress      string  `json:"progress"`
}

type clientDTO struct {
	Name string `json:"name"`
	Firm string `json:"firm"`
}

// reviewEnvelope is the /review payload: {reviewed, review}. Review is a pointer so a job with
// no review sends `"review":null` (a nil pointer, no omitempty), matching Node's `review || null`.
type reviewEnvelope struct {
	Reviewed bool       `json:"reviewed"`
	Review   *reviewDTO `json:"review"`
}

type reviewDTO struct {
	ID        string     `json:"_id"`
	Scores    scoresDTO  `json:"scores"`
	Comment   string     `json:"comment"`
	CreatedAt httpx.Time `json:"createdAt"`
}

type scoresDTO struct {
	Quality       int `json:"quality"`
	Speed         int `json:"speed"`
	Communication int `json:"communication"`
	Satisfaction  int `json:"satisfaction"`
	Overall       int `json:"overall"`
}

type companyDTO struct {
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	Logo    string `json:"logo"`
	Address string `json:"address"`
	Gst     string `json:"gst"`
}

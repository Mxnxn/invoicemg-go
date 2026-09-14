// Package days is the first vertical slice: POST /sheet/only and POST /sheet/open-jobs.
//
// These two were chosen to go first for one reason - they are READS. A strangler's first
// route should be one where being wrong costs a wrong number on a screen, not a corrupted
// invoice. Both services run against the same documents, so anything that writes has to be
// right the first time; these can be diffed against Node's answer at leisure.
package days

import (
	"net/http"
	"sort"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

// openJobsLimit matches the cap in routes/Sheet.js.
const openJobsLimit = 200

type Handler struct{ Days store.Days }

func New(days store.Days) *Handler { return &Handler{Days: days} }

// dayResponse is the wire shape of one entry in /sheet/only's data array.
//
// ID is a *string so a day with no Sheet document serialises as null rather than "" - the
// Node route sends null, and the client uses it as a fallback link target, where "" and null
// behave differently.
type dayResponse struct {
	ID    *string `json:"_id"`
	Date  string  `json:"date"`
	Jobs  int     `json:"jobs"`
	Cards int     `json:"cards"`
}

// Only is POST /sheet/only - every date this company has work on, newest first.
//
// A day is a date that has job-ids on it. It is NOT the set of Sheet documents: a Sheet is
// only written when a job's rows convert to Entries at invoicing, so listing sheets meant a
// day did not appear until its work had been billed. Sheets still contribute their dates, so
// a day that only ever held Entries does not disappear.
func (h *Handler) Only(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)

	counts, err := h.Days.CountsByDate(ctx, sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	sheets, err := h.Days.SheetDates(ctx, sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}

	byDate := map[string]*dayResponse{}
	order := []string{}
	put := func(date string, sheetID *string, jobs, cards int) {
		if date == "" {
			return
		}
		if found, ok := byDate[date]; ok {
			if found.ID == nil && sheetID != nil {
				found.ID = sheetID
			}
			return
		}
		byDate[date] = &dayResponse{ID: sheetID, Date: date, Jobs: jobs, Cards: cards}
		order = append(order, date)
	}

	// Days that have job-ids on them - the list proper.
	for _, c := range counts {
		put(c.Date, nil, c.Jobs, c.Cards)
	}
	// Then the sheets, so a day that only ever held Entries still appears, and a day that has
	// both keeps its sheet id.
	for date, id := range sheets {
		s := id.String()
		if existing, ok := byDate[date]; ok {
			if existing.ID == nil {
				existing.ID = &s
			}
			continue
		}
		put(date, &s, 0, 0)
	}

	out := make([]dayResponse, 0, len(order))
	for _, date := range order {
		out = append(out, *byDate[date])
	}
	// Newest first. The dates are YYYY-MM-DD, so string order IS date order - which is the
	// whole reason this app stores them that way rather than as Date objects.
	// SliceStable, not Slice (#19): the convention is a deterministic order, and though these
	// dates are unique today (one entry per date), a stable sort keeps the list reproducible
	// if that ever stops holding.
	sort.SliceStable(out, func(i, j int) bool { return out[i].Date > out[j].Date })

	// This route sends {code, data, message} with NO `status` field. Matching Node exactly,
	// quirk and all - see internal/httpx for why that matters.
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: out})
}

type clientResponse struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

type openJobResponse struct {
	ID            string          `json:"_id"`
	ChallanNumber string          `json:"challanNumber"`
	ReceivedDate  string          `json:"receivedDate"`
	Total         float64         `json:"total"`
	Cards         int             `json:"cards"`
	OpenCards     int             `json:"openCards"`
	Client        *clientResponse `json:"client"`
}

// OpenJobs is POST /sheet/open-jobs - job-ids carrying at least one card short of Done,
// oldest received date first.
func (h *Handler) OpenJobs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)

	jobs, err := h.Days.OpenJobs(ctx, sess.CompanyID, openJobsLimit)
	if err != nil {
		httpx.Internal(w, err)
		return
	}

	// Built with make(..., 0, n) rather than declared nil: a nil slice marshals to `null`, and
	// the dashboard does `res.data || []` on it but then reads .length on the result of a
	// .slice - an empty ARRAY is what Node sends and what the client expects.
	out := make([]openJobResponse, 0, len(jobs))
	for _, job := range jobs {
		row := openJobResponse{
			ID:            job.ID.String(),
			ChallanNumber: job.ChallanNumber,
			ReceivedDate:  job.ReceivedDate,
			Total:         job.Total,
			Cards:         job.Cards,
			OpenCards:     job.OpenCards,
		}
		if job.HasClient {
			row.Client = &clientResponse{Name: job.ClientName, Phone: job.ClientPhone}
		}
		out = append(out, row)
	}

	httpx.OK(w, out)
}

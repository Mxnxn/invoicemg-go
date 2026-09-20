package lifecycle

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

// The Job Report (More > Reports), from routes/Lifecycle.js `POST /lifecycle/jobs/report`.
//
// One header row per job-id - jobcard number, invoice number(s), a PO placeholder, the job date,
// the customer and phone, a derived job name, and the job's Final Bill - with its line items
// nested under it. Built from the same populated jobs the board reads (Jobs.List, which already
// filters by client and carries each row's invoice number), filtered here to a Job-Date range.
// HSN is resolved from the product by name because job rows do not store it.

type reportLine struct {
	Media       string  `json:"media"`
	Hsn         string  `json:"hsn"`
	Description string  `json:"description"`
	Size        string  `json:"size"`
	Rate        float64 `json:"rate"`
	Qty         float64 `json:"qty"`
	Area        float64 `json:"area"`
	Tax         float64 `json:"tax"`
	Amount      float64 `json:"amount"`
}

type reportJob struct {
	JobcardNo    string       `json:"jobcardNo"`
	InvoiceNo    string       `json:"invoiceNo"`
	PoNo         string       `json:"poNo"`
	JobDate      httpx.Time   `json:"jobDate"`
	CustomerName string       `json:"customerName"`
	Mobile       string       `json:"mobile"`
	JobName      string       `json:"jobName"`
	FinalBill    float64      `json:"finalBill"`
	Lines        []reportLine `json:"lines"`
}

type reportResponse struct {
	Jobs       []reportJob `json:"jobs"`
	GrandTotal float64     `json:"grandTotal"`
}

func (h *Handler) Report(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)

	// Reuse the board's populated jobs (client filter included); the report is a different
	// arrangement of the same data, not a different query.
	list, err := h.store.List(r.Context(), sess.UID, sess.CompanyID, store.ID(form.String("client_id")))
	if err != nil {
		httpx.Internal(w, err)
		return
	}

	// Job-Date range on createdAt, whole days in UTC to match how the day-scoped reports read.
	var from, to time.Time
	if s := form.String("from"); s != "" {
		if t, e := time.Parse("2006-01-02", s); e == nil {
			from = t.UTC()
		}
	}
	if s := form.String("to"); s != "" {
		if t, e := time.Parse("2006-01-02", s); e == nil {
			to = t.UTC().Add(24*time.Hour - time.Nanosecond)
		}
	}

	// One HSN lookup for the whole report - a report reuses a handful of products across jobs.
	hsnByName := map[string]string{}
	if mats, e := h.materials.Visible(r.Context(), sess.CompanyID); e == nil {
		for _, m := range mats {
			hsnByName[m.MaterialName] = m.Hsn
		}
	}

	out := reportResponse{Jobs: make([]reportJob, 0, len(list))}
	for _, j := range list {
		created := j.CreatedAt.UTC()
		if !from.IsZero() && created.Before(from) {
			continue
		}
		if !to.IsZero() && created.After(to) {
			continue
		}

		lines := make([]reportLine, 0, len(j.Rows))
		var finalBill float64
		invoiceNos := make([]string, 0)
		seenInv := map[string]bool{}
		for _, row := range j.Rows {
			dimensional := row.HasDimensions == nil || *row.HasDimensions
			factor := 1.0
			if dimensional {
				factor = parseNum(row.Length) * parseNum(row.Width)
			}
			area := row.Qty * factor
			base := area * row.Rate
			gstPct := row.Cgst + row.Sgst + row.Igst
			tax := base * gstPct / 100
			amount := base + tax
			finalBill += amount

			size := ""
			if dimensional {
				size = row.Length + " x " + row.Width
			}
			lines = append(lines, reportLine{
				Media:       row.Material,
				Hsn:         hsnByName[row.Material],
				Description: row.Description,
				Size:        size,
				Rate:        row.Rate,
				Qty:         row.Qty,
				Area:        area,
				Tax:         tax,
				Amount:      amount,
			})

			if row.InvoiceNumber != "" && !seenInv[row.InvoiceNumber] {
				seenInv[row.InvoiceNumber] = true
				invoiceNos = append(invoiceNos, row.InvoiceNumber)
			}
		}

		out.Jobs = append(out.Jobs, reportJob{
			JobcardNo:    j.ChallanNumber,
			InvoiceNo:    strings.Join(invoiceNos, ", "),
			PoNo:         "-",
			JobDate:      httpx.Time(j.CreatedAt),
			CustomerName: clientName(j.Client),
			Mobile:       clientPhone(j.Client),
			JobName:      deriveJobName(j.Rows, j.ChallanNumber),
			FinalBill:    finalBill,
			Lines:        lines,
		})
		out.GrandTotal += finalBill
	}

	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: out})
}

// parseNum coerces a stored dimension string the way Node's Number(x)||0 does.
func parseNum(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return v
}

func clientName(c *store.JobClient) string {
	if c == nil {
		return ""
	}
	if c.ClientFirm != "" {
		return c.ClientFirm
	}
	return c.ClientName
}

func clientPhone(c *store.JobClient) string {
	if c == nil {
		return ""
	}
	return c.ClientPhone
}

// deriveJobName gives the report a job name a job has no field for: the distinct line
// descriptions read best, falling back to the distinct product names, then the challan number.
func deriveJobName(rows []store.JobRow, challan string) string {
	if d := distinctNonEmpty(rows, func(r store.JobRow) string { return strings.TrimSpace(r.Description) }); len(d) > 0 {
		return strings.Join(d, " & ")
	}
	if m := distinctNonEmpty(rows, func(r store.JobRow) string { return strings.TrimSpace(r.Material) }); len(m) > 0 {
		return strings.Join(m, " & ")
	}
	return challan
}

func distinctNonEmpty(rows []store.JobRow, get func(store.JobRow) string) []string {
	seen := map[string]bool{}
	out := make([]string, 0)
	for _, r := range rows {
		v := get(r)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

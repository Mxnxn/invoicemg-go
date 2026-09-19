package lifecycle

import (
	"encoding/json"
	"net/http"
	"strconv"
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

// jobRawRow is one submitted job row before coercion.
type jobRawRow struct {
	Material    string
	Description string
	Length      string
	Width       string
	Qty         float64
	Rate        float64
	Cgst        float64
	Sgst        float64
	Igst        float64
	Discount    float64
	Charges     float64
	QuotationID string
}

func (r *jobRawRow) UnmarshalJSON(b []byte) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	str := func(k string) string {
		if raw, ok := m[k]; ok {
			var s string
			if json.Unmarshal(raw, &s) == nil {
				return s
			}
		}
		return ""
	}
	strOr := func(k, def string) string {
		if s := str(k); s != "" {
			return s
		}
		return def
	}
	num := func(k string) float64 {
		raw, ok := m[k]
		if !ok {
			return 0
		}
		var f float64
		if json.Unmarshal(raw, &f) == nil {
			return f
		}
		var s string
		if json.Unmarshal(raw, &s) == nil {
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				return f
			}
		}
		return 0
	}
	r.Material, r.Description = str("material"), str("description")
	r.Length, r.Width = strOr("length", "1"), strOr("width", "1")
	r.Qty, r.Rate = num("qty"), num("rate")
	r.Cgst, r.Sgst, r.Igst = num("cgst"), num("sgst"), num("igst")
	r.Discount, r.Charges = num("discount"), num("charges")
	r.QuotationID = str("quotation_id")
	return nil
}

// numOrZero is Number(x)||0 for a submitted string.
func numOrZero(s string) float64 {
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return 0
}

// jobRowGrossTotal is routes/Lifecycle.rowGrossTotal: qty·length·width·rate, net of
// discount/charges, inclusive of tax (job rows always price by dimension; length/width default 1).
func jobRowGrossTotal(r store.JobRowWrite) float64 {
	length := numOrZero(r.Length)
	width := numOrZero(r.Width)
	amount := r.Qty * length * width * r.Rate
	net := amount - r.Discount + r.Charges
	tax := (r.Cgst + r.Sgst + r.Igst) / 100
	return net * (1 + tax)
}

func jobRowsTotal(rows []store.JobRowWrite) float64 {
	var sum float64
	for _, r := range rows {
		sum += jobRowGrossTotal(r)
	}
	return sum
}

// Create is POST /lifecycle/jobs/create.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)

	clientID := form.String("client_id")
	challan := form.String("challanNumber")
	if clientID == "" || challan == "" || !form.Has("rows") {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	var raw []jobRawRow
	if err := form.JSON("rows", &raw); err != nil {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "rows must be a JSON array.", Status: httpx.False()})
		return
	}
	if len(raw) == 0 {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "A job needs at least one row.", Status: httpx.False()})
		return
	}
	rows := make([]store.JobRowWrite, 0, len(raw))
	for _, rr := range raw {
		rows = append(rows, store.JobRowWrite{
			Material: rr.Material, Description: rr.Description, Length: rr.Length, Width: rr.Width,
			Qty: rr.Qty, Rate: rr.Rate, Cgst: rr.Cgst, Sgst: rr.Sgst, Igst: rr.Igst,
			Discount: rr.Discount, Charges: rr.Charges, QuotationID: store.ID(rr.QuotationID),
		})
	}
	progress := "Unassigned"
	if form.Has("employee_id") || form.Has("vendor_id") {
		progress = "In Progress"
	}
	job, dup, err := h.store.Create(r.Context(), store.JobCreateInput{
		UID: sess.UID, CompanyID: sess.CompanyID, ClientID: store.ID(clientID),
		EmployeeID: store.ID(form.String("employee_id")), VendorID: store.ID(form.String("vendor_id")),
		ChallanNumber: challan, ReceivedDate: form.String("receivedDate"),
		Advance: numOrZero(form.String("advance")), Total: jobRowsTotal(rows), Progress: progress, Rows: rows,
		Actor: actorOf(sess),
	})
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if dup {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "This job number is already in use.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Job created.", Data: build(job)})
}

// Update is POST /lifecycle/jobs/update: partial edit + row diff (converted rows are immutable).
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)

	jobID := form.String("job_id")
	if jobID == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	in := store.JobUpdateInput{UID: sess.UID, CompanyID: sess.CompanyID, JobID: store.ID(jobID), Actor: actorOf(sess)}
	// client_id / challanNumber / receivedDate change only when non-empty (Node's `!== undefined`
	// with a truthy body value; the edit form always sends these, so Present is the faithful gate).
	if form.Present("client_id") {
		v := store.ID(form.String("client_id"))
		in.ClientID = &v
	}
	if form.Present("challanNumber") {
		v := form.String("challanNumber")
		in.ChallanNumber = &v
	}
	if form.Present("receivedDate") {
		v := form.String("receivedDate")
		in.ReceivedDate = &v
	}
	if form.Present("advance") {
		v := numOrZero(form.String("advance"))
		in.Advance = &v
	}
	if form.Present("rows") {
		var raw []jobUpdateRawRow
		if err := form.JSON("rows", &raw); err != nil {
			httpx.Write(w, httpx.Envelope{Code: 422, Message: "rows must be a JSON array.", Status: httpx.False()})
			return
		}
		in.RowsSet = true
		for _, rr := range raw {
			in.Rows = append(in.Rows, store.JobRowPatch{
				ID: store.ID(rr.ID), Material: rr.Material, Description: rr.Description, Length: rr.Length, Width: rr.Width,
				Qty: rr.Qty, Rate: rr.Rate, Cgst: rr.Cgst, Sgst: rr.Sgst, Igst: rr.Igst,
				Discount: rr.Discount, Charges: rr.Charges, QuotationID: store.ID(rr.QuotationID),
			})
		}
	}
	job, found, dup, empty, err := h.store.Update(r.Context(), in)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if empty {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "A job needs at least one row.", Status: httpx.False()})
		return
	}
	if dup {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "This job number is already in use.", Status: httpx.False()})
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Job not found.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Job updated.", Data: build(job)})
}

// jobUpdateRawRow adds the row _id to the create row shape.
type jobUpdateRawRow struct {
	ID string
	jobRawRow
}

func (r *jobUpdateRawRow) UnmarshalJSON(b []byte) error {
	if err := r.jobRawRow.UnmarshalJSON(b); err != nil {
		return err
	}
	var m struct {
		ID string `json:"_id"`
	}
	_ = json.Unmarshal(b, &m)
	r.ID = m.ID
	return nil
}

// ConvertToEntries is POST /lifecycle/jobs/convert-to-entries (admin): turn Done job rows into
// billable entries.
func (h *Handler) ConvertToEntries(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	if !form.Has("job_ids") {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	var ids []string
	if err := form.JSON("job_ids", &ids); err != nil {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "job_ids must be a JSON array of ids.", Status: httpx.False()})
		return
	}
	if len(ids) == 0 {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "job_ids must be a non-empty array.", Status: httpx.False()})
		return
	}
	jobIDs := make([]store.ID, len(ids))
	for i, s := range ids {
		jobIDs[i] = store.ID(s)
	}
	entries, jobs, found, err := h.store.ConvertToEntries(r.Context(), sess.UID, sess.CompanyID, jobIDs, actorOf(sess))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "No convertible jobs found - jobs must have at least one Done, unconverted row.", Status: httpx.False()})
		return
	}
	entryDTO := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		entryDTO = append(entryDTO, map[string]any{
			"_id": string(e.ID), "description": e.Description, "material": e.Material, "hsn": e.Hsn,
			"rate": e.Rate, "qty": e.Qty, "length": e.Length, "width": e.Width, "date": e.Date,
			"amount": e.Amount, "cgst": e.Cgst, "sgst": e.Sgst, "igst": e.Igst,
			"discount": e.Discount, "charges": e.Charges, "advance": e.Advance, "total": e.Total, "has_issued": false,
		})
	}
	jobsOut := make([]jobDTO, 0, len(jobs))
	for _, jb := range jobs {
		jobsOut = append(jobsOut, build(jb))
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Converted to entries.", Data: map[string]any{"entries": entryDTO, "jobs": jobsOut}})
}

// Delete is POST /lifecycle/jobs/delete: move a job to trash (snapshot + "Trashed" history +
// removal). Refused with 403 when the invoice lock forbids deletion. Sends status:true, no data.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	jobID := form.String("job_id")
	if jobID == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	status, err := h.store.Delete(r.Context(), sess.UID, sess.CompanyID, store.ID(jobID), actorOf(sess))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	switch status {
	case store.JobTxJobNotFound:
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Job not found.", Status: httpx.False()})
		return
	case store.JobTxLocked:
		httpx.Write(w, httpx.Envelope{Code: 403, Message: "This job is invoiced and cannot be deleted. Delete the invoice first.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Job moved to trash.", Status: httpx.True()})
}

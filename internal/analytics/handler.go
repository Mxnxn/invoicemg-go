// Package analytics serves the reporting reads from routes/Analytics.js. Only /analytics/revenue
// is ported so far - the Revenue tab's billed/collected trend, honouring the Invoiced/All switch.
// Behind the analytics feature.
package analytics

import (
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/datebuckets"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type Handler struct {
	store store.Analytics
	now   func() time.Time
}

func New(s store.Analytics) *Handler {
	return &Handler{store: s, now: func() time.Time { return time.Now().UTC() }}
}

func round2(n float64) float64 { return math.Floor(n*100+0.5) / 100 }

// Revenue is POST /analytics/revenue. Requires `period` (weekly|monthly|yearly); optional
// focusYear/focusMonth drill-in; `source` all|invoiced (default invoiced).
func (h *Handler) Revenue(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)

	period := form.String("period")
	if period == "" {
		httpx.Invalid(w, "")
		return
	}
	now := h.now()
	hasYear := form.Has("focusYear")
	hasMonth := form.Has("focusMonth")
	focusYear := form.Int("focusYear", 0)
	focusMonth := form.Int("focusMonth", 0)

	var buckets []datebuckets.Bucket
	var yearAgo *datebuckets.Bucket
	switch period {
	case "weekly":
		if hasYear && hasMonth {
			buckets = datebuckets.WeeksForMonth(focusYear, focusMonth)
		} else {
			buckets = datebuckets.WeeklyBuckets(now, 8)
			b := datebuckets.WeeklyBuckets(now, 53)[0] // 52 weeks back
			yearAgo = &b
		}
	case "yearly":
		buckets = datebuckets.YearlyBuckets(now, 15, 0)
	default: // monthly
		if hasYear {
			buckets = datebuckets.MonthsForYear(focusYear)
		} else {
			buckets = datebuckets.MonthlyBuckets(now, 36)
			b := datebuckets.MonthlyBuckets(now, 13)[0]
			yearAgo = &b
		}
	}

	allBuckets := buckets
	if yearAgo != nil {
		allBuckets = append(append([]datebuckets.Bucket{}, buckets...), *yearAgo)
	}

	source := "invoiced"
	if form.String("source") == "all" {
		source = "all"
	}
	billed, collected, err := h.store.RevenueSeries(r.Context(), sess.CompanyID, source)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	billedSums := datebuckets.SumIntoBuckets(toDated(billed), allBuckets)
	collectedSums := datebuckets.SumIntoBuckets(toDated(collected), allBuckets)

	data := make([]point, 0, len(buckets))
	for i, b := range buckets {
		data = append(data, point{Label: b.Label, Billed: round2(billedSums[i]), Collected: round2(collectedSums[i])})
	}
	var yearAgoOut *point
	if yearAgo != nil {
		last := len(allBuckets) - 1
		yearAgoOut = &point{Label: yearAgo.Label, Billed: round2(billedSums[last]), Collected: round2(collectedSums[last])}
	}

	// Custom envelope: data plus a TOP-LEVEL yearAgo (not nested under data), so it is encoded
	// directly rather than through httpx.Write. Always HTTP 200 like the Node route.
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(revenueEnvelope{Code: 200, Message: "Operation successful.", Data: data, YearAgo: yearAgoOut})
}

func toDated(in []store.DatedAmount) []datebuckets.Dated {
	out := make([]datebuckets.Dated, 0, len(in))
	for _, d := range in {
		out = append(out, datebuckets.Dated{Date: d.Date, Amount: d.Amount})
	}
	return out
}

// revenue sends data AND a top-level yearAgo (not nested under the envelope's data), so it needs
// its own envelope shape rather than httpx.Envelope.
type revenueEnvelope struct {
	Code    int     `json:"code"`
	Message string  `json:"message"`
	Data    []point `json:"data"`
	YearAgo *point  `json:"yearAgo"`
}

type point struct {
	Label     string  `json:"label"`
	Billed    float64 `json:"billed"`
	Collected float64 `json:"collected"`
}

// TopSales / TopCredits / TopPaid are the Customers-tab client rankings. Each names its value
// field as the Node route does: amount / due / paid.
func (h *Handler) TopSales(w http.ResponseWriter, r *http.Request) {
	h.rank(w, r, "amount", func(s store.Analytics, companyID store.ID) ([]store.ClientRank, error) {
		return s.TopSales(r.Context(), companyID)
	})
}
func (h *Handler) TopCredits(w http.ResponseWriter, r *http.Request) {
	h.rank(w, r, "due", func(s store.Analytics, companyID store.ID) ([]store.ClientRank, error) {
		return s.TopCredits(r.Context(), companyID)
	})
}
func (h *Handler) TopPaid(w http.ResponseWriter, r *http.Request) {
	h.rank(w, r, "paid", func(s store.Analytics, companyID store.ID) ([]store.ClientRank, error) {
		return s.TopPaid(r.Context(), companyID)
	})
}

func (h *Handler) rank(w http.ResponseWriter, r *http.Request, field string, fn func(store.Analytics, store.ID) ([]store.ClientRank, error)) {
	sess := auth.MustFrom(r.Context())
	list, err := fn(h.store, sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, c := range list {
		out = append(out, map[string]any{
			"clientId": string(c.ClientID), "clientName": c.ClientName, "clientFirm": c.ClientFirm, field: c.Value,
		})
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: out})
}

func avg(xs []float64) (float64, int) {
	if len(xs) == 0 {
		return 0, 0
	}
	var s float64
	for _, x := range xs {
		s += x
	}
	a := s / float64(len(xs))
	if a < 1 {
		a = 1 // Node clamps a sub-day average up to 1
	}
	return a, len(xs)
}

func round1(n float64) float64 { return math.Floor(n*10+0.5) / 10 }

// AvgPaymentTime is POST /analytics/avg-payment-time.
func (h *Handler) AvgPaymentTime(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	gaps, err := h.store.PaymentGaps(r.Context(), sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	a, n := avg(gaps)
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: map[string]any{"avgDays": round1(a), "count": n}})
}

// AvgPendingTime is POST /analytics/avg-pending-time.
func (h *Handler) AvgPendingTime(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	since, err := h.store.PendingSince(r.Context(), sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	now := h.now()
	gaps := make([]float64, 0, len(since))
	for _, t := range since {
		if d := now.Sub(t).Hours() / 24; d >= 0 {
			gaps = append(gaps, d)
		}
	}
	a, n := avg(gaps)
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: map[string]any{"avgDays": round1(a), "count": n}})
}

// Payables is POST /analytics/payables - totalPayable + bySupplier at the TOP level (not under
// data), with status:true, exactly as the Node route sends it.
func (h *Handler) Payables(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	total, bySupplier, err := h.store.Payables(r.Context(), sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	sup := make([]map[string]any, 0, len(bySupplier))
	for _, s := range bySupplier {
		sup = append(sup, map[string]any{"name": s.Name, "due": s.Due})
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code": 200, "message": "Operation successful.", "status": true,
		"totalPayable": total, "bySupplier": sup,
	})
}

// Aging is POST /analytics/aging - outstanding AR bucketed by age.
func (h *Handler) Aging(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	list, err := h.store.OutstandingInvoices(r.Context(), sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	now := h.now()
	type bd struct {
		label    string
		min, max float64
	}
	defs := []bd{{"0-30 days", 0, 30}, {"31-60 days", 30, 60}, {"61-90 days", 60, 90}, {"90+ days", 90, math.Inf(1)}}
	sums := make([]float64, len(defs))
	counts := make([]int, len(defs))
	var totalOutstanding, oldestDays float64
	for _, iv := range list {
		age := now.Sub(iv.Date).Hours() / 24
		for i, b := range defs {
			if age >= b.min && age < b.max {
				sums[i] += iv.Amount
				counts[i]++
				totalOutstanding += iv.Amount
				if age > oldestDays {
					oldestDays = age
				}
				break
			}
		}
	}
	data := make([]map[string]any, len(defs))
	for i, b := range defs {
		data[i] = map[string]any{"label": b.label, "amount": round2(sums[i]), "count": counts[i]}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code": 200, "message": "Operation successful.", "data": data,
		"totalOutstanding": round2(totalOutstanding), "oldestDays": math.Floor(oldestDays + 0.5),
	})
}

// Unbilled is POST /analytics/unbilled - done-but-not-invoiced work.
func (h *Handler) Unbilled(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	entries, err := h.store.UnbilledEntries(r.Context(), sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	now := h.now()
	var totalValue, ageSum float64
	ageCount := 0
	byClient := map[string]*unbilledAcc{}
	order := []string{}
	for _, e := range entries {
		totalValue += e.Value
		if e.HasDate {
			ageSum += now.Sub(e.Date).Hours() / 24
			ageCount++
		}
		if e.ClientID == "" {
			continue
		}
		cid := string(e.ClientID)
		if byClient[cid] == nil {
			byClient[cid] = &unbilledAcc{id: cid, name: e.ClientName, firm: e.ClientFirm}
			order = append(order, cid)
		}
		byClient[cid].value += e.Value
		byClient[cid].count++
	}
	ranked := make([]*unbilledAcc, 0, len(order))
	for _, cid := range order {
		ranked = append(ranked, byClient[cid])
	}
	sortAcc(ranked)
	top := ranked
	if len(top) > 5 {
		top = top[:5]
	}
	topClients := make([]map[string]any, 0, len(top))
	for _, c := range top {
		topClients = append(topClients, map[string]any{"clientId": c.id, "clientName": c.name, "clientFirm": c.firm, "value": round2(c.value), "count": c.count})
	}
	avgAge := 0.0
	if ageCount > 0 {
		avgAge = round1(ageSum / float64(ageCount))
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code": 200, "message": "Operation successful.", "count": len(entries),
		"totalValue": round2(totalValue), "avgAgeDays": avgAge, "topClients": topClients,
	})
}

type unbilledAcc struct {
	id, name, firm string
	value          float64
	count          int
}

func sortAcc(xs []*unbilledAcc) {
	sort.SliceStable(xs, func(i, j int) bool { return xs[i].value > xs[j].value })
}

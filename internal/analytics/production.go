package analytics

import (
	"math"
	"net/http"
	"sort"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/datebuckets"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/stagetiming"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

func emptyBuckets() map[string]int {
	m := make(map[string]int, len(stagetiming.AgeBuckets))
	for _, b := range stagetiming.AgeBuckets {
		m[b] = 0
	}
	return m
}

type wipRow struct {
	Name    string         `json:"name"`
	ID      *string        `json:"id"`
	Total   int            `json:"total"`
	Buckets map[string]int `json:"buckets"`
}

type wipUnassigned struct {
	Name    string         `json:"name"`
	Total   int            `json:"total"`
	Buckets map[string]int `json:"buckets"`
}

// ProductionWip is POST /analytics/production/wip: open cards bucketed by stage, holder and age.
// Dwell is measured from the last "Queue advanced" into the current stage, falling back to the
// card's creation when no such event exists (agesExact then reports the gap).
func (h *Handler) ProductionWip(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	data, err := h.store.ProductionWip(r.Context(), sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	now := h.now()

	// job|rowKey -> when it last entered a stage (events are ascending, so the last write wins).
	enteredAt := map[string]int64{}
	for _, e := range data.Events {
		rowKey := e.RowKey
		if e.ToStage == "" { // no structured fields: parse the display detail
			p := stagetiming.ParseQueueAdvance(e.Detail)
			if !p.Ok {
				continue
			}
			rowKey = p.RowKey
		}
		enteredAt[string(e.JobID)+"|"+rowKey] = e.CreatedAt.UnixMilli()
	}

	stageMap := map[string]*wipRow{}
	holderMap := map[string]*wipRow{}
	holderOrder := []string{}
	unassigned := wipUnassigned{Name: "Nobody assigned", Buckets: emptyBuckets()}
	totalOpen := 0
	oldestDays := 0.0
	agesExact := true

	for _, c := range data.Cards {
		if stagetiming.FoldStage(c.Queue) == "Done" {
			continue // isOpenStage
		}
		entered, ok := enteredAt[string(c.JobID)+"|"+c.Key]
		if !ok {
			agesExact = false
			entered = c.CreatedAt.UnixMilli()
		}
		days := stagetiming.Round1(math.Max(0, float64(now.UnixMilli()-entered)/stagetiming.MsPerDay))
		bucket := stagetiming.AgeBucket(days)
		totalOpen++
		if days > oldestDays {
			oldestDays = days
		}

		fold := stagetiming.FoldStage(c.Queue)
		row, ok := stageMap[fold]
		if !ok {
			row = &wipRow{Name: fold, ID: nil, Buckets: emptyBuckets()}
			stageMap[fold] = row
		}
		row.Total++
		row.Buckets[bucket]++

		if c.EmployeeID != "" {
			id := string(c.EmployeeID)
			hr, ok := holderMap[id]
			if !ok {
				idCopy := id
				hr = &wipRow{ID: &idCopy, Buckets: emptyBuckets()}
				holderMap[id] = hr
				holderOrder = append(holderOrder, id)
			}
			hr.Total++
			hr.Buckets[bucket]++
		} else {
			unassigned.Total++
			unassigned.Buckets[bucket]++
		}
	}

	// Stage order: the defaults minus Done, then the catch-all, keeping only stages seen.
	stageOrder := append([]string{}, stagetiming.DefaultStages...)
	stages := make([]*wipRow, 0)
	for _, s := range stageOrder {
		if s == "Done" {
			continue
		}
		if row, ok := stageMap[s]; ok {
			stages = append(stages, row)
		}
	}
	if row, ok := stageMap[stagetiming.OtherStage]; ok {
		stages = append(stages, row)
	}

	holders := make([]*wipRow, 0, len(holderOrder))
	for _, id := range holderOrder {
		hr := holderMap[id]
		hr.Name = "Unknown"
		if n, ok := data.PersonNames[store.ID(id)]; ok && n != "" {
			hr.Name = n
		}
		holders = append(holders, hr)
	}
	sort.SliceStable(holders, func(i, j int) bool { return holders[i].Total > holders[j].Total })

	httpx.Write(w, httpx.Envelope{Code: 200, Status: httpx.True(), Data: map[string]any{
		"stages": stages, "holders": holders, "unassigned": unassigned,
		"totalOpen": totalOpen, "oldestDays": oldestDays, "agesExact": agesExact,
	}})
}

type throughputBucket struct {
	Label     string `json:"label"`
	Completed int    `json:"completed"`
}

type personCount struct {
	Name      string `json:"name"`
	Completed int    `json:"completed"`
}

// ProductionThroughput is POST /analytics/production/throughput: how much work finished per period
// (12 weekly or monthly buckets), who finished it, and the job-level cycle-time median/p90.
func (h *Handler) ProductionThroughput(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)

	period := "weekly"
	if form.String("period") == "monthly" {
		period = "monthly"
	}
	var buckets []datebuckets.Bucket
	if period == "monthly" {
		buckets = datebuckets.MonthlyBuckets(h.now(), 12)
	} else {
		buckets = datebuckets.WeeklyBuckets(h.now(), 12)
	}
	windowStart := buckets[0].Start

	data, err := h.store.ProductionThroughput(r.Context(), sess.CompanyID, windowStart)
	if err != nil {
		httpx.Internal(w, err)
		return
	}

	trend := make([]throughputBucket, len(buckets))
	for i, b := range buckets {
		trend[i] = throughputBucket{Label: b.Label, Completed: 0}
	}
	perPerson := map[string]int{}
	perPersonOrder := []string{}

	for _, e := range data.Events {
		to := e.ToStage
		if to == "" {
			p := stagetiming.ParseQueueAdvance(e.Detail)
			if !p.Ok {
				continue
			}
			to = p.To
		}
		if stagetiming.FoldStage(to) != "Done" {
			continue
		}
		at := e.CreatedAt
		for i, b := range buckets {
			if !at.Before(b.Start) && at.Before(b.End) {
				trend[i].Completed++
				break
			}
		}
		who := e.ActorName
		if who == "" {
			who = "Unknown"
		}
		if _, seen := perPerson[who]; !seen {
			perPersonOrder = append(perPersonOrder, who)
		}
		perPerson[who]++
	}

	cycleDays := []float64{}
	for _, j := range data.DoneJobs {
		queues := j.RowQueues
		if len(queues) == 0 {
			queues = []string{j.Queue}
		}
		allDone := true
		for _, q := range queues {
			if stagetiming.FoldStage(q) != "Done" {
				allDone = false
				break
			}
		}
		if !allDone {
			continue
		}
		cycleDays = append(cycleDays, stagetiming.Round1(math.Max(0, float64(j.UpdatedAt.UnixMilli()-j.CreatedAt.UnixMilli())/stagetiming.MsPerDay)))
	}

	people := make([]personCount, 0, len(perPersonOrder))
	for _, name := range perPersonOrder {
		people = append(people, personCount{Name: name, Completed: perPerson[name]})
	}
	sort.SliceStable(people, func(i, j int) bool { return people[i].Completed > people[j].Completed })

	httpx.Write(w, httpx.Envelope{Code: 200, Status: httpx.True(), Data: map[string]any{
		"trend":    trend,
		"perPerson": people,
		"cycleTimeDays": map[string]any{
			"median": stagetiming.Median(cycleDays),
			"p90":    stagetiming.Percentile(cycleDays, 90),
			"count":  len(cycleDays),
		},
	}})
}

package analytics

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/datebuckets"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

var prodNow = time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)

func ago(days int) time.Time { return prodNow.AddDate(0, 0, -days) }

func decodeProd(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, rec.Body.String())
	}
	if body["code"] != float64(200) {
		t.Fatalf("code = %v (%s)", body["code"], rec.Body.String())
	}
	return body["data"].(map[string]any)
}

func TestProductionWip_BucketsStagesHoldersAges(t *testing.T) {
	s := &stubAnalytics{wip: store.ProductionWipData{
		Cards: []store.ProductionCard{
			{JobID: "j1", Key: "r1", Queue: "Printing", EmployeeID: "p1", CreatedAt: ago(20)},
			{JobID: "j1", Key: "r2", Queue: "Done", EmployeeID: "p1", CreatedAt: ago(20)}, // closed -> skipped
			{JobID: "j2", Key: "", Queue: "Created", CreatedAt: ago(14)},                  // unassigned, no event
		},
		Events: []store.ProductionEvent{
			{JobID: "j1", ToStage: "Printing", RowKey: "r1", CreatedAt: ago(2)}, // r1 entered Printing 2d ago
		},
		PersonNames: map[store.ID]string{"p1": "Alice"},
	}}
	h := New(s)
	h.now = func() time.Time { return prodNow }

	r := httptest.NewRequest("POST", "/", nil)
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	h.ProductionWip(rec, r)
	data := decodeProd(t, rec)

	if data["totalOpen"] != float64(2) {
		t.Errorf("totalOpen = %v, want 2 (Done card skipped)", data["totalOpen"])
	}
	if data["oldestDays"] != float64(14) {
		t.Errorf("oldestDays = %v, want 14 (the unassigned Created card)", data["oldestDays"])
	}
	if data["agesExact"] != false {
		t.Errorf("agesExact = %v, want false (j2 had no queue-advanced event)", data["agesExact"])
	}
	stages := data["stages"].([]any)
	// Stage order: Created before Printing.
	if len(stages) != 2 || stages[0].(map[string]any)["name"] != "Created" || stages[1].(map[string]any)["name"] != "Printing" {
		t.Fatalf("stages = %v, want [Created, Printing]", stages)
	}
	if stages[0].(map[string]any)["id"] != nil {
		t.Error("stage rows carry id:null")
	}
	holders := data["holders"].([]any)
	if len(holders) != 1 || holders[0].(map[string]any)["name"] != "Alice" {
		t.Fatalf("holders = %v, want [Alice]", holders)
	}
	// Alice's card is 2 days old -> the 0-2d bucket.
	aliceBuckets := holders[0].(map[string]any)["buckets"].(map[string]any)
	if aliceBuckets["0-2d"] != float64(1) {
		t.Errorf("Alice buckets = %v, want 0-2d:1", aliceBuckets)
	}
	un := data["unassigned"].(map[string]any)
	if un["name"] != "Nobody assigned" || un["total"] != float64(1) {
		t.Errorf("unassigned = %v", un)
	}
	if un["buckets"].(map[string]any)["8-14d"] != float64(1) {
		t.Errorf("unassigned bucket = %v, want 8-14d:1", un["buckets"])
	}
}

func TestProductionThroughput_TrendPeopleCycle(t *testing.T) {
	s := &stubAnalytics{throughput: store.ProductionThroughputData{
		Events: []store.ProductionEvent{
			{JobID: "j1", ToStage: "Done", ActorName: "Alice", CreatedAt: ago(2)},
			{JobID: "j2", ToStage: "Done", ActorName: "Bob", CreatedAt: ago(3)},
			{JobID: "j3", ToStage: "Done", ActorName: "Bob", CreatedAt: ago(4)},
			{JobID: "j4", ToStage: "Printing", ActorName: "Carol", CreatedAt: ago(1)}, // not Done -> skipped
		},
		DoneJobs: []store.ProductionDoneJob{
			{CreatedAt: ago(10), UpdatedAt: ago(3), Queue: "Done", RowQueues: []string{"Done", "Done"}}, // 7d
			{CreatedAt: ago(5), UpdatedAt: ago(4), RowQueues: []string{"Done", "Printing"}},             // not all done -> skip
			{CreatedAt: ago(2), UpdatedAt: ago(1), Queue: "Done"},                                       // rowless, uses job queue -> 1d
		},
	}}
	h := New(s)
	h.now = func() time.Time { return prodNow }

	r := httptest.NewRequest("POST", "/", strings.NewReader(url.Values{}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	h.ProductionThroughput(rec, r)
	data := decodeProd(t, rec)

	// windowStart is the first weekly bucket's start.
	wantStart := datebuckets.WeeklyBuckets(prodNow, 12)[0].Start
	if !s.gotWindowStart.Equal(wantStart) {
		t.Errorf("windowStart = %v, want %v", s.gotWindowStart, wantStart)
	}
	trend := data["trend"].([]any)
	if len(trend) != 12 {
		t.Fatalf("trend = %d buckets, want 12", len(trend))
	}
	total := 0.0
	for _, b := range trend {
		total += b.(map[string]any)["completed"].(float64)
	}
	if total != 3 {
		t.Errorf("total completed across trend = %v, want 3 (Carol's non-Done skipped)", total)
	}
	people := data["perPerson"].([]any)
	if len(people) != 2 || people[0].(map[string]any)["name"] != "Bob" || people[0].(map[string]any)["completed"] != float64(2) {
		t.Fatalf("perPerson = %v, want Bob(2) first", people)
	}
	cycle := data["cycleTimeDays"].(map[string]any)
	if cycle["count"] != float64(2) || cycle["median"] != float64(4) || cycle["p90"] != 6.4 {
		t.Errorf("cycleTimeDays = %v, want count2/median4/p90 6.4", cycle)
	}
}

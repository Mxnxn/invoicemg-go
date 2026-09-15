package batchreceive

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"strings"
	"testing"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type stub struct {
	openJobs    []store.BatchOpenJob
	created     store.BatchReceive
	deleteFound bool
	gotCreate   store.BatchReceiveWrite
	gotDeleteID store.ID
}

func (s *stub) List(context.Context, store.ID, store.ID, store.ID) ([]store.BatchReceive, error) {
	return nil, nil
}
func (s *stub) OpenJobs(context.Context, store.ID, store.ID, store.ID) ([]store.BatchOpenJob, error) {
	return s.openJobs, nil
}
func (s *stub) Create(_ context.Context, _, _ store.ID, in store.BatchReceiveWrite) (store.BatchReceive, error) {
	s.gotCreate = in
	return s.created, nil
}
func (s *stub) Delete(_ context.Context, _, _, id store.ID) (bool, error) {
	s.gotDeleteID = id
	return s.deleteFound, nil
}

func post(t *testing.T, s store.BatchReceives, fn func(*Handler) http.HandlerFunc, fields map[string]string) map[string]any {
	t.Helper()
	vals := neturl.Values{}
	for k, v := range fields {
		vals.Set(k, v)
	}
	r := httptest.NewRequest("POST", "/", strings.NewReader(vals.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	fn(New(s))(rec, r)
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("not json: %v (%s)", err, rec.Body.String())
	}
	return out
}

func TestOpenJobs(t *testing.T) {
	s := &stub{openJobs: []store.BatchOpenJob{{ID: "j1", ChallanNumber: "C-1", Total: 1000, Advance: 400, Remaining: 600, EntryCount: 3}}}
	body := post(t, s, func(h *Handler) http.HandlerFunc { return h.OpenJobs }, map[string]string{"client_id": "c1"})
	row := body["data"].([]any)[0].(map[string]any)
	if row["remaining"] != float64(600) || row["entryCount"] != float64(3) || row["challanNumber"] != "C-1" {
		t.Errorf("open job row: %v", row)
	}
	body = post(t, &stub{}, func(h *Handler) http.HandlerFunc { return h.OpenJobs }, map[string]string{})
	if body["code"] != float64(422) {
		t.Errorf("missing client_id: %v", body)
	}
}

func TestCreate_Validation(t *testing.T) {
	base := map[string]string{"client_id": "c1", "amount": "500", "mode": "auto"}
	// missing mode
	body := post(t, &stub{}, func(h *Handler) http.HandlerFunc { return h.Create }, map[string]string{"client_id": "c1", "amount": "500"})
	if body["code"] != float64(422) || body["message"] != "Invalid request." {
		t.Errorf("missing mode: %v", body)
	}
	// amount not > 0
	b2 := map[string]string{"client_id": "c1", "amount": "0", "mode": "auto"}
	body = post(t, &stub{}, func(h *Handler) http.HandlerFunc { return h.Create }, b2)
	if body["message"] != "Amount must be greater than 0." {
		t.Errorf("amount: %v", body)
	}
	// bad mode
	b3 := map[string]string{"client_id": "c1", "amount": "500", "mode": "weird"}
	body = post(t, &stub{}, func(h *Handler) http.HandlerFunc { return h.Create }, b3)
	if body["message"] != "mode must be 'auto' or 'manual'." {
		t.Errorf("mode: %v", body)
	}
	_ = base
}

func TestCreate_ManualAllocations(t *testing.T) {
	s := &stub{created: store.BatchReceive{ID: "br1", Amount: 500, Mode: "manual"}}
	body := post(t, s, func(h *Handler) http.HandlerFunc { return h.Create }, map[string]string{
		"client_id": "c1", "amount": "500", "mode": "manual",
		"allocations": `[{"job_id":"j1","amount":300},{"job_id":"j2","amount":200}]`,
	})
	if body["code"] != float64(200) || body["message"] != "Batch receive created." {
		t.Fatalf("envelope: %v", body)
	}
	if len(s.gotCreate.Allocations) != 2 || s.gotCreate.Allocations[0].Amount != 300 {
		t.Errorf("allocations passed: %+v", s.gotCreate.Allocations)
	}
	// over-allocation is rejected before the store
	body = post(t, &stub{}, func(h *Handler) http.HandlerFunc { return h.Create }, map[string]string{
		"client_id": "c1", "amount": "500", "mode": "manual",
		"allocations": `[{"job_id":"j1","amount":600}]`,
	})
	if body["message"] != "Allocated amount exceeds the batch amount." {
		t.Errorf("over-alloc: %v", body)
	}
	// manual with no allocations
	body = post(t, &stub{}, func(h *Handler) http.HandlerFunc { return h.Create }, map[string]string{
		"client_id": "c1", "amount": "500", "mode": "manual", "allocations": "[]",
	})
	if body["message"] != "Manual mode needs at least one job allocation." {
		t.Errorf("empty alloc: %v", body)
	}
}

func TestCreate_AutoPassesThrough(t *testing.T) {
	s := &stub{created: store.BatchReceive{ID: "br1", Mode: "auto"}}
	body := post(t, s, func(h *Handler) http.HandlerFunc { return h.Create }, map[string]string{
		"client_id": "c1", "amount": "1000", "mode": "auto", "note": "cash",
	})
	if body["code"] != float64(200) {
		t.Fatalf("envelope: %v", body)
	}
	if s.gotCreate.Mode != "auto" || s.gotCreate.Amount != 1000 || s.gotCreate.Note != "cash" {
		t.Errorf("create input: %+v", s.gotCreate)
	}
	if len(s.gotCreate.Allocations) != 0 {
		t.Errorf("auto mode should carry no manual allocations: %+v", s.gotCreate.Allocations)
	}
}

func TestDelete(t *testing.T) {
	body := post(t, &stub{deleteFound: true}, func(h *Handler) http.HandlerFunc { return h.Delete }, map[string]string{"batch_id": "br1"})
	if body["code"] != float64(200) || body["status"] != true || body["message"] != "Batch receive deleted." {
		t.Errorf("delete ok: %v", body)
	}
	body = post(t, &stub{deleteFound: false}, func(h *Handler) http.HandlerFunc { return h.Delete }, map[string]string{"batch_id": "br1"})
	if body["code"] != float64(404) {
		t.Errorf("delete 404: %v", body)
	}
	body = post(t, &stub{}, func(h *Handler) http.HandlerFunc { return h.Delete }, map[string]string{})
	if body["code"] != float64(422) {
		t.Errorf("missing id: %v", body)
	}
}

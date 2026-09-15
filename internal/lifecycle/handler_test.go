package lifecycle

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type stubJobs struct {
	list        []store.Job
	gotClientID store.ID
}

func (s *stubJobs) List(_ context.Context, _, _, clientID store.ID) ([]store.Job, error) {
	s.gotClientID = clientID
	return s.list, nil
}

func serve(t *testing.T, s store.Jobs) map[string]any {
	t.Helper()
	r := httptest.NewRequest("POST", "/", nil)
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	New(s).List(rec, r)
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("not json: %v (%s)", err, rec.Body.String())
	}
	return out
}

func boolPtr(b bool) *bool { return &b }

func TestJobsList_ShapeAndDerivedState(t *testing.T) {
	s := &stubJobs{list: []store.Job{{
		ID: "j1", ChallanNumber: "JOB/26-27/000001", ReceivedDate: "2026-09-11", Total: 5000,
		Queue: "Printing", Progress: "Unassigned",
		Client:   &store.JobClient{ID: "c1", ClientName: "Priya", ClientFirm: "Acme"},
		Employee: &store.JobPerson{ID: "p1", Name: "Anita"},
		Rows: []store.JobRow{
			{ID: "r1", Material: "Vinyl", Qty: 10, Rate: 45, HasDimensions: boolPtr(false), Queue: "Done",
				Entry: &store.JobRowEntry{ID: "e1", HasIssued: true, Total: 100}, EntryIssued: true, InvoiceNumber: "INV/1"},
			{ID: "r2", Material: "Flex", Qty: 1, Rate: 2500, HasDimensions: boolPtr(true), Queue: "Printing"},
		},
	}}}
	body := serve(t, s)

	if body["message"] != "Operation successful." {
		t.Errorf("message = %v", body["message"])
	}
	job := body["data"].([]any)[0].(map[string]any)

	// populated client / employee objects, not ids
	if c, ok := job["client_id"].(map[string]any); !ok || c["clientName"] != "Priya" {
		t.Errorf("client_id not populated: %v", job["client_id"])
	}
	if e, ok := job["employee_id"].(map[string]any); !ok || e["name"] != "Anita" {
		t.Errorf("employee_id not populated: %v", job["employee_id"])
	}
	if job["vendor_id"] != nil {
		t.Errorf("vendor_id should be null, got %v", job["vendor_id"])
	}
	// one of two rows invoiced -> partial
	if job["invoiceState"] != "partial" || job["invoicedRows"] != float64(1) {
		t.Errorf("invoiceState/invoicedRows wrong: %v / %v", job["invoiceState"], job["invoicedRows"])
	}
	if nums, _ := job["invoiceNumbers"].([]any); len(nums) != 1 || nums[0] != "INV/1" {
		t.Errorf("invoiceNumbers wrong: %v", job["invoiceNumbers"])
	}
	// partial -> not ready for invoice
	if job["readyForInvoice"] != false {
		t.Errorf("readyForInvoice should be false")
	}
	// lock: invoiced+locked forbids edits
	lock := job["lock"].(map[string]any)
	if lock["canEditValues"] != false || lock["invoiced"] != true {
		t.Errorf("lock wrong: %v", lock)
	}
	// alerts present with both channels
	alerts := job["alerts"].(map[string]any)
	if _, ok := alerts["created"].(map[string]any); !ok {
		t.Errorf("alerts.created missing: %v", alerts)
	}
	// per-row invoiced flag
	rows := job["rows"].([]any)
	if rows[0].(map[string]any)["invoiced"] != true || rows[1].(map[string]any)["invoiced"] != false {
		t.Errorf("row invoiced flags wrong")
	}
	// row entry_id populated as object
	if ent, ok := rows[0].(map[string]any)["entry_id"].(map[string]any); !ok || ent["has_issued"] != true {
		t.Errorf("row entry_id not populated: %v", rows[0])
	}
}

func TestJobsList_EmptyIsArray(t *testing.T) {
	body := serve(t, &stubJobs{list: nil})
	if rows, ok := body["data"].([]any); !ok || len(rows) != 0 {
		t.Errorf("data should be [], got %v", body["data"])
	}
}

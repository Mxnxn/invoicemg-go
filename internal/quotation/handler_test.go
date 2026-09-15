package quotation

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type stubQuotations struct {
	list        []store.Quotation
	gotClientID store.ID
}

func (s *stubQuotations) List(_ context.Context, _, _, clientID store.ID) ([]store.Quotation, error) {
	s.gotClientID = clientID
	return s.list, nil
}

func serve(t *testing.T, s store.Quotations) map[string]any {
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

func TestList_PopulatedClientAndRows(t *testing.T) {
	now := time.Date(2026, 9, 10, 4, 0, 0, 0, time.UTC)
	s := &stubQuotations{list: []store.Quotation{{
		ID: "q1", UID: "u1", CompanyID: "co1", QuotationNumber: "QT/26-27/000001", Date: "2026-09-10",
		Client:    &store.QuotationClient{ID: "c1", ClientName: "Priya", ClientFirm: "Acme"},
		Rows:      []store.QuotationRow{{ID: "r1", Material: "Vinyl", Qty: 10, Rate: 45}},
		CreatedAt: now, UpdatedAt: now,
	}}}
	body := serve(t, s)

	q := body["data"].([]any)[0].(map[string]any)
	if q["quotationNumber"] != "QT/26-27/000001" {
		t.Errorf("number = %v", q["quotationNumber"])
	}
	// client_id is the POPULATED object, not a string.
	client, ok := q["client_id"].(map[string]any)
	if !ok {
		t.Fatalf("client_id should be a populated object, got %T: %v", q["client_id"], q["client_id"])
	}
	if client["_id"] != "c1" || client["clientName"] != "Priya" || client["clientFirm"] != "Acme" {
		t.Errorf("populated client wrong: %v", client)
	}
	rows := q["rows"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["material"] != "Vinyl" {
		t.Errorf("rows wrong: %v", rows)
	}
	if _, hasV := q["__v"]; !hasV {
		t.Error("__v must be present")
	}
}

// A dangling client (deleted) populates to null, not an object.
func TestList_NullClientWhenMissing(t *testing.T) {
	s := &stubQuotations{list: []store.Quotation{{ID: "q1", QuotationNumber: "QT/1", Client: nil, Rows: nil}}}
	body := serve(t, s)
	q := body["data"].([]any)[0].(map[string]any)
	if v, ok := q["client_id"]; !ok || v != nil {
		t.Errorf("client_id must be present and null, got ok=%v v=%v", ok, v)
	}
	// rows still an array
	if _, ok := q["rows"].([]any); !ok {
		t.Errorf("rows should be [], got %v", q["rows"])
	}
}

func TestList_ClientFilterPassedThrough(t *testing.T) {
	s := &stubQuotations{list: nil}
	r := httptest.NewRequest("POST", "/", nil)
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	// no client_id field -> empty filter
	rec := httptest.NewRecorder()
	New(s).List(rec, r)
	if s.gotClientID != "" {
		t.Errorf("client filter should be empty, got %q", s.gotClientID)
	}
}

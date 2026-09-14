package bank

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type stubBanks struct {
	banks []store.Bank
	err   error
	got   store.ID
}

func (s *stubBanks) List(_ context.Context, companyID store.ID) ([]store.Bank, error) {
	s.got = companyID
	return s.banks, s.err
}

func serve(t *testing.T, s store.Banks, companyID store.ID) (map[string]any, *stubBanks) {
	t.Helper()
	req := httptest.NewRequest("POST", "/bank/list", nil)
	req = req.WithContext(auth.WithSession(req.Context(), store.Session{CompanyID: companyID}))
	rec := httptest.NewRecorder()
	New(s).List(rec, req)
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("not json: %v (%s)", err, rec.Body.String())
	}
	stub, _ := s.(*stubBanks)
	return body, stub
}

// The list is scoped to the acting company and returns the whole record - including __v and the
// timestamps in Date.toJSON form - with no `status` field, matching routes/Bank.js.
func TestList_Shape(t *testing.T) {
	created := time.Date(2026, 9, 1, 8, 30, 0, 0, time.UTC)
	s := &stubBanks{banks: []store.Bank{{
		ID: "b1", UID: "u1", CompanyID: "co1", Name: "HDFC", OpeningBalance: 50000,
		CreatedAt: created, UpdatedAt: created, Version: 0,
	}}}
	body, stub := serve(t, s, "co1")

	if stub.got != "co1" {
		t.Errorf("company scope = %q, want co1", stub.got)
	}
	if body["code"] != float64(200) {
		t.Fatalf("code = %v", body["code"])
	}
	if _, hasStatus := body["status"]; hasStatus {
		t.Error("bank/list must NOT send a status field")
	}
	rows, ok := body["data"].([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("data not a 1-element array: %v", body["data"])
	}
	row := rows[0].(map[string]any)
	if row["_id"] != "b1" || row["name"] != "HDFC" || row["openingBalance"] != float64(50000) {
		t.Errorf("row fields wrong: %v", row)
	}
	if row["company_id"] != "co1" || row["uid"] != "u1" {
		t.Errorf("scope fields wrong: %v", row)
	}
	if _, hasV := row["__v"]; !hasV || row["__v"] != float64(0) {
		t.Errorf("__v must be present and 0 (#21): %v", row["__v"])
	}
	if row["createdAt"] != "2026-09-01T08:30:00.000Z" {
		t.Errorf("createdAt = %v, want Date.toJSON form (#5)", row["createdAt"])
	}
}

// An empty company sends data: [] (a non-nil array), never null or an omitted key (#22).
func TestList_EmptyIsArray(t *testing.T) {
	body, _ := serve(t, &stubBanks{banks: nil}, "co1")
	rows, ok := body["data"].([]any)
	if !ok {
		t.Fatalf("data should be [], got %v", body["data"])
	}
	if len(rows) != 0 {
		t.Errorf("want empty array, got %v", rows)
	}
}

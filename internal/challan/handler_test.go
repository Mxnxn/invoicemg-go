package challan

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type stubChallans struct {
	list []store.Challan
	got  store.ID
}

func (s *stubChallans) List(_ context.Context, companyID store.ID) ([]store.Challan, error) {
	s.got = companyID
	return s.list, nil
}

func TestGetAll(t *testing.T) {
	s := &stubChallans{list: []store.Challan{
		{ID: "ch1", UID: "u1", CompanyID: "co1", CompanyName: "Acme", Description: "Banners", Date: "2026-09-12", Type: "Delivery", Quantity: 5, Amount: 4500},
	}}
	r := httptest.NewRequest("POST", "/", nil)
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	New(s).GetAll(rec, r)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("not json: %v", err)
	}
	if s.got != "co1" {
		t.Errorf("company scope = %q", s.got)
	}
	if body["code"] != float64(200) || body["status"] != true {
		t.Errorf("envelope wrong: %v", body)
	}
	// Challan getAll deliberately sends NO message.
	if _, ok := body["message"]; ok {
		t.Errorf("challan/getAll must not send a message field")
	}
	row := body["data"].([]any)[0].(map[string]any)
	if row["companyName"] != "Acme" || row["quantity"] != float64(5) || row["type"] != "Delivery" {
		t.Errorf("row wrong: %v", row)
	}
}

func TestGetAll_EmptyIsArray(t *testing.T) {
	r := httptest.NewRequest("POST", "/", nil)
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	New(&stubChallans{list: nil}).GetAll(rec, r)
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if rows, ok := body["data"].([]any); !ok || len(rows) != 0 {
		t.Errorf("data should be [], got %v", body["data"])
	}
}

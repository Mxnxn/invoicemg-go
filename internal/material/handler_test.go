package material

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type stubMaterials struct {
	list []store.Material
	got  store.ID
}

func (s *stubMaterials) Visible(_ context.Context, companyID store.ID) ([]store.Material, error) {
	s.got = companyID
	return s.list, nil
}

func serve(t *testing.T, s store.Materials, companyID store.ID) map[string]any {
	t.Helper()
	r := httptest.NewRequest("POST", "/", nil)
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{CompanyID: companyID}))
	rec := httptest.NewRecorder()
	New(s).Getall(rec, r)
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("not json: %v (%s)", err, rec.Body.String())
	}
	return body
}

func share(ids ...store.ID) *[]store.ID { out := append([]store.ID{}, ids...); return &out }

func TestGetall_WholeRecordAndBorrowed(t *testing.T) {
	changed := time.Date(2026, 9, 1, 6, 0, 0, 0, time.UTC)
	s := &stubMaterials{list: []store.Material{
		{ID: "own", UID: "u1", CompanyID: "co1", MaterialName: "Vinyl", MaterialRate: 45, PurchaseRate: 30,
			Unit: "SQ. Ft", Hsn: "4911", Tax: 18, Sharing: share("co1"),
			PriceHistory: []store.PriceHistoryEntry{{MaterialRate: 45, PurchaseRate: 30, ChangedAt: changed}}},
		{ID: "bor", UID: "u1", CompanyID: "co2", MaterialName: "Flex", Sharing: share("co2", "co1")},
	}}
	body := serve(t, s, "co1")

	if s.got != "co1" {
		t.Errorf("company scope = %q", s.got)
	}
	if body["message"] != "Operation successful." {
		t.Errorf("message = %v", body["message"])
	}
	if _, ok := body["status"]; ok {
		t.Error("material/getall sends no status field")
	}
	rows := body["data"].([]any)
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rows))
	}
	own := rows[0].(map[string]any)
	if own["material_name"] != "Vinyl" || own["material_rate"] != float64(45) || own["tax"] != float64(18) {
		t.Errorf("own fields wrong: %v", own)
	}
	if own["borrowed"] != false {
		t.Error("own product must not be borrowed")
	}
	if _, hasV := own["__v"]; !hasV {
		t.Error("__v must be present (#21)")
	}
	ph := own["priceHistory"].([]any)
	if len(ph) != 1 || ph[0].(map[string]any)["changed_at"] != "2026-09-01T06:00:00.000Z" {
		t.Errorf("priceHistory wrong: %v", ph)
	}
	bor := rows[1].(map[string]any)
	if bor["borrowed"] != true {
		t.Error("product owned by co2, shared with co1 must be borrowed")
	}
}

func TestGetall_EmptyIsArray(t *testing.T) {
	body := serve(t, &stubMaterials{list: nil}, "co1")
	if rows, ok := body["data"].([]any); !ok || len(rows) != 0 {
		t.Errorf("data should be [], got %v", body["data"])
	}
}

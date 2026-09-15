package material

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"strings"
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

// stubMaterials satisfies the write half of the interface with no-ops (the read tests use it).
func (s *stubMaterials) Create(context.Context, store.ID, store.ID, store.MaterialWrite) (store.Material, error) {
	return store.Material{}, nil
}
func (s *stubMaterials) Update(context.Context, store.ID, store.ID, store.MaterialWrite) (store.Material, bool, error) {
	return store.Material{}, false, nil
}
func (s *stubMaterials) Delete(context.Context, store.ID, store.ID) error { return nil }

// writeStub drives the write-path tests.
type writeStub struct {
	created     store.Material
	updated     store.Material
	updateFound bool
	gotCreate   store.MaterialWrite
	gotUpdate   store.MaterialWrite
	gotDeleteID store.ID
	deleted     bool
}

func (s *writeStub) Visible(context.Context, store.ID) ([]store.Material, error) { return nil, nil }
func (s *writeStub) Create(_ context.Context, _, _ store.ID, in store.MaterialWrite) (store.Material, error) {
	s.gotCreate = in
	return s.created, nil
}
func (s *writeStub) Update(_ context.Context, _, _ store.ID, in store.MaterialWrite) (store.Material, bool, error) {
	s.gotUpdate = in
	return s.updated, s.updateFound, nil
}
func (s *writeStub) Delete(_ context.Context, _, id store.ID) error {
	s.gotDeleteID, s.deleted = id, true
	return nil
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

func postForm(t *testing.T, s store.Materials, fn func(*Handler) http.HandlerFunc, fields map[string]string) map[string]any {
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
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("not json: %v (%s)", err, rec.Body.String())
	}
	return body
}

func TestAdd_Success(t *testing.T) {
	s := &writeStub{created: store.Material{ID: "m1", UID: "u1", CompanyID: "co1", MaterialName: "Vinyl",
		MaterialRate: 45, PurchaseRate: 30, Hsn: "3919", Tax: 18, PriceHistory: []store.PriceHistoryEntry{}}}
	body := postForm(t, s, func(h *Handler) http.HandlerFunc { return h.Add },
		map[string]string{"material_name": "Vinyl", "material_rate": "45", "purchase_rate": "30", "hsn": "3919", "tax": "18"})
	if body["code"] != float64(200) || body["message"] != "Material has added." {
		t.Fatalf("envelope: %v", body)
	}
	if s.gotCreate.MaterialRate != 45 || s.gotCreate.Tax != 18 || s.gotCreate.Hsn != "3919" {
		t.Errorf("create input: %+v", s.gotCreate)
	}
	data := body["data"].(map[string]any)
	if data["_id"] != "m1" || data["material_name"] != "Vinyl" || data["tax"] != float64(18) {
		t.Errorf("data: %v", data)
	}
	if _, ok := data["borrowed"]; ok {
		t.Errorf("write response must not carry borrowed")
	}
	if ph, ok := data["priceHistory"].([]any); !ok || len(ph) != 0 {
		t.Errorf("priceHistory should be []: %v", data["priceHistory"])
	}
}

func TestAdd_Validation(t *testing.T) {
	// material_rate missing -> 422 with the period, status:false.
	body := postForm(t, &writeStub{}, func(h *Handler) http.HandlerFunc { return h.Add },
		map[string]string{"material_name": "Vinyl", "purchase_rate": "30"})
	if body["code"] != float64(422) || body["status"] != false || body["message"] != "Invalid request." {
		t.Errorf("validation: %v", body)
	}
}

func TestUpdate_SuccessAndNotFound(t *testing.T) {
	s := &writeStub{updateFound: true, updated: store.Material{ID: "m1", MaterialName: "Vinyl HD", MaterialRate: 50,
		PriceHistory: []store.PriceHistoryEntry{{MaterialRate: 45, PurchaseRate: 30}}}}
	f := map[string]string{"material_id": "m1", "material_name": "Vinyl HD", "material_rate": "50", "purchase_rate": "30"}
	body := postForm(t, s, func(h *Handler) http.HandlerFunc { return h.Update }, f)
	if body["code"] != float64(200) || body["message"] != "Operation successful." {
		t.Errorf("update ok: %v", body)
	}
	if ph := body["data"].(map[string]any)["priceHistory"].([]any); len(ph) != 1 {
		t.Errorf("price history should carry the old rates: %v", ph)
	}

	body = postForm(t, &writeStub{updateFound: false}, func(h *Handler) http.HandlerFunc { return h.Update }, f)
	if body["code"] != float64(404) || body["status"] != false || body["message"] != "Material not found." {
		t.Errorf("update 404: %v", body)
	}

	body = postForm(t, &writeStub{}, func(h *Handler) http.HandlerFunc { return h.Update }, map[string]string{"material_name": "x"})
	if body["code"] != float64(422) {
		t.Errorf("missing id -> 422: %v", body)
	}
}

func TestRemove(t *testing.T) {
	s := &writeStub{}
	body := postForm(t, s, func(h *Handler) http.HandlerFunc { return h.Remove }, map[string]string{"material_id": "m1"})
	if body["code"] != float64(200) || body["status"] != true || body["message"] != "Material has removed." {
		t.Errorf("remove: %v", body)
	}
	if s.gotDeleteID != "m1" {
		t.Errorf("delete id = %q", s.gotDeleteID)
	}
	// A miss is still 200 (Node does not 404 here).
	body = postForm(t, &writeStub{}, func(h *Handler) http.HandlerFunc { return h.Remove }, map[string]string{"material_id": "gone"})
	if body["code"] != float64(200) {
		t.Errorf("remove miss should still be 200: %v", body)
	}
	// No id -> 422.
	body = postForm(t, &writeStub{}, func(h *Handler) http.HandlerFunc { return h.Remove }, map[string]string{})
	if body["code"] != float64(422) {
		t.Errorf("remove no id: %v", body)
	}
}

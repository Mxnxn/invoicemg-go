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
	one      store.Material
	oneFound bool
	list     []store.Material
	got      store.ID
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
func (s *stubMaterials) Get(context.Context, store.ID, store.ID) (store.Material, bool, error) {
	return s.one, s.oneFound, nil
}
func (s *stubMaterials) SetUnit(context.Context, store.ID, store.ID, string) (string, string, bool, bool, error) {
	return "", "", false, false, nil
}
func (s *stubMaterials) SharedList(context.Context, []store.ID) ([]store.SharedMaterial, error) {
	return nil, nil
}
func (s *stubMaterials) OwnedSharing(context.Context, store.ID, store.ID) ([]store.ID, bool, error) {
	return nil, false, nil
}
func (s *stubMaterials) SetSharing(context.Context, store.ID, store.ID, []store.ID) ([]store.ID, bool, error) {
	return nil, false, nil
}
func (s *stubMaterials) DuplicatesSource(context.Context, []store.ID) ([]store.MaterialDuplicate, error) {
	return nil, nil
}

// writeStub drives the write-path tests.
type writeStub struct {
	created     store.Material
	updated     store.Material
	updateFound bool
	gotCreate   store.MaterialWrite
	gotUpdate   store.MaterialWrite
	gotDeleteID store.ID
	deleted     bool

	// set-unit
	setName     string
	setUnit     string
	setFound    bool
	setBorrowed bool
	gotSetID    store.ID
	gotSetUnit  string
}

func (s *writeStub) Visible(context.Context, store.ID) ([]store.Material, error) { return nil, nil }
func (s *writeStub) Get(context.Context, store.ID, store.ID) (store.Material, bool, error) {
	return store.Material{}, false, nil
}
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
func (s *writeStub) SetUnit(_ context.Context, _, id store.ID, unit string) (string, string, bool, bool, error) {
	s.gotSetID, s.gotSetUnit = id, unit
	return s.setName, s.setUnit, s.setFound, s.setBorrowed, nil
}
func (s *writeStub) SharedList(context.Context, []store.ID) ([]store.SharedMaterial, error) {
	return nil, nil
}
func (s *writeStub) OwnedSharing(context.Context, store.ID, store.ID) ([]store.ID, bool, error) {
	return nil, false, nil
}
func (s *writeStub) SetSharing(context.Context, store.ID, store.ID, []store.ID) ([]store.ID, bool, error) {
	return nil, false, nil
}
func (s *writeStub) DuplicatesSource(context.Context, []store.ID) ([]store.MaterialDuplicate, error) {
	return nil, nil
}

func postSetUnit(t *testing.T, s *writeStub, id, unit string) map[string]any {
	t.Helper()
	vals := neturl.Values{}
	if id != "" {
		vals.Set("material_id", id)
	}
	if unit != "" {
		vals.Set("unit", unit)
	}
	req := httptest.NewRequest("POST", "/material/set-unit", strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(auth.WithSession(req.Context(), store.Session{CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	New(s).SetUnit(rec, req)
	var b map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("not json: %v (%s)", err, rec.Body.String())
	}
	return b
}

const validID = "0123456789abcdef01234567" // 24 hex

func TestSetUnit_Success(t *testing.T) {
	s := &writeStub{setFound: true, setName: "Vinyl", setUnit: "SQ. Ft"}
	b := postSetUnit(t, s, validID, "SQ. Ft")
	if b["code"] != float64(200) || b["message"] != "Unit set." || b["status"] != true {
		t.Fatalf("envelope: %v", b)
	}
	if s.gotSetID != store.ID(validID) || s.gotSetUnit != "SQ. Ft" {
		t.Errorf("store args: %v %q", s.gotSetID, s.gotSetUnit)
	}
	data := b["data"].(map[string]any)
	if data["_id"] != validID || data["material_name"] != "Vinyl" || data["unit"] != "SQ. Ft" {
		t.Errorf("data shape: %v", data)
	}
	if _, hasV := data["__v"]; hasV {
		t.Error("projected doc must not carry __v")
	}
}

func TestSetUnit_MissingFields(t *testing.T) {
	if b := postSetUnit(t, &writeStub{}, "", "SQ. Ft"); b["code"] != float64(422) {
		t.Errorf("missing id should be 422: %v", b)
	}
	if b := postSetUnit(t, &writeStub{}, validID, ""); b["code"] != float64(422) {
		t.Errorf("missing unit should be 422: %v", b)
	}
}

// A malformed id is a guarded 404, never a 500 (Node's Alert-style id guard).
func TestSetUnit_MalformedIDIs404(t *testing.T) {
	s := &writeStub{}
	b := postSetUnit(t, s, "not-an-objectid", "SQ. Ft")
	if b["code"] != float64(404) || b["message"] != "Product not found." {
		t.Errorf("malformed id should be guarded 404: %v", b)
	}
	if s.gotSetID != "" {
		t.Error("store must not be touched for a malformed id")
	}
}

func TestSetUnit_NotFoundVsBorrowed(t *testing.T) {
	miss := postSetUnit(t, &writeStub{setFound: false, setBorrowed: false}, validID, "SQ. Ft")
	if miss["code"] != float64(404) || miss["message"] != "Product not found." {
		t.Errorf("plain miss: %v", miss)
	}
	bor := postSetUnit(t, &writeStub{setFound: false, setBorrowed: true}, validID, "SQ. Ft")
	if bor["code"] != float64(404) || !strings.Contains(bor["message"].(string), "belongs to another company") {
		t.Errorf("borrowed message: %v", bor)
	}
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

func TestGet(t *testing.T) {
	s := &stubMaterials{oneFound: true, one: store.Material{ID: "m1", CompanyID: "co1", MaterialName: "Vinyl", MaterialRate: 45}}
	r := httptest.NewRequest("POST", "/", strings.NewReader("material_id=m1"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	New(s).Get(rec, r)
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["code"] != float64(200) || body["data"].(map[string]any)["material_name"] != "Vinyl" {
		t.Fatalf("get: %v", body)
	}
	// missing id -> 422
	r2 := httptest.NewRequest("POST", "/", nil)
	r2 = r2.WithContext(auth.WithSession(r2.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec2 := httptest.NewRecorder()
	New(s).Get(rec2, r2)
	json.Unmarshal(rec2.Body.Bytes(), &body)
	if body["code"] != float64(422) {
		t.Errorf("missing id should 422: %v", body)
	}
	// not found -> 200 data null
	r3 := httptest.NewRequest("POST", "/", strings.NewReader("material_id=x"))
	r3.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r3 = r3.WithContext(auth.WithSession(r3.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec3 := httptest.NewRecorder()
	New(&stubMaterials{oneFound: false}).Get(rec3, r3)
	json.Unmarshal(rec3.Body.Bytes(), &body)
	if body["data"] != nil {
		t.Errorf("not found should be data null: %v", body)
	}
}

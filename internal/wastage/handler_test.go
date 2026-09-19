package wastage

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	neturl "net/url"
	"strings"
	"testing"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type stub struct {
	summary     []store.MaterialAvg
	list        []store.Wastage
	got         store.ID
	created     store.Wastage
	gotCreate   store.WastageWrite
	deleteFound bool
	deleteErr   error
	gotDeleteID store.ID
}

func (s *stub) List(_ context.Context, c store.ID) ([]store.Wastage, error) {
	s.got = c
	return s.list, nil
}

func (s *stub) MaterialsSummary(_ context.Context, _ store.ID) ([]store.MaterialAvg, error) {
	return s.summary, nil
}
func (s *stub) Create(_ context.Context, companyID, uid store.ID, in store.WastageWrite) (store.Wastage, error) {
	s.gotCreate = in
	return s.created, nil
}
func (s *stub) Delete(_ context.Context, companyID, wastageID store.ID) (bool, error) {
	s.got = companyID
	s.gotDeleteID = wastageID
	return s.deleteFound, s.deleteErr
}

func postRemove(t *testing.T, s *stub, id string) map[string]any {
	t.Helper()
	vals := neturl.Values{}
	if id != "\x00" {
		vals.Set("wastage_id", id)
	}
	req := httptest.NewRequest("POST", "/wastage/remove", strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(auth.WithSession(req.Context(), store.Session{CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	New(s).Remove(rec, req)
	var b map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("not json: %v (%s)", err, rec.Body.String())
	}
	return b
}

func TestRemove_Success(t *testing.T) {
	s := &stub{deleteFound: true}
	b := postRemove(t, s, "w1")
	if b["code"] != float64(200) || b["message"] != "Wastage record deleted." || b["status"] != true {
		t.Fatalf("envelope: %v", b)
	}
	if s.got != "co1" || s.gotDeleteID != "w1" {
		t.Errorf("scope/id wrong: %v %v", s.got, s.gotDeleteID)
	}
}

func TestRemove_MissingID(t *testing.T) {
	if b := postRemove(t, &stub{}, "\x00"); b["code"] != float64(422) || b["message"] != "Invalid request." {
		t.Errorf("missing id should be 422: %v", b)
	}
}

func TestRemove_NotFound(t *testing.T) {
	if b := postRemove(t, &stub{deleteFound: false}, "w1"); b["code"] != float64(404) || b["message"] != "Wastage record not found." {
		t.Errorf("miss should be 404: %v", b)
	}
}

func TestGetAll(t *testing.T) {
	s := &stub{list: []store.Wastage{{ID: "w1", CompanyID: "co1", MaterialName: "Vinyl", Total: 2, Rate: 45}}}
	r := httptest.NewRequest("POST", "/", nil)
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	New(s).GetAll(rec, r)
	var b map[string]any
	json.Unmarshal(rec.Body.Bytes(), &b)
	if s.got != "co1" || b["code"] != float64(200) {
		t.Fatalf("scope/code wrong: %v %v", s.got, b["code"])
	}
	row := b["data"].([]any)[0].(map[string]any)
	if row["material_name"] != "Vinyl" || row["rate"] != float64(45) {
		t.Errorf("row wrong: %v", row)
	}
}

func addForm(t *testing.T, s *stub, fields map[string]string) map[string]any {
	t.Helper()
	vals := neturl.Values{}
	for k, v := range fields {
		vals.Set(k, v)
	}
	r := httptest.NewRequest("POST", "/", strings.NewReader(vals.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	New(s).Add(rec, r)
	var b map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("not json: %v (%s)", err, rec.Body.String())
	}
	return b
}

func validWastage() map[string]string {
	return map[string]string{"material_name": "Vinyl", "rate": "45", "length": "10", "height": "4", "total": "40", "date": "2026-09-15"}
}

func TestAdd_Success(t *testing.T) {
	s := &stub{created: store.Wastage{ID: "w1", CompanyID: "co1", MaterialName: "Vinyl", Rate: 45, Length: 10, Height: 4, Total: 40, Date: "2026-09-15"}}
	f := validWastage()
	f["purchase_rate"] = "30"
	b := addForm(t, s, f)
	if b["code"] != float64(200) || b["message"] != "Wastage has added." {
		t.Fatalf("envelope: %v", b)
	}
	if s.gotCreate.Rate != 45 || s.gotCreate.PurchaseRate != 30 || s.gotCreate.Length != 10 {
		t.Errorf("create input: %+v", s.gotCreate)
	}
	data := b["data"].(map[string]any)
	if data["_id"] != "w1" || data["material_name"] != "Vinyl" || data["total"] != float64(40) {
		t.Errorf("data: %v", data)
	}
	// purchase_rate/cost_total default to 0 when omitted
	s2 := &stub{}
	addForm(t, s2, validWastage())
	if s2.gotCreate.PurchaseRate != 0 || s2.gotCreate.CostTotal != 0 {
		t.Errorf("optional fields should default to 0: %+v", s2.gotCreate)
	}
}

func TestAdd_Validation(t *testing.T) {
	for _, missing := range []string{"material_name", "rate", "length", "height", "total", "date"} {
		f := validWastage()
		delete(f, missing)
		b := addForm(t, &stub{}, f)
		if b["code"] != float64(422) || b["status"] != false || b["message"] != "Invalid request." {
			t.Errorf("missing %s should 422: %v", missing, b)
		}
	}
}

func TestMaterials(t *testing.T) {
	s := &stub{summary: []store.MaterialAvg{{MaterialName: "Vinyl", MaterialRate: 45, PurchaseRate: 30}}}
	r := httptest.NewRequest("POST", "/", nil)
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	New(s).Materials(rec, r)
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	rows := body["data"].([]any)
	if body["code"] != float64(200) || len(rows) != 1 || rows[0].(map[string]any)["material_name"] != "Vinyl" {
		t.Fatalf("materials: %v", body)
	}
}

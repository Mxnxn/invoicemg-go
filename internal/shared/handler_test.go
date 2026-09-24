package shared

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

// Embed the store interfaces so each stub satisfies the full contract while overriding only the
// one method these reports call; any other call would nil-panic, which is the point.
type stubCompanies struct {
	store.Companies
	ids    []store.ID
	shared bool
	labels map[store.ID]string
	gotID  store.ID
	gotUID store.ID
}

func (s *stubCompanies) Scope(_ context.Context, companyID, uid store.ID) ([]store.ID, bool, map[store.ID]string, error) {
	s.gotID, s.gotUID = companyID, uid
	return s.ids, s.shared, s.labels, nil
}

type stubClients struct {
	store.Clients
	list   []store.SharedClient
	gotIDs []store.ID
}

func (s *stubClients) SharedList(_ context.Context, ids []store.ID) ([]store.SharedClient, error) {
	s.gotIDs = ids
	return s.list, nil
}

type stubMaterials struct {
	store.Materials
	list []store.SharedMaterial
}

func (s *stubMaterials) SharedList(_ context.Context, _ []store.ID) ([]store.SharedMaterial, error) {
	return s.list, nil
}

func req(sess store.Session) *http.Request {
	r := httptest.NewRequest("GET", "/", nil)
	return r.WithContext(auth.WithSession(r.Context(), sess))
}

func decodeData(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, rec.Body.String())
	}
	if body["code"] != float64(200) {
		t.Fatalf("code = %v, want 200 (%s)", body["code"], rec.Body.String())
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("data is not an object: %s", rec.Body.String())
	}
	return data
}

func TestCustomers_LabelsScopeAndDuplicates(t *testing.T) {
	co := &stubCompanies{
		ids:    []store.ID{"c1", "c2"},
		shared: true,
		labels: map[store.ID]string{"c1": "Alpha", "c2": "Beta"},
	}
	cl := &stubClients{list: []store.SharedClient{
		{ID: "a", ClientName: "N1", ClientFirm: "F1", ClientPhone: "999", ClientGST: "G1", CompanyID: "c1"},
		{ID: "b", ClientName: "N2", ClientFirm: "F2", ClientPhone: "999", ClientGST: "G2", CompanyID: "c2"},
		{ID: "d", ClientName: "N3", ClientFirm: "F3", ClientPhone: "888", ClientGST: "G3", CompanyID: "c1"},
	}}
	h := New(co, cl, &stubMaterials{})
	rec := httptest.NewRecorder()
	h.Customers(rec, req(store.Session{UID: "u1", CompanyID: "c1", Role: "admin"}))

	data := decodeData(t, rec)
	if co.gotID != "c1" || co.gotUID != "u1" {
		t.Errorf("Scope called with (%q,%q), want (c1,u1)", co.gotID, co.gotUID)
	}
	if len(co.ids) != len(cl.gotIDs) {
		t.Errorf("SharedList got %v ids, want the scoped set %v", cl.gotIDs, co.ids)
	}
	if data["shared"] != true {
		t.Errorf("shared = %v, want true", data["shared"])
	}
	if data["companyCount"] != float64(2) {
		t.Errorf("companyCount = %v, want 2", data["companyCount"])
	}
	// "999" appears in two companies, "888" in one -> one duplicated value.
	if data["duplicates"] != float64(1) {
		t.Errorf("duplicates = %v, want 1", data["duplicates"])
	}
	rows := data["rows"].([]any)
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	first := rows[0].(map[string]any)
	if first["companyName"] != "Alpha" {
		t.Errorf("row0 companyName = %v, want Alpha", first["companyName"])
	}
	if first["clientName"] != "N1" || first["company_id"] != "c1" {
		t.Errorf("row0 projection wrong: %v", first)
	}
	if _, hasV := first["__v"]; hasV {
		t.Error("shared customer row must not carry __v")
	}
}

func TestMaterials_DuplicatesOnName(t *testing.T) {
	co := &stubCompanies{ids: []store.ID{"c1"}, shared: false, labels: map[store.ID]string{"c1": "Alpha"}}
	m := &stubMaterials{list: []store.SharedMaterial{
		{ID: "a", MaterialName: "Art Card", MaterialRate: 10, PurchaseRate: 5, Hsn: "48", Unit: "kg", CompanyID: "c1"},
		{ID: "b", MaterialName: "art card", MaterialRate: 12, PurchaseRate: 6, Hsn: "48", Unit: "kg", CompanyID: "c1"},
		{ID: "d", MaterialName: "Glue", MaterialRate: 3, PurchaseRate: 1, Hsn: "35", Unit: "ltr", CompanyID: "c1"},
	}}
	h := New(co, &stubClients{}, m)
	rec := httptest.NewRecorder()
	h.Materials(rec, req(store.Session{UID: "u1", CompanyID: "c1", Role: "admin"}))

	data := decodeData(t, rec)
	if data["shared"] != false {
		t.Errorf("shared = %v, want false", data["shared"])
	}
	if data["companyCount"] != float64(1) {
		t.Errorf("companyCount = %v, want 1", data["companyCount"])
	}
	// "Art Card" and "art card" collapse case-insensitively -> one duplicated name; "Glue" is alone.
	if data["duplicates"] != float64(1) {
		t.Errorf("duplicates = %v, want 1", data["duplicates"])
	}
	rows := data["rows"].([]any)
	first := rows[0].(map[string]any)
	if first["material_name"] != "Art Card" || first["material_rate"] != float64(10) || first["companyName"] != "Alpha" {
		t.Errorf("row0 projection wrong: %v", first)
	}
}

func TestCustomers_EmptyIsArrayNotNull(t *testing.T) {
	co := &stubCompanies{ids: []store.ID{"c1"}, labels: map[store.ID]string{}}
	h := New(co, &stubClients{list: nil}, &stubMaterials{})
	rec := httptest.NewRecorder()
	h.Customers(rec, req(store.Session{UID: "u1", CompanyID: "c1"}))
	// An empty report must serialise rows as [] so the client can .map over it.
	if got := rec.Body.String(); !containsRowsArray(got) {
		t.Errorf(`expected "rows":[] in %s`, got)
	}
}

func containsRowsArray(s string) bool {
	for i := 0; i+9 <= len(s); i++ {
		if s[i:i+9] == `"rows":[]` {
			return true
		}
	}
	return false
}

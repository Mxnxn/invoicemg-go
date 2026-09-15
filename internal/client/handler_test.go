package client

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

type stubClients struct {
	list       []store.Client
	err        error
	gotUID     store.ID
	gotCompany store.ID
}

func (s *stubClients) Visible(_ context.Context, uid, companyID store.ID) ([]store.Client, error) {
	s.gotUID, s.gotCompany = uid, companyID
	return s.list, s.err
}

// Default write methods so stubClients satisfies store.Clients for the read tests; writeStub
// embeds stubClients and overrides these to drive the write-path tests.
func (s *stubClients) Create(context.Context, store.ID, store.ID, int64, store.ClientWrite) (store.Client, store.Dup, error) {
	return store.Client{}, store.DupNone, nil
}
func (s *stubClients) Update(context.Context, store.ID, store.ID, store.ClientWrite) (store.Client, store.Dup, bool, error) {
	return store.Client{}, store.DupNone, false, nil
}
func (s *stubClients) Delete(context.Context, store.ID, store.ID) (bool, error) { return false, nil }
func (s *stubClients) EnsureSupplier(context.Context, store.ID, store.ClientWrite) (bool, error) {
	return false, nil
}

// Write-path stub state; each field lets a test steer one outcome.
type writeStub struct {
	stubClients
	createDup    store.Dup
	created      store.Client
	updateDup    store.Dup
	updated      store.Client
	updateFound  bool
	deleteFound  bool
	supplierMade bool
	gotLegacyID  int64
	gotCreate    store.ClientWrite
	ensureCalled bool
	gotDeleteID  store.ID
}

func (s *writeStub) Create(_ context.Context, _, _ store.ID, legacyID int64, in store.ClientWrite) (store.Client, store.Dup, error) {
	s.gotLegacyID, s.gotCreate = legacyID, in
	if s.createDup != store.DupNone {
		return store.Client{}, s.createDup, nil
	}
	return s.created, store.DupNone, nil
}

func (s *writeStub) Update(_ context.Context, _, id store.ID, in store.ClientWrite) (store.Client, store.Dup, bool, error) {
	if s.updateDup != store.DupNone {
		return store.Client{}, s.updateDup, false, nil
	}
	return s.updated, store.DupNone, s.updateFound, nil
}

func (s *writeStub) Delete(_ context.Context, _, id store.ID) (bool, error) {
	s.gotDeleteID = id
	return s.deleteFound, nil
}

func (s *writeStub) EnsureSupplier(_ context.Context, _ store.ID, _ store.ClientWrite) (bool, error) {
	s.ensureCalled = true
	return s.supplierMade, nil
}

func share(ids ...store.ID) *[]store.ID {
	out := append([]store.ID{}, ids...)
	return &out
}

func serve(t *testing.T, s store.Clients, fn func(*Handler) func(http.ResponseWriter, *http.Request)) map[string]any {
	t.Helper()
	req := httptest.NewRequest("POST", "/", nil)
	req = req.WithContext(auth.WithSession(req.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	fn(New(s))(rec, req)
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("not json: %v (%s)", err, rec.Body.String())
	}
	return body
}

// The three cases #1 turns on: a client the acting company owns, one shared IN from another of
// the owner's companies (borrowed), and a legacy row with a null company and no sharing field.
func sampleClients() []store.Client {
	return []store.Client{
		{ID: "own1", UID: "u1", CompanyID: "co1", ClientName: "Priya", ClientFirm: "Acme", OpeningBalance: 100, Sharing: share("co1")},
		{ID: "bor1", UID: "u1", CompanyID: "co2", ClientName: "Ravi", ClientFirm: "Bolt", OpeningBalance: 0, Sharing: share("co2", "co1")},
		{ID: "leg1", UID: "u1", CompanyID: "", ClientName: "Sam", ClientFirm: "Cobalt", OpeningBalance: 250, Sharing: nil},
	}
}

func TestGetall_SharingAndBorrowed(t *testing.T) {
	s := &stubClients{list: sampleClients()}
	body := serve(t, s, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Getall })

	if s.gotUID != "u1" || s.gotCompany != "co1" {
		t.Errorf("scope from session wrong: uid=%q company=%q", s.gotUID, s.gotCompany)
	}
	if body["message"] != "Operation successful" { // NO trailing period
		t.Errorf("message = %q", body["message"])
	}
	if _, hasStatus := body["status"]; hasStatus {
		t.Error("getall must not send a status field")
	}
	rows := body["data"].([]any)
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(rows))
	}

	own := rows[0].(map[string]any)
	if own["borrowed"] != false {
		t.Error("own client must not be borrowed")
	}
	if own["company_id"] != "co1" {
		t.Errorf("own company_id = %v", own["company_id"])
	}
	if sh, ok := own["sharing"].(map[string]any); !ok || len(sh["companies"].([]any)) != 1 {
		t.Errorf("own sharing wrong: %v", own["sharing"])
	}

	bor := rows[1].(map[string]any)
	if bor["borrowed"] != true {
		t.Error("client owned by co2 but shared with co1 must be borrowed")
	}

	leg := rows[2].(map[string]any)
	if leg["borrowed"] != false {
		t.Error("legacy null-company client must not be borrowed")
	}
	if v, ok := leg["company_id"]; !ok || v != nil {
		t.Errorf("legacy company_id must be present and null, got ok=%v v=%v", ok, v)
	}
	if _, hasSharing := leg["sharing"]; hasSharing {
		t.Error("legacy client with no sharing field must OMIT the sharing key")
	}
}

func TestOnly_Shape(t *testing.T) {
	body := serve(t, &stubClients{list: sampleClients()}, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Only })
	if body["message"] != "Operation successful" {
		t.Errorf("message = %q", body["message"])
	}
	rows := body["data"].([]any)
	own := rows[0].(map[string]any)
	if own["openingBalance"] != float64(100) {
		t.Errorf("openingBalance = %v, want 100", own["openingBalance"])
	}
	if _, hasSharing := own["sharing"]; hasSharing {
		t.Error("only must not include sharing")
	}
	if _, hasBorrowed := own["borrowed"]; hasBorrowed {
		t.Error("only must not include borrowed")
	}
}

func TestGetall_EmptyIsArray(t *testing.T) {
	body := serve(t, &stubClients{list: nil}, func(h *Handler) func(http.ResponseWriter, *http.Request) { return h.Getall })
	if rows, ok := body["data"].([]any); !ok || len(rows) != 0 {
		t.Errorf("data should be [], got %v", body["data"])
	}
}

func postForm(t *testing.T, s store.Clients, fn func(*Handler) http.HandlerFunc, fields map[string]string) map[string]any {
	t.Helper()
	vals := neturl.Values{}
	for k, v := range fields {
		vals.Set(k, v)
	}
	req := httptest.NewRequest("POST", "/", strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(auth.WithSession(req.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	fn(New(s))(rec, req)
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("not json: %v (%s)", err, rec.Body.String())
	}
	return body
}

func validAddFields() map[string]string {
	return map[string]string{
		"uid": "u1", "client_name": "Priya", "client_firm": "Acme",
		"client_phone": "9876543210", "client_gst": "24AAAAA0000A1Z5", "client_address": "Baroda",
	}
}

func TestAdd_Success(t *testing.T) {
	lid := int64(1700000000000)
	s := &writeStub{created: store.Client{ID: "new1", LegacyID: &lid, ClientName: "Priya", ClientFirm: "Acme",
		ClientPhone: "9876543210", ClientGST: "24AAAAA0000A1Z5", ClientAddress: "Baroda"}}
	body := postForm(t, s, func(h *Handler) http.HandlerFunc { return h.Add }, validAddFields())
	if body["code"] != float64(200) || body["message"] != "Client has added." {
		t.Fatalf("envelope: %v", body)
	}
	if _, ok := body["status"]; ok {
		t.Errorf("add success sends no status field")
	}
	data := body["data"].(map[string]any)
	if data["_id"] != "new1" || data["clientName"] != "Priya" || data["client_id"] != float64(1700000000000) {
		t.Errorf("data: %v", data)
	}
	if data["supplierCreated"] != false {
		t.Errorf("supplierCreated should be false without the flag: %v", data)
	}
	if s.ensureCalled {
		t.Errorf("EnsureSupplier must not be called without also_supplier=true")
	}
	if s.gotLegacyID == 0 {
		t.Errorf("a legacy client_id millis should be stamped")
	}
}

func TestAdd_AlsoSupplier(t *testing.T) {
	s := &writeStub{created: store.Client{ID: "new1"}, supplierMade: true}
	f := validAddFields()
	f["also_supplier"] = "true"
	body := postForm(t, s, func(h *Handler) http.HandlerFunc { return h.Add }, f)
	if body["message"] != "Client added, and added as a supplier." {
		t.Errorf("message: %v", body["message"])
	}
	if body["data"].(map[string]any)["supplierCreated"] != true {
		t.Errorf("supplierCreated should be true: %v", body["data"])
	}
	if !s.ensureCalled {
		t.Errorf("EnsureSupplier should be called")
	}
}

func TestAdd_ValidationMissing(t *testing.T) {
	f := validAddFields()
	delete(f, "client_gst")
	body := postForm(t, &writeStub{}, func(h *Handler) http.HandlerFunc { return h.Add }, f)
	if body["code"] != float64(422) || body["message"] != "Invalid request" || body["status"] != true {
		t.Errorf("missing field 422: %v", body)
	}
}

func TestAdd_BadPhoneAndGstLengths(t *testing.T) {
	f := validAddFields()
	f["client_phone"] = "12345"
	body := postForm(t, &writeStub{}, func(h *Handler) http.HandlerFunc { return h.Add }, f)
	if body["code"] != float64(422) || body["error"].(map[string]any)["client_phone"] == nil {
		t.Errorf("bad phone: %v", body)
	}
	f = validAddFields()
	f["client_gst"] = "SHORT"
	body = postForm(t, &writeStub{}, func(h *Handler) http.HandlerFunc { return h.Add }, f)
	if body["code"] != float64(422) || body["error"].(map[string]any)["client_gst"] == nil {
		t.Errorf("bad gst: %v", body)
	}
}

func TestAdd_DuplicateGST(t *testing.T) {
	body := postForm(t, &writeStub{createDup: store.DupGST}, func(h *Handler) http.HandlerFunc { return h.Add }, validAddFields())
	if body["code"] != float64(422) || body["status"] != true {
		t.Fatalf("dup envelope: %v", body)
	}
	if body["error"].(map[string]any)["client_gst"] == nil {
		t.Errorf("dup gst error object: %v", body)
	}
}

func TestUpdate_SuccessAndNotFound(t *testing.T) {
	s := &writeStub{updateFound: true, updated: store.Client{ID: "c1", ClientName: "New"}}
	f := map[string]string{"client_id": "c1", "client_name": "New", "client_firm": "F",
		"client_phone": "9876543210", "client_gst": "24AAAAA0000A1Z5", "client_address": "A"}
	body := postForm(t, s, func(h *Handler) http.HandlerFunc { return h.Update }, f)
	if body["code"] != float64(200) || body["message"] != "Operation successful." {
		t.Errorf("update ok: %v", body)
	}
	if body["data"].(map[string]any)["clientName"] != "New" {
		t.Errorf("data: %v", body["data"])
	}

	body = postForm(t, &writeStub{updateFound: false}, func(h *Handler) http.HandlerFunc { return h.Update }, f)
	if body["code"] != float64(404) || body["status"] != false || body["message"] != "Client not found." {
		t.Errorf("update 404: %v", body)
	}
}

func TestUpdate_ValidationAndDup(t *testing.T) {
	body := postForm(t, &writeStub{}, func(h *Handler) http.HandlerFunc { return h.Update }, map[string]string{"client_name": "x"})
	if body["code"] != float64(422) || body["message"] != "Invalid request" {
		t.Errorf("missing client_id: %v", body)
	}
	f := map[string]string{"client_id": "c1", "client_name": "N", "client_firm": "F",
		"client_phone": "9876543210", "client_gst": "24AAAAA0000A1Z5", "client_address": "A"}
	body = postForm(t, &writeStub{updateDup: store.DupPhone}, func(h *Handler) http.HandlerFunc { return h.Update }, f)
	if body["code"] != float64(422) || body["error"].(map[string]any)["client_phone"] == nil {
		t.Errorf("dup phone: %v", body)
	}
}

func TestRemove(t *testing.T) {
	body := postForm(t, &writeStub{deleteFound: true}, func(h *Handler) http.HandlerFunc { return h.Remove }, map[string]string{"client_id": "c1"})
	if body["code"] != float64(200) || body["status"] != true || body["message"] != "Client has removed." {
		t.Errorf("remove ok: %v", body)
	}
	body = postForm(t, &writeStub{deleteFound: false}, func(h *Handler) http.HandlerFunc { return h.Remove }, map[string]string{"client_id": "c1"})
	if body["code"] != float64(404) || body["status"] != false {
		t.Errorf("remove 404: %v", body)
	}
	body = postForm(t, &writeStub{}, func(h *Handler) http.HandlerFunc { return h.Remove }, map[string]string{})
	if body["code"] != float64(422) {
		t.Errorf("remove no id 422: %v", body)
	}
}

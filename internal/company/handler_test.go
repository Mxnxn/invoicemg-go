package company

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

type stubCompanies struct {
	list   []store.Company
	active store.Company
	actErr error

	created       store.Company
	updated       store.Company
	updateFound   bool
	findActive    store.Company
	found         bool
	deactivate    store.DeactivateResult
	gotCreate     store.CompanyWrite
	gotPatch      store.CompanyPatch
	gotDeactivate store.ID

	queueStored []string
	queueFound  bool
	gotQueue    []string

	numbering       map[string]json.RawMessage
	setNumberFound  bool
	gotNumberKind   string
	gotNumberFormat json.RawMessage
	setNumberCalled bool

	setReportsFound  bool
	setReportsOn     bool
	setReportsCalled bool
}

func (s *stubCompanies) List(_ context.Context, _ store.ID) ([]store.Company, error) {
	return s.list, nil
}
func (s *stubCompanies) Active(_ context.Context, _, _ store.ID) (store.Company, error) {
	return s.active, s.actErr
}
func (s *stubCompanies) Count(_ context.Context, _ store.ID) (int, error) { return len(s.list), nil }
func (s *stubCompanies) Scope(context.Context, store.ID, store.ID) ([]store.ID, bool, map[store.ID]string, error) {
	return nil, false, nil, nil
}
func (s *stubCompanies) SetReportsAcrossCompanies(_ context.Context, _, _ store.ID, on bool) (bool, error) {
	s.setReportsCalled = true
	s.setReportsOn = on
	return s.setReportsFound, nil
}
func (s *stubCompanies) Create(_ context.Context, _ store.ID, in store.CompanyWrite) (store.Company, error) {
	s.gotCreate = in
	return s.created, nil
}
func (s *stubCompanies) Update(_ context.Context, _, _ store.ID, patch store.CompanyPatch) (store.Company, bool, error) {
	s.gotPatch = patch
	return s.updated, s.updateFound, nil
}
func (s *stubCompanies) FindActive(_ context.Context, _, _ store.ID) (store.Company, bool, error) {
	return s.findActive, s.found, nil
}
func (s *stubCompanies) Deactivate(_ context.Context, _, companyID store.ID) (store.DeactivateResult, error) {
	s.gotDeactivate = companyID
	return s.deactivate, nil
}
func (s *stubCompanies) SetQueueOrder(_ context.Context, _ store.ID, order []string) ([]string, bool, error) {
	s.gotQueue = order
	return s.queueStored, s.queueFound, nil
}
func (s *stubCompanies) Numbering(context.Context, store.ID, store.ID) (map[string]json.RawMessage, error) {
	if s.numbering == nil {
		return map[string]json.RawMessage{}, nil
	}
	return s.numbering, nil
}
func (s *stubCompanies) SetNumbering(_ context.Context, _, _ store.ID, kind string, format json.RawMessage) (bool, error) {
	s.setNumberCalled, s.gotNumberKind, s.gotNumberFormat = true, kind, format
	return s.setNumberFound, nil
}

// stubSessions implements store.Sessions; only BindCompany matters here.
type stubSessions struct {
	boundCompany store.ID
	boundTab     string
}

func (s *stubSessions) FindByToken(context.Context, string) (store.Session, error) {
	return store.Session{}, store.ErrNotFound
}
func (s *stubSessions) Deactivate(context.Context, store.ID) error { return nil }
func (s *stubSessions) ResolveCompany(context.Context, string, string, store.ID) (store.ID, error) {
	return "", nil
}
func (s *stubSessions) BindCompany(_ context.Context, _, tabID string, _, companyID store.ID) error {
	s.boundTab, s.boundCompany = tabID, companyID
	return nil
}

// stubUsers implements store.Users; only FindByID matters here.
type stubUsers struct{ limit int }

func (s *stubUsers) UpdateProfile(_ context.Context, _ store.ID, _, _ string) (store.User, bool, error) {
	return store.User{}, false, nil
}
func (s *stubUsers) FindByEmail(_ context.Context, _ string) (store.User, error) {
	return store.User{}, store.ErrNotFound
}
func (s *stubUsers) FindByID(_ context.Context, _ store.ID) (store.User, error) {
	return store.User{CompanyLimit: s.limit}, nil
}
func (s *stubUsers) CreateSession(_ context.Context, _ store.NewSession) (store.Session, error) {
	return store.Session{}, nil
}

func req(sess store.Session) *http.Request {
	r := httptest.NewRequest("POST", "/", nil)
	return r.WithContext(auth.WithSession(r.Context(), sess))
}

func TestList_LimitAndCanAdd(t *testing.T) {
	h := New(&stubCompanies{list: []store.Company{{ID: "co1", Name: "A", IsActive: true}}}, &stubUsers{limit: 3}, &stubSessions{})
	rec := httptest.NewRecorder()
	h.List(rec, req(store.Session{UID: "u1", CompanyID: "co1", Role: "admin"}))

	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	data := body["data"].(map[string]any)
	if data["company_limit"] != float64(3) {
		t.Errorf("company_limit = %v, want 3", data["company_limit"])
	}
	if data["active_company_id"] != "co1" {
		t.Errorf("active_company_id = %v", data["active_company_id"])
	}
	if data["can_add_company"] != true { // 1 company < limit 3
		t.Errorf("can_add_company = %v, want true", data["can_add_company"])
	}
	if _, ok := body["status"]; ok {
		t.Error("company/list sends no status field")
	}
}

// limit is clamped to at least 1 (Math.max(1, ...)), so a user row with 0 still allows nothing
// beyond one and can_add is false once one exists.
func TestList_LimitClampedToOne(t *testing.T) {
	h := New(&stubCompanies{list: []store.Company{{ID: "co1"}}}, &stubUsers{limit: 0}, &stubSessions{})
	rec := httptest.NewRecorder()
	h.List(rec, req(store.Session{UID: "u1", CompanyID: "co1", Role: "admin"}))
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	data := body["data"].(map[string]any)
	if data["company_limit"] != float64(1) {
		t.Errorf("limit = %v, want clamped to 1", data["company_limit"])
	}
	if data["can_add_company"] != false {
		t.Error("can_add_company should be false at the limit")
	}
}

func TestActive_NoCompany(t *testing.T) {
	h := New(&stubCompanies{}, &stubUsers{}, &stubSessions{})
	rec := httptest.NewRecorder()
	h.Active(rec, req(store.Session{UID: "u1", CompanyID: ""}))
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["code"] != float64(404) {
		t.Errorf("code = %v, want 404 when no active company", body["code"])
	}
}

func TestActive_Success(t *testing.T) {
	h := New(&stubCompanies{active: store.Company{ID: "co1", Name: "A", Firm: "A LLP", IsActive: true}}, &stubUsers{}, &stubSessions{})
	rec := httptest.NewRecorder()
	h.Active(rec, req(store.Session{UID: "u1", CompanyID: "co1"}))
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	data := body["data"].(map[string]any)
	if data["_id"] != "co1" || data["firm"] != "A LLP" {
		t.Errorf("company wrong: %v", data)
	}
	if et, ok := data["exportTemplate"].(map[string]any); !ok || et["logo"] != true {
		t.Errorf("exportTemplate defaults missing: %v", data["exportTemplate"])
	}
}

func postForm(t *testing.T, h *Handler, fn func(*Handler) http.HandlerFunc, sess store.Session, tabID string, fields map[string]string) map[string]any {
	t.Helper()
	vals := neturl.Values{}
	for k, v := range fields {
		vals.Set(k, v)
	}
	r := httptest.NewRequest("POST", "/", strings.NewReader(vals.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if tabID != "" {
		r.Header.Set("TAB-ID", tabID)
	}
	r = r.WithContext(auth.WithSession(r.Context(), sess))
	rec := httptest.NewRecorder()
	fn(h)(rec, r)
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("not json: %v (%s)", err, rec.Body.String())
	}
	return body
}

func TestCreate(t *testing.T) {
	s := &stubCompanies{created: store.Company{ID: "co2", Name: "New Co", Firm: "New Co", IsDefault: false, IsActive: true}}
	h := New(s, &stubUsers{}, &stubSessions{})
	body := postForm(t, h, func(h *Handler) http.HandlerFunc { return h.Create }, store.Session{UID: "u1", Role: "admin"}, "",
		map[string]string{"name": "New Co", "gst": "24AAA"})
	if body["code"] != float64(200) || body["message"] != "Company created." {
		t.Fatalf("envelope: %v", body)
	}
	if s.gotCreate.Name != "New Co" || s.gotCreate.Gst != "24AAA" {
		t.Errorf("create input: %+v", s.gotCreate)
	}
	if body["data"].(map[string]any)["_id"] != "co2" {
		t.Errorf("data: %v", body["data"])
	}

	// name required
	body = postForm(t, h, func(h *Handler) http.HandlerFunc { return h.Create }, store.Session{UID: "u1", Role: "admin"}, "", map[string]string{})
	if body["code"] != float64(422) || body["message"] != "Company name is required." {
		t.Errorf("missing name: %v", body)
	}
}

func TestUpdate(t *testing.T) {
	s := &stubCompanies{updateFound: true, updated: store.Company{ID: "co1", Name: "Renamed"}}
	h := New(s, &stubUsers{}, &stubSessions{})
	body := postForm(t, h, func(h *Handler) http.HandlerFunc { return h.Update }, store.Session{UID: "u1", Role: "admin"}, "",
		map[string]string{"company_id": "co1", "name": "Renamed", "gst": ""})
	if body["code"] != float64(200) || body["message"] != "Company updated." {
		t.Fatalf("envelope: %v", body)
	}
	// name submitted -> patched; gst present-but-empty -> patched to "" ; phone absent -> nil
	if s.gotPatch.Name == nil || *s.gotPatch.Name != "Renamed" {
		t.Errorf("name patch: %v", s.gotPatch.Name)
	}
	if s.gotPatch.Gst == nil || *s.gotPatch.Gst != "" {
		t.Errorf("gst present-empty should patch to empty: %v", s.gotPatch.Gst)
	}
	if s.gotPatch.Phone != nil {
		t.Errorf("phone absent should be nil: %v", s.gotPatch.Phone)
	}

	body = postForm(t, h, func(h *Handler) http.HandlerFunc { return h.Update }, store.Session{UID: "u1", Role: "admin"}, "", map[string]string{})
	if body["code"] != float64(422) {
		t.Errorf("missing company_id: %v", body)
	}
	body = postForm(t, New(&stubCompanies{updateFound: false}, &stubUsers{}, &stubSessions{}), func(h *Handler) http.HandlerFunc { return h.Update },
		store.Session{UID: "u1", Role: "admin"}, "", map[string]string{"company_id": "co9"})
	if body["code"] != float64(404) {
		t.Errorf("update 404: %v", body)
	}
}

func TestSwitch(t *testing.T) {
	sess := &stubSessions{}
	h := New(&stubCompanies{found: true, findActive: store.Company{ID: "co2", Name: "B"}}, &stubUsers{}, sess)
	body := postForm(t, h, func(h *Handler) http.HandlerFunc { return h.Switch },
		store.Session{UID: "u1", Token: "tok"}, "tab-7", map[string]string{"company_id": "co2"})
	if body["code"] != float64(200) || body["message"] != "Company switched." {
		t.Fatalf("envelope: %v", body)
	}
	if sess.boundTab != "tab-7" || sess.boundCompany != "co2" {
		t.Errorf("binding wrong: tab=%q company=%q", sess.boundTab, sess.boundCompany)
	}

	// missing TAB-ID
	body = postForm(t, h, func(h *Handler) http.HandlerFunc { return h.Switch }, store.Session{UID: "u1"}, "", map[string]string{"company_id": "co2"})
	if body["code"] != float64(422) || body["message"] != "TAB-ID header is required to switch company." {
		t.Errorf("missing tab: %v", body)
	}

	// company not owned/active
	body = postForm(t, New(&stubCompanies{found: false}, &stubUsers{}, &stubSessions{}), func(h *Handler) http.HandlerFunc { return h.Switch },
		store.Session{UID: "u1"}, "tab-1", map[string]string{"company_id": "cx"})
	if body["code"] != float64(404) {
		t.Errorf("switch 404: %v", body)
	}
}

func TestDeactivate(t *testing.T) {
	cases := []struct {
		res  store.DeactivateResult
		code float64
		msg  string
	}{
		{store.DeactivateOK, 200, "Company deactivated."},
		{store.DeactivateMustKeepOne, 422, "You must keep at least one active company."},
		{store.DeactivateIsDefault, 422, "Set another company as default before deactivating this one."},
		{store.DeactivateNotFound, 404, "Company not found."},
	}
	for _, c := range cases {
		h := New(&stubCompanies{deactivate: c.res}, &stubUsers{}, &stubSessions{})
		body := postForm(t, h, func(h *Handler) http.HandlerFunc { return h.Deactivate },
			store.Session{UID: "u1", Role: "admin"}, "", map[string]string{"company_id": "co1"})
		if body["code"] != c.code || body["message"] != c.msg {
			t.Errorf("result %d: got %v", c.res, body)
		}
	}
	h := New(&stubCompanies{}, &stubUsers{}, &stubSessions{})
	body := postForm(t, h, func(h *Handler) http.HandlerFunc { return h.Deactivate }, store.Session{UID: "u1", Role: "admin"}, "", map[string]string{})
	if body["code"] != float64(422) {
		t.Errorf("missing id: %v", body)
	}
}

func (s *stubSessions) DeactivateOthers(context.Context, store.ID, string) error { return nil }
func (s *stubUsers) UpdatePassword(context.Context, store.ID, string) (bool, error) {
	return false, nil
}

func TestQueueDefault(t *testing.T) {
	sess := store.Session{UID: "u1", CompanyID: "co1", Role: "admin"}

	// a good order is normalised (First/Last pinned) and stored; returns queueOrder
	s := &stubCompanies{queueFound: true, queueStored: []string{"Created", "Printing", "Done"}}
	body := postForm(t, New(s, &stubUsers{}, &stubSessions{}), func(h *Handler) http.HandlerFunc { return h.QueueDefault },
		sess, "", map[string]string{"queueOrder": `["Printing"]`})
	if body["code"] != float64(200) || body["message"] != "Saved as the default for new jobs." || body["status"] != true {
		t.Fatalf("envelope: %v", body)
	}
	// handler normalises before storing: ["Printing"] -> [Created, Printing, Done]
	if len(s.gotQueue) != 3 || s.gotQueue[0] != "Created" || s.gotQueue[1] != "Printing" || s.gotQueue[2] != "Done" {
		t.Errorf("stored order not normalised: %v", s.gotQueue)
	}
	if qo, _ := body["data"].(map[string]any)["queueOrder"].([]any); len(qo) != 3 {
		t.Errorf("data.queueOrder: %v", body["data"])
	}

	// missing field -> 422 "Invalid request."
	if b := postForm(t, New(&stubCompanies{}, &stubUsers{}, &stubSessions{}), func(h *Handler) http.HandlerFunc { return h.QueueDefault },
		sess, "", map[string]string{}); b["code"] != float64(422) || b["message"] != "Invalid request." {
		t.Errorf("missing: %v", b)
	}

	// malformed JSON -> 422 parse message
	if b := postForm(t, New(&stubCompanies{}, &stubUsers{}, &stubSessions{}), func(h *Handler) http.HandlerFunc { return h.QueueDefault },
		sess, "", map[string]string{"queueOrder": "{bad"}); b["code"] != float64(422) || b["message"] != "queueOrder must be a JSON array of strings." {
		t.Errorf("malformed: %v", b)
	}

	// valid JSON non-array -> normalises to the fallback (not an error)
	s2 := &stubCompanies{queueFound: true, queueStored: store.QueueStages}
	postForm(t, New(s2, &stubUsers{}, &stubSessions{}), func(h *Handler) http.HandlerFunc { return h.QueueDefault },
		sess, "", map[string]string{"queueOrder": `{}`})
	if len(s2.gotQueue) != 4 || s2.gotQueue[0] != "Created" || s2.gotQueue[3] != "Done" {
		t.Errorf("non-array should fall back to default: %v", s2.gotQueue)
	}

	// company not found -> 404
	if b := postForm(t, New(&stubCompanies{queueFound: false}, &stubUsers{}, &stubSessions{}), func(h *Handler) http.HandlerFunc { return h.QueueDefault },
		sess, "", map[string]string{"queueOrder": `["Printing"]`}); b["code"] != float64(404) || b["message"] != "Company not found." {
		t.Errorf("not found: %v", b)
	}
}

func TestNumbering_ReadDefaults(t *testing.T) {
	numberingClock = func() time.Time { return time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC) } // FY 26-27
	defer func() { numberingClock = func() time.Time { return time.Now().UTC() } }()

	sess := store.Session{UID: "u1", CompanyID: "co1"}
	// nothing stored -> every kind filled from defaults
	body := postForm(t, New(&stubCompanies{}, &stubUsers{}, &stubSessions{}), func(h *Handler) http.HandlerFunc { return h.Numbering },
		sess, "", map[string]string{})
	if body["code"] != float64(200) || body["status"] != true {
		t.Fatalf("envelope: %v", body)
	}
	num := body["data"].(map[string]any)["numbering"].(map[string]any)
	inv := num["invoice"].(map[string]any)
	if inv["prefix"] != "INV" || inv["year"] != "fy" || inv["pad"] != float64(6) || inv["separator"] != "/" {
		t.Errorf("invoice default: %v", inv)
	}
	// all four kinds present, no purchaseOrder
	for _, k := range []string{"invoice", "job", "quotation", "purchase"} {
		if _, ok := num[k]; !ok {
			t.Errorf("missing kind %q", k)
		}
	}
	if _, ok := num["purchaseOrder"]; ok {
		t.Error("read must not include purchaseOrder")
	}
	prev := body["data"].(map[string]any)["preview"].(map[string]any)
	if prev["invoice"] != "INV/26-27/000001" {
		t.Errorf("invoice preview = %v", prev["invoice"])
	}
}

func TestNumbering_ReadStoredOverride(t *testing.T) {
	numberingClock = func() time.Time { return time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC) }
	defer func() { numberingClock = func() time.Time { return time.Now().UTC() } }()

	s := &stubCompanies{numbering: map[string]json.RawMessage{
		"invoice": json.RawMessage(`{"prefix":"BILL","year":"yyyy","pad":4,"separator":"-"}`),
	}}
	body := postForm(t, New(s, &stubUsers{}, &stubSessions{}), func(h *Handler) http.HandlerFunc { return h.Numbering },
		store.Session{UID: "u1", CompanyID: "co1"}, "", map[string]string{})
	prev := body["data"].(map[string]any)["preview"].(map[string]any)
	if prev["invoice"] != "BILL-2026-0001" {
		t.Errorf("stored override preview = %v", prev["invoice"])
	}
	// an unset kind still shows its default
	if prev["job"] != "JOB/26-27/000001" {
		t.Errorf("job default preview = %v", prev["job"])
	}
}

func TestNumberingUpdate(t *testing.T) {
	numberingClock = func() time.Time { return time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC) }
	defer func() { numberingClock = func() time.Time { return time.Now().UTC() } }()
	admin := store.Session{UID: "u1", CompanyID: "co1", Role: "admin"}

	// success: normalised format stored + echoed with preview
	s := &stubCompanies{setNumberFound: true}
	body := postForm(t, New(s, &stubUsers{}, &stubSessions{}), func(h *Handler) http.HandlerFunc { return h.NumberingUpdate },
		admin, "", map[string]string{"kind": "invoice", "prefix": "  BILL  ", "year": "yyyy", "pad": "99", "separator": "-"})
	if body["code"] != float64(200) || body["message"] != "Numbering updated. Documents already issued keep the numbers they have." {
		t.Fatalf("envelope: %v", body)
	}
	if !s.setNumberCalled || s.gotNumberKind != "invoice" {
		t.Errorf("store not called for invoice: %+v", s)
	}
	// stored format is normalised: prefix trimmed, pad clamped to 8
	var stored map[string]any
	json.Unmarshal(s.gotNumberFormat, &stored)
	if stored["prefix"] != "BILL" || stored["pad"] != float64(8) {
		t.Errorf("stored format not normalised: %v", stored)
	}
	data := body["data"].(map[string]any)
	// pad clamped to 8 -> "BILL-2026-00000001"
	if data["preview"] != "BILL-2026-00000001" {
		t.Errorf("preview = %v", data["preview"])
	}

	// unknown kind -> 422
	if b := postForm(t, New(&stubCompanies{}, &stubUsers{}, &stubSessions{}), func(h *Handler) http.HandlerFunc { return h.NumberingUpdate },
		admin, "", map[string]string{"kind": "bogus", "prefix": "X"}); b["code"] != float64(422) || b["message"] != "Unknown document type." {
		t.Errorf("unknown kind: %v", b)
	}

	// empty prefix -> 422
	if b := postForm(t, New(&stubCompanies{setNumberFound: true}, &stubUsers{}, &stubSessions{}), func(h *Handler) http.HandlerFunc { return h.NumberingUpdate },
		admin, "", map[string]string{"kind": "invoice", "prefix": ""}); b["code"] != float64(422) || b["message"] != "A prefix is required - it is what tells one series from another." {
		t.Errorf("empty prefix: %v", b)
	}

	// purchaseOrder is a valid kind for update (even though read hides it)
	s2 := &stubCompanies{setNumberFound: true}
	if b := postForm(t, New(s2, &stubUsers{}, &stubSessions{}), func(h *Handler) http.HandlerFunc { return h.NumberingUpdate },
		admin, "", map[string]string{"kind": "purchaseOrder", "prefix": "PO"}); b["code"] != float64(200) {
		t.Errorf("purchaseOrder update should be accepted: %v", b)
	}

	// company gone -> 404
	if b := postForm(t, New(&stubCompanies{setNumberFound: false}, &stubUsers{}, &stubSessions{}), func(h *Handler) http.HandlerFunc { return h.NumberingUpdate },
		admin, "", map[string]string{"kind": "invoice", "prefix": "X"}); b["code"] != float64(404) || b["message"] != "Company not found." {
		t.Errorf("not found: %v", b)
	}
}

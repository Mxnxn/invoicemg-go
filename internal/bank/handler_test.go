package bank

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	neturl "net/url"
	"strings"
	"testing"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type stubBanks struct {
	banks     []store.Bank
	err       error
	got       store.ID
	created   store.Bank
	gotCreate string

	// update
	updated       store.Bank
	updateFound   bool
	gotUpdateName string
	gotOpening    *float64

	// remove
	removeInUse int
	removeFound bool
	removeErr   error
	gotRemoveID store.ID

	// report
	reportData store.BankReportData
}

func (s *stubBanks) List(_ context.Context, companyID store.ID) ([]store.Bank, error) {
	s.got = companyID
	return s.banks, s.err
}

func (s *stubBanks) Create(_ context.Context, companyID, uid store.ID, name string) (store.Bank, error) {
	s.gotCreate = name
	return s.created, s.err
}

func (s *stubBanks) Update(_ context.Context, companyID, bankID store.ID, name string, openingBalance *float64) (store.Bank, bool, error) {
	s.got = companyID
	s.gotUpdateName = name
	s.gotOpening = openingBalance
	return s.updated, s.updateFound, s.err
}

func (s *stubBanks) Remove(_ context.Context, companyID, bankID store.ID) (int, bool, error) {
	s.got = companyID
	s.gotRemoveID = bankID
	return s.removeInUse, s.removeFound, s.removeErr
}

func (s *stubBanks) ReportData(_ context.Context, companyID store.ID) (store.BankReportData, error) {
	s.got = companyID
	return s.reportData, s.err
}

func TestReport_NamesRowsFiltersAndTotals(t *testing.T) {
	s := &stubBanks{reportData: store.BankReportData{
		BatchReceives: []store.BankTxn{{BankID: "b1", Date: "2026-05-10", Amount: 1000}, {BankID: "b2", Date: "2026-05-10", Amount: 200}},
		Expenses:      []store.BankTxn{{BankID: "", Date: "2026-05-12", Amount: 50, Note: "Fuel"}}, // unassigned bucket
		Banks:         []store.BankOpening{{ID: "b1", Name: "HDFC", OpeningBalance: 0}, {ID: "b2", Name: "ICICI"}},
	}}
	// No filter: expect b1 (1000), b2 (200), Unassigned (-50).
	r := httptest.NewRequest("POST", "/", strings.NewReader(neturl.Values{}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	New(s).Report(rec, r)

	var out map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out["code"] != float64(200) {
		t.Fatalf("code = %v (%s)", out["code"], rec.Body.String())
	}
	data := out["data"].(map[string]any)
	rows := data["rows"].([]any)
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3 (b1, b2, unassigned)", len(rows))
	}
	first := rows[0].(map[string]any)
	if first["bankName"] != "HDFC" || first["current"] != float64(1000) {
		t.Errorf("row0 = %v, want HDFC/1000 (richest first)", first)
	}
	// The null-bank row is labelled Unassigned.
	last := rows[2].(map[string]any)
	if last["bankName"] != "Unassigned" {
		t.Errorf("unassigned row bankName = %v", last["bankName"])
	}
	totals := data["totals"].(map[string]any)
	if totals["credits"] != float64(1200) || totals["debits"] != float64(50) {
		t.Errorf("totals credits=%v debits=%v, want 1200/50", totals["credits"], totals["debits"])
	}
	if len(data["banks"].([]any)) != 2 {
		t.Errorf("banks list = %v, want 2", data["banks"])
	}
}

func TestReport_FilterByBank(t *testing.T) {
	s := &stubBanks{reportData: store.BankReportData{
		BatchReceives: []store.BankTxn{{BankID: "b1", Date: "2026-05-10", Amount: 1000}, {BankID: "b2", Date: "2026-05-10", Amount: 200}},
		Banks:         []store.BankOpening{{ID: "b1", Name: "HDFC"}, {ID: "b2", Name: "ICICI"}},
	}}
	r := httptest.NewRequest("POST", "/", strings.NewReader(neturl.Values{"bank_id": {"b2"}}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	New(s).Report(rec, r)

	var out map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out)
	rows := out["data"].(map[string]any)["rows"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["bankName"] != "ICICI" {
		t.Errorf("filtered rows = %v, want only ICICI", rows)
	}
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

func postCreate(t *testing.T, s *stubBanks, name string) map[string]any {
	t.Helper()
	vals := neturl.Values{}
	if name != "\x00" { // sentinel: omit the field entirely
		vals.Set("name", name)
	}
	req := httptest.NewRequest("POST", "/bank/create", strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(auth.WithSession(req.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	New(s).Create(rec, req)
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("not json: %v (%s)", err, rec.Body.String())
	}
	return body
}

func TestCreate_Success(t *testing.T) {
	s := &stubBanks{created: store.Bank{ID: "b1", UID: "u1", CompanyID: "co1", Name: "HDFC"}}
	body := postCreate(t, s, "  HDFC  ") // trimmed
	if body["code"] != float64(200) || body["message"] != "Bank created." {
		t.Fatalf("envelope: %v", body)
	}
	if s.gotCreate != "HDFC" {
		t.Errorf("name should be trimmed, got %q", s.gotCreate)
	}
	data := body["data"].(map[string]any)
	if data["_id"] != "b1" || data["name"] != "HDFC" || data["__v"] != float64(0) {
		t.Errorf("data: %v", data)
	}
	if _, ok := body["status"]; ok {
		t.Errorf("create success sends no status field")
	}
}

func TestCreate_BlankName(t *testing.T) {
	for _, name := range []string{"", "   ", "\x00"} {
		body := postCreate(t, &stubBanks{}, name)
		if body["code"] != float64(422) || body["status"] != false || body["message"] != "Invalid request." {
			t.Errorf("blank name %q should be 422: %v", name, body)
		}
	}
}

func postUpdate(t *testing.T, s *stubBanks, form neturl.Values) map[string]any {
	t.Helper()
	req := httptest.NewRequest("POST", "/bank/update", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(auth.WithSession(req.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	New(s).Update(rec, req)
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("not json: %v (%s)", err, rec.Body.String())
	}
	return body
}

// A name-only edit does not send openingBalance to the store, so a stored balance is preserved.
func TestUpdate_NameOnlyLeavesBalanceAlone(t *testing.T) {
	s := &stubBanks{updateFound: true, updated: store.Bank{ID: "b1", Name: "Axis"}}
	form := neturl.Values{}
	form.Set("bank_id", "b1")
	form.Set("name", "  Axis  ")
	body := postUpdate(t, s, form)

	if body["code"] != float64(200) || body["message"] != "Bank updated." || body["status"] != true {
		t.Fatalf("envelope: %v", body)
	}
	if s.gotUpdateName != "Axis" {
		t.Errorf("name should be trimmed, got %q", s.gotUpdateName)
	}
	if s.gotOpening != nil {
		t.Errorf("openingBalance must be nil when the field is absent, got %v", *s.gotOpening)
	}
}

// A submitted openingBalance is coerced Number-style and passed through, even when blank (->0).
func TestUpdate_OpeningBalanceSubmitted(t *testing.T) {
	s := &stubBanks{updateFound: true, updated: store.Bank{ID: "b1", Name: "Axis", OpeningBalance: -250.5}}
	form := neturl.Values{}
	form.Set("bank_id", "b1")
	form.Set("name", "Axis")
	form.Set("openingBalance", "-250.5")
	postUpdate(t, s, form)
	if s.gotOpening == nil || *s.gotOpening != -250.5 {
		t.Fatalf("openingBalance = %v, want -250.5", s.gotOpening)
	}

	// blank but present -> 0, not nil (the field was submitted)
	s2 := &stubBanks{updateFound: true, updated: store.Bank{ID: "b1", Name: "Axis"}}
	form.Set("openingBalance", "")
	postUpdate(t, s2, form)
	if s2.gotOpening == nil || *s2.gotOpening != 0 {
		t.Fatalf("blank openingBalance should coerce to 0, got %v", s2.gotOpening)
	}
}

func TestUpdate_MissingIDOrName(t *testing.T) {
	// missing bank_id
	f1 := neturl.Values{}
	f1.Set("name", "Axis")
	if body := postUpdate(t, &stubBanks{}, f1); body["code"] != float64(422) || body["message"] != "A bank needs a name." {
		t.Errorf("missing id should be 422 with bank-name wording: %v", body)
	}
	// blank name
	f2 := neturl.Values{}
	f2.Set("bank_id", "b1")
	f2.Set("name", "   ")
	if body := postUpdate(t, &stubBanks{}, f2); body["code"] != float64(422) || body["message"] != "A bank needs a name." {
		t.Errorf("blank name should be 422 with bank-name wording: %v", body)
	}
}

func TestUpdate_NotFoundIs404(t *testing.T) {
	s := &stubBanks{updateFound: false}
	f := neturl.Values{}
	f.Set("bank_id", "b1")
	f.Set("name", "Axis")
	if body := postUpdate(t, s, f); body["code"] != float64(404) || body["message"] != "Bank not found." {
		t.Errorf("unowned bank should be 404: %v", body)
	}
}

func postRemove(t *testing.T, s *stubBanks, bankID string) map[string]any {
	t.Helper()
	f := neturl.Values{}
	if bankID != "\x00" {
		f.Set("bank_id", bankID)
	}
	req := httptest.NewRequest("POST", "/bank/remove", strings.NewReader(f.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(auth.WithSession(req.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	New(s).Remove(rec, req)
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("not json: %v (%s)", err, rec.Body.String())
	}
	return body
}

func TestRemove_Success(t *testing.T) {
	s := &stubBanks{removeFound: true}
	body := postRemove(t, s, "b1")
	if body["code"] != float64(200) || body["message"] != "Bank removed." || body["status"] != true {
		t.Fatalf("envelope: %v", body)
	}
	if s.gotRemoveID != "b1" {
		t.Errorf("remove id = %q", s.gotRemoveID)
	}
}

func TestRemove_MissingID(t *testing.T) {
	if body := postRemove(t, &stubBanks{}, "\x00"); body["code"] != float64(422) || body["message"] != "Invalid request." {
		t.Errorf("missing id should be 422: %v", body)
	}
}

// A bank money has moved through is refused with the exact count and correct pluralisation.
func TestRemove_InUseRefused(t *testing.T) {
	one := postRemove(t, &stubBanks{removeInUse: 1}, "b1")
	if one["code"] != float64(422) || one["status"] != false {
		t.Fatalf("in-use should be 422: %v", one)
	}
	if msg, _ := one["message"].(string); !strings.Contains(msg, "1 transaction against it") {
		t.Errorf("singular wording wrong: %q", one["message"])
	}
	many := postRemove(t, &stubBanks{removeInUse: 3}, "b1")
	if msg, _ := many["message"].(string); !strings.Contains(msg, "3 transactions against it") {
		t.Errorf("plural wording wrong: %q", many["message"])
	}
}

func TestRemove_NotFoundIs404(t *testing.T) {
	if body := postRemove(t, &stubBanks{removeFound: false}, "b1"); body["code"] != float64(404) || body["message"] != "Bank not found." {
		t.Errorf("unowned bank should be 404: %v", body)
	}
}

package expense

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

func TestCreate_ValidationRejectsMissingOrNonPositive(t *testing.T) {
	cases := []url.Values{
		{"date": {"2026-09-10"}, "amount": {"100"}},               // no bank
		{"bank_id": {"b1"}, "amount": {"100"}},                    // no date
		{"bank_id": {"b1"}, "date": {"2026-09-10"}},               // no amount -> 0
		{"bank_id": {"b1"}, "date": {"2026-09-10"}, "amount": {"0"}},
		{"bank_id": {"b1"}, "date": {"2026-09-10"}, "amount": {"-5"}},
		{"bank_id": {"b1"}, "date": {"2026-09-10"}, "amount": {"abc"}},
	}
	for _, v := range cases {
		s := &stubExpenses{}
		rec := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/", strings.NewReader(v.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
		New(s).Create(rec, r)

		var out map[string]any
		json.Unmarshal(rec.Body.Bytes(), &out)
		if out["code"] != float64(422) {
			t.Errorf("%v -> code %v, want 422", v, out["code"])
		}
		if s.createCalled {
			t.Errorf("%v must not reach the store", v)
		}
	}
}

func TestCreate_RecordsAndEchoesPopulated(t *testing.T) {
	s := &stubExpenses{created: store.Expense{
		ID: "x9", UID: "u1", CompanyID: "co1", Date: "2026-09-10", Amount: 1200, Notes: "Ink",
		Bank: &store.ExpenseBank{ID: "b1", Name: "HDFC"},
	}}
	v := url.Values{"bank_id": {"b1"}, "date": {"2026-09-10"}, "amount": {"1200"}, "notes": {"Ink"}}
	r := httptest.NewRequest("POST", "/", strings.NewReader(v.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	New(s).Create(rec, r)

	if s.gotWrite.BankID != "b1" || s.gotWrite.Amount != 1200 || s.gotWrite.Notes != "Ink" || s.gotWrite.Date != "2026-09-10" {
		t.Errorf("write not passed through: %+v", s.gotWrite)
	}
	var out map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out["code"] != float64(200) || out["message"] != "Expense recorded." {
		t.Errorf("got code=%v msg=%v", out["code"], out["message"])
	}
	row := out["data"].(map[string]any)
	if row["_id"] != "x9" || row["amount"] != float64(1200) {
		t.Errorf("echoed row wrong: %v", row)
	}
	bank := row["bank_id"].(map[string]any)
	if bank["name"] != "HDFC" {
		t.Errorf("bank not populated on create: %v", row["bank_id"])
	}
}

func TestRemove_MissingIdAndNotFoundAndOk(t *testing.T) {
	// missing id -> 422
	s := &stubExpenses{}
	rec := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", strings.NewReader(url.Values{}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{CompanyID: "co1"}))
	New(s).Remove(rec, r)
	var out map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out["code"] != float64(422) {
		t.Errorf("missing id -> %v, want 422", out["code"])
	}

	// not found -> 404
	s = &stubExpenses{deleteFound: false}
	rec = httptest.NewRecorder()
	r = httptest.NewRequest("POST", "/", strings.NewReader(url.Values{"expense_id": {"gone"}}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{CompanyID: "co1"}))
	New(s).Remove(rec, r)
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out["code"] != float64(404) || out["message"] != "No such expense." {
		t.Errorf("not found -> code=%v msg=%v", out["code"], out["message"])
	}

	// ok -> 200, company-scoped id passed
	s = &stubExpenses{deleteFound: true}
	rec = httptest.NewRecorder()
	r = httptest.NewRequest("POST", "/", strings.NewReader(url.Values{"expense_id": {"x9"}}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{CompanyID: "co1"}))
	New(s).Remove(rec, r)
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out["code"] != float64(200) || out["message"] != "Expense removed." {
		t.Errorf("ok -> code=%v msg=%v", out["code"], out["message"])
	}
	if s.gotDeleteID != "x9" {
		t.Errorf("delete id = %q, want x9", s.gotDeleteID)
	}
}

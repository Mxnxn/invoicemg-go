package expense

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type stubExpenses struct {
	list []store.Expense
	got  store.ExpenseFilter

	created     store.Expense
	gotWrite    store.ExpenseWrite
	createCalled bool
	deleteFound bool
	gotDeleteID store.ID
}

func (s *stubExpenses) List(_ context.Context, _ store.ID, f store.ExpenseFilter) ([]store.Expense, error) {
	s.got = f
	return s.list, nil
}
func (s *stubExpenses) Create(_ context.Context, _, _ store.ID, in store.ExpenseWrite) (store.Expense, error) {
	s.createCalled = true
	s.gotWrite = in
	return s.created, nil
}
func (s *stubExpenses) Delete(_ context.Context, _, expenseID store.ID) (bool, error) {
	s.gotDeleteID = expenseID
	return s.deleteFound, nil
}

func TestList_PopulatedBankAndFilters(t *testing.T) {
	s := &stubExpenses{list: []store.Expense{
		{ID: "x1", UID: "u1", CompanyID: "co1", Date: "2026-09-10", Amount: 1200, Notes: "Ink",
			Bank: &store.ExpenseBank{ID: "b1", Name: "HDFC"}},
	}}
	body := url.Values{"bank_id": {"b1"}, "from": {"2026-09-01"}, "to": {"2026-09-30"}}.Encode()
	r := httptest.NewRequest("POST", "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	New(s).List(rec, r)

	if s.got.BankID != "b1" || s.got.From != "2026-09-01" || s.got.To != "2026-09-30" {
		t.Errorf("filter not passed through: %+v", s.got)
	}
	var out map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out)
	row := out["data"].([]any)[0].(map[string]any)
	if row["amount"] != float64(1200) || row["notes"] != "Ink" {
		t.Errorf("row wrong: %v", row)
	}
	bank, ok := row["bank_id"].(map[string]any)
	if !ok || bank["name"] != "HDFC" || bank["_id"] != "b1" {
		t.Errorf("bank not populated: %v", row["bank_id"])
	}
}

func TestList_EmptyIsArray(t *testing.T) {
	r := httptest.NewRequest("POST", "/", nil)
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	New(&stubExpenses{list: nil}).List(rec, r)
	var out map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out)
	if rows, ok := out["data"].([]any); !ok || len(rows) != 0 {
		t.Errorf("data should be [], got %v", out["data"])
	}
}

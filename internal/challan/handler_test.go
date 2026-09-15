package challan

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type stubChallans struct {
	list    []store.Challan
	got     store.ID
	created store.Challan
	gotIn   store.ChallanWrite
}

func (s *stubChallans) Create(_ context.Context, _, _ store.ID, in store.ChallanWrite) (store.Challan, error) {
	s.gotIn = in
	return s.created, nil
}
func (s *stubChallans) List(_ context.Context, companyID store.ID) ([]store.Challan, error) {
	s.got = companyID
	return s.list, nil
}

func TestGetAll(t *testing.T) {
	s := &stubChallans{list: []store.Challan{
		{ID: "ch1", UID: "u1", CompanyID: "co1", CompanyName: "Acme", Description: "Banners", Date: "2026-09-12", Type: "Delivery", Quantity: 5, Amount: 4500},
	}}
	r := httptest.NewRequest("POST", "/", nil)
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	New(s).GetAll(rec, r)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("not json: %v", err)
	}
	if s.got != "co1" {
		t.Errorf("company scope = %q", s.got)
	}
	if body["code"] != float64(200) || body["status"] != true {
		t.Errorf("envelope wrong: %v", body)
	}
	// Challan getAll deliberately sends NO message.
	if _, ok := body["message"]; ok {
		t.Errorf("challan/getAll must not send a message field")
	}
	row := body["data"].([]any)[0].(map[string]any)
	if row["companyName"] != "Acme" || row["quantity"] != float64(5) || row["type"] != "Delivery" {
		t.Errorf("row wrong: %v", row)
	}
}

func TestGetAll_EmptyIsArray(t *testing.T) {
	r := httptest.NewRequest("POST", "/", nil)
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	New(&stubChallans{list: nil}).GetAll(rec, r)
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if rows, ok := body["data"].([]any); !ok || len(rows) != 0 {
		t.Errorf("data should be [], got %v", body["data"])
	}
}

func TestNew(t *testing.T) {
	s := &stubChallans{created: store.Challan{ID: "ch1", CompanyName: "Acme", Type: "In CASH", Quantity: 5, Amount: 4500}}
	// valid, no type -> In CASH, 200
	body := postForm(t, s, "companyName=Acme&amount=4500&qty=5&date=2026-09-12&description=Banners")
	if body["code"] != float64(200) || body["status"] != true {
		t.Fatalf("new: %v", body)
	}
	if s.gotIn.Type != "In CASH" || s.gotIn.Quantity != 5 {
		t.Errorf("write not built as In CASH: %+v", s.gotIn)
	}
	if body["data"].(map[string]any)["type"] != "In CASH" {
		t.Errorf("data type: %v", body["data"])
	}
	// missing field -> 422 with status:true (Node quirk)
	body = postForm(t, &stubChallans{}, "companyName=Acme&amount=4500&qty=5&date=2026-09-12")
	if body["code"] != float64(422) || body["status"] != true {
		t.Errorf("missing description should 422 status:true: %v", body)
	}
	// type present -> Node TDZ crash -> 500
	body = postForm(t, &stubChallans{}, "companyName=Acme&amount=1&qty=1&date=d&description=x&type=In+BILLING")
	if body["code"] != float64(500) {
		t.Errorf("type present should 500: %v", body)
	}
}

func postForm(t *testing.T, s store.Challans, form string) map[string]any {
	t.Helper()
	r := httptest.NewRequest("POST", "/", strings.NewReader(form))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	New(s).New(rec, r)
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	return body
}

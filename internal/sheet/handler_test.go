package sheet

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

type stubSheets struct {
	date    string
	entries []store.Entry
	found   bool
	gotSid  store.ID
}

func (s *stubSheets) Get(_ context.Context, _, sheetID store.ID) (string, []store.Entry, bool, error) {
	s.gotSid = sheetID
	return s.date, s.entries, s.found, nil
}

func do(t *testing.T, s store.Sheets, form map[string]string) map[string]any {
	t.Helper()
	vals := neturl.Values{}
	for k, v := range form {
		vals.Set(k, v)
	}
	r := httptest.NewRequest("POST", "/", strings.NewReader(vals.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	New(s).Get(rec, r)
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("not json: %v (%s)", err, rec.Body.String())
	}
	return out
}

func TestSheetGet_GroupsByClient(t *testing.T) {
	c1 := &store.EntryClient{ID: "c1", ClientName: "Priya", ClientFirm: "Acme"}
	c2 := &store.EntryClient{ID: "c2", ClientName: "Ravi", ClientFirm: "Neo"}
	s := &stubSheets{found: true, date: "2026-09-15", entries: []store.Entry{
		{ID: "e1", UID: "u1", ClientID: "c1", Client: c1, Amount: 100},
		{ID: "e2", UID: "u1", ClientID: "c2", Client: c2, Amount: 200},
		{ID: "e3", UID: "u1", ClientID: "c1", Client: c1, Amount: 300},
		{ID: "e4", ClientID: "cx", Client: nil}, // dropped: no populated client
	}}
	body := do(t, s, map[string]string{"sid": "s1"})
	if body["code"] != float64(200) || body["message"] != "Operation successful." {
		t.Fatalf("envelope: %v", body)
	}
	if body["date"] != "2026-09-15" {
		t.Errorf("top-level date wrong: %v", body["date"])
	}
	cards := body["data"].([]any)
	if len(cards) != 2 {
		t.Fatalf("want 2 client cards, got %d: %v", len(cards), cards)
	}
	first := cards[0].(map[string]any)
	if first["_id"] != "c1" || first["clientName"] != "Priya" {
		t.Errorf("first card wrong: %v", first)
	}
	if ents := first["entries"].([]any); len(ents) != 2 {
		t.Errorf("client c1 should have 2 entries, got %d", len(ents))
	}
	if s.gotSid != "s1" {
		t.Errorf("sid not passed: %v", s.gotSid)
	}
}

func TestSheetGet_Validation(t *testing.T) {
	body := do(t, &stubSheets{}, map[string]string{})
	if body["code"] != float64(422) {
		t.Errorf("missing sid should 422: %v", body)
	}
}

func TestSheetGet_NotFound(t *testing.T) {
	body := do(t, &stubSheets{found: false}, map[string]string{"sid": "nope"})
	if body["code"] != float64(404) || body["message"] != "Sheet not found." {
		t.Errorf("not found: %v", body)
	}
}

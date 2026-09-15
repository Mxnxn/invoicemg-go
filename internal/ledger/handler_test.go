package ledger

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

type stubLedger struct {
	data      store.LedgerClientData
	dues      store.ClientDuesData
	gotCo     store.ID
	gotClient store.ID
}

func (s *stubLedger) ClientStatement(_ context.Context, companyID, clientID store.ID) (store.LedgerClientData, error) {
	s.gotCo, s.gotClient = companyID, clientID
	return s.data, nil
}

func (s *stubLedger) ClientDues(_ context.Context, companyID store.ID) (store.ClientDuesData, error) {
	s.gotCo = companyID
	return s.dues, nil
}

func run(t *testing.T, s store.Ledger, form url.Values) map[string]any {
	t.Helper()
	r := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	New(s).Client(rec, r)
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("not json: %v\n%s", err, rec.Body.String())
	}
	return body
}

func TestClient_MissingID(t *testing.T) {
	body := run(t, &stubLedger{}, url.Values{})
	if body["code"] != float64(422) || body["status"] != false {
		t.Errorf("want 422/false, got %v", body)
	}
	if _, ok := body["data"]; ok {
		t.Errorf("422 must carry no data: %v", body)
	}
}

func TestClient_NotFound(t *testing.T) {
	body := run(t, &stubLedger{data: store.LedgerClientData{Found: false}}, url.Values{"client_id": {"c1"}})
	if body["code"] != float64(404) || body["status"] != false || body["message"] != "Client not found." {
		t.Errorf("want 404 not found, got %v", body)
	}
}

// A statement with two bills and one same-day receipt, no window: running balance, string
// money, no opening-balance row, and currentBalance == closingBalance.
func TestClient_RunningBalanceNoWindow(t *testing.T) {
	s := &stubLedger{data: store.LedgerClientData{
		Found: true, ClientName: "Acme", ClientFirm: "Acme Signs", ClientGST: "24AAA", ClientAddress: "Baroda",
		Txns: []store.LedgerTxn{
			{Date: "2026-01-10", Type: "Sales Invoice", InvoiceNo: "INV-1", Bill: 1000, Seq: 0},
			// same-day receipt must sort AFTER the bill (Seq 1) even though listed first here
			{Date: "2026-02-05", Type: "Receipts", InvoiceNo: "INV-2", Receipt: 500, Seq: 1},
			{Date: "2026-02-05", Type: "Sales Invoice", InvoiceNo: "INV-2", Bill: 750, Seq: 0},
		},
	}}
	body := run(t, s, url.Values{"client_id": {"c1"}})
	if s.gotCo != "co1" || s.gotClient != "c1" {
		t.Errorf("scope = %q/%q", s.gotCo, s.gotClient)
	}
	if body["code"] != float64(200) || body["message"] != "Operation successful." {
		t.Fatalf("envelope: %v", body)
	}
	if _, ok := body["status"]; ok {
		t.Errorf("success carries no status field")
	}
	data := body["data"].(map[string]any)
	if data["clientName"] != "Acme" || data["clientGST"] != "24AAA" {
		t.Errorf("client fields: %v", data)
	}
	if data["openingBalance"] != "0.00" {
		t.Errorf("opening without from must be 0.00, got %v", data["openingBalance"])
	}
	rows := data["rows"].([]any)
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d: %v", len(rows), rows)
	}
	// order: INV-1 (bill 1000, bal 1000), INV-2 bill 750 (bal 1750), INV-2 receipt 500 (bal 1250)
	r0 := rows[0].(map[string]any)
	if r0["bill"] != float64(1000) || r0["balance"] != "1000.00" || r0["receipt"] != nil {
		t.Errorf("row0: %v", r0)
	}
	r1 := rows[1].(map[string]any)
	if r1["type"] != "Sales Invoice" || r1["bill"] != float64(750) || r1["balance"] != "1750.00" {
		t.Errorf("row1 (same-day bill before receipt): %v", r1)
	}
	r2 := rows[2].(map[string]any)
	if r2["type"] != "Receipts" || r2["receipt"] != float64(500) || r2["bill"] != nil || r2["balance"] != "1250.00" {
		t.Errorf("row2: %v", r2)
	}
	if data["closingBalance"] != "1250.00" || data["currentBalance"] != "1250.00" {
		t.Errorf("closing/current: %v", data)
	}
}

// With a `from`, pre-window txns fold into the opening balance and an Opening Balance row leads.
// currentBalance stays unbounded by `to`.
func TestClient_WindowOpeningAndCurrent(t *testing.T) {
	s := &stubLedger{data: store.LedgerClientData{
		Found: true, ClientName: "Acme",
		Txns: []store.LedgerTxn{
			{Date: "2025-12-01", Type: "Sales Invoice", Bill: 2000, Seq: 0}, // before from -> opening
			{Date: "2026-01-15", Type: "Receipts", Receipt: 500, Seq: 1},    // in window
			{Date: "2026-03-01", Type: "Sales Invoice", Bill: 300, Seq: 0},  // after to -> only in current
		},
	}}
	body := run(t, s, url.Values{"client_id": {"c1"}, "from": {"2026-01-01"}, "to": {"2026-02-01"}})
	data := body["data"].(map[string]any)
	if data["openingBalance"] != "2000.00" {
		t.Errorf("opening = %v", data["openingBalance"])
	}
	rows := data["rows"].([]any)
	if len(rows) != 2 {
		t.Fatalf("want opening row + 1 in-window, got %d: %v", len(rows), rows)
	}
	op := rows[0].(map[string]any)
	if op["type"] != "Opening Balance" || op["date"] != "2026-01-01" || op["bill"] != nil || op["balance"] != "2000.00" {
		t.Errorf("opening row: %v", op)
	}
	in := rows[1].(map[string]any)
	if in["receipt"] != float64(500) || in["balance"] != "1500.00" {
		t.Errorf("in-window row: %v", in)
	}
	// closing counts only through `to` (2000-500=1500); current includes the after-to bill (1800).
	if data["closingBalance"] != "1500.00" {
		t.Errorf("closing = %v", data["closingBalance"])
	}
	if data["currentBalance"] != "1800.00" {
		t.Errorf("current = %v", data["currentBalance"])
	}
}

// A credit balance (receipts exceed bills) rounds toward +Inf like JS Math.round: -0.005*100
// rounds up. Confirms roundOff formats negatives the Node way.
func TestClient_NegativeRounding(t *testing.T) {
	s := &stubLedger{data: store.LedgerClientData{
		Found: true,
		Txns: []store.LedgerTxn{
			{Date: "2026-01-01", Type: "Sales Invoice", Bill: 100, Seq: 0},
			{Date: "2026-01-02", Type: "Receipts", Receipt: 150.005, Seq: 1},
		},
	}}
	body := run(t, s, url.Values{"client_id": {"c1"}})
	data := body["data"].(map[string]any)
	// balance = 100 - 150.005 = -50.005; Math.round(-5000.5)/100 = -50.00 (round half toward +Inf)
	if data["closingBalance"] != "-50.00" {
		t.Errorf("negative round: want -50.00, got %v", data["closingBalance"])
	}
}

func TestDues_RowsTotalsAndFilter(t *testing.T) {
	s := &stubLedger{dues: store.ClientDuesData{
		Dues: []store.ClientDue{
			{ClientID: "a", Billed: 1250.01, Received: 500, Due: 750.01},
			{ClientID: "c", Billed: 800, Received: 300, Due: 500},
			{ClientID: "b", Billed: 500, Received: 500, Due: 0},
			{ClientID: "gone", Billed: 900, Received: 0, Due: 900}, // deleted client -> dropped
		},
		Clients: map[string]store.ClientBrief{
			"a": {Name: "Ann", Firm: "Ann Co", Phone: "111"},
			"b": {Name: "Bob", Firm: "Bob Co", Phone: ""},
			"c": {Name: "Cy", Firm: "Cy Co", Phone: "333"},
		},
	}}
	body := run2(t, s)
	if body["code"] != float64(200) || body["message"] != "Operation successful." {
		t.Fatalf("envelope: %v", body)
	}
	data := body["data"].(map[string]any)
	rows := data["rows"].([]any)
	if len(rows) != 3 {
		t.Fatalf("deleted client must be dropped; want 3 rows, got %d: %v", len(rows), rows)
	}
	r0 := rows[0].(map[string]any)
	if r0["clientId"] != "a" || r0["clientName"] != "Ann" || r0["due"] != 750.01 || r0["clientPhone"] != "111" {
		t.Errorf("row0: %v", r0)
	}
	totals := data["totals"].(map[string]any)
	// billed sums all kept rows (1250.01+800+500=2550.01); due sums only positive-due (750.01+500=1250.01);
	// outstandingClients counts positive-due rows (a, c) = 2.
	if totals["billed"] != 2550.01 || totals["received"] != float64(1300) {
		t.Errorf("totals billed/received: %v", totals)
	}
	if totals["due"] != 1250.01 || totals["outstandingClients"] != float64(2) {
		t.Errorf("totals due/outstanding: %v", totals)
	}
}

func run2(t *testing.T, s store.Ledger) map[string]any {
	t.Helper()
	r := httptest.NewRequest("POST", "/", nil)
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	New(s).Dues(rec, r)
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("not json: %v\n%s", err, rec.Body.String())
	}
	return body
}

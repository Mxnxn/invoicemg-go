package purchaseorder

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

func runForm(s store.PurchaseOrders, method func(*Handler, http.ResponseWriter, *http.Request), vals url.Values) map[string]any {
	r := httptest.NewRequest("POST", "/", strings.NewReader(vals.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1", Role: "admin"}))
	rec := httptest.NewRecorder()
	method(New(s), rec, r)
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	return body
}

func TestApprove_Statuses(t *testing.T) {
	cases := []struct {
		status store.POActionStatus
		want   float64
	}{
		{store.POActionOK, 200},
		{store.POActionNotFound, 404},
		{store.POActionConverted, 409},
		{store.POActionNoRows, 422},
		{store.POActionAlreadyApproved, 409},
	}
	for _, c := range cases {
		s := &stubPO{actionStatus: c.status, actionPO: store.PurchaseOrder{ID: "p1"}}
		body := runForm(s, (*Handler).Approve, url.Values{"po_id": {"p1"}})
		if body["code"] != c.want {
			t.Errorf("status %d -> code %v, want %v", c.status, body["code"], c.want)
		}
	}
	// missing id -> 422 without touching the store
	body := runForm(&stubPO{}, (*Handler).Approve, url.Values{})
	if body["code"] != float64(422) {
		t.Errorf("no id -> %v, want 422", body["code"])
	}
}

func TestRevoke_Statuses(t *testing.T) {
	if body := runForm(&stubPO{actionStatus: store.POActionNotApproved}, (*Handler).Revoke, url.Values{"po_id": {"p1"}}); body["code"] != float64(409) {
		t.Errorf("not approved -> %v, want 409", body["code"])
	}
	if body := runForm(&stubPO{actionStatus: store.POActionOK, actionPO: store.PurchaseOrder{ID: "p1"}}, (*Handler).Revoke, url.Values{"po_id": {"p1"}}); body["code"] != float64(200) {
		t.Errorf("ok -> %v, want 200", body["code"])
	}
}

func TestNote_Validation(t *testing.T) {
	if body := runForm(&stubPO{}, (*Handler).Note, url.Values{"po_id": {"p1"}, "text": {"  "}}); body["code"] != float64(422) {
		t.Errorf("blank text -> %v, want 422", body["code"])
	}
	if body := runForm(&stubPO{noteFound: false}, (*Handler).Note, url.Values{"po_id": {"p1"}, "text": {"hi"}}); body["code"] != float64(404) {
		t.Errorf("po missing -> %v, want 404", body["code"])
	}
	s := &stubPO{noteFound: true, note: store.PONote{ID: "n1", Text: "hi"}}
	if body := runForm(s, (*Handler).Note, url.Values{"po_id": {"p1"}, "text": {"hi"}}); body["code"] != float64(200) || s.gotNoteText != "hi" {
		t.Errorf("ok -> code=%v text=%q", body["code"], s.gotNoteText)
	}
}

func TestNoteEdit_Statuses(t *testing.T) {
	if body := runForm(&stubPO{}, (*Handler).NoteEdit, url.Values{"note_id": {""}, "text": {"x"}}); body["code"] != float64(422) {
		t.Errorf("no id -> %v, want 422", body["code"])
	}
	if body := runForm(&stubPO{noteFound: false}, (*Handler).NoteEdit, url.Values{"note_id": {"n1"}, "text": {"x"}}); body["code"] != float64(404) {
		t.Errorf("missing -> %v, want 404", body["code"])
	}
	if body := runForm(&stubPO{noteFound: true, noteForbidden: true}, (*Handler).NoteEdit, url.Values{"note_id": {"n1"}, "text": {"x"}}); body["code"] != float64(403) {
		t.Errorf("forbidden -> %v, want 403", body["code"])
	}
	if body := runForm(&stubPO{noteFound: true, note: store.PONote{ID: "n1"}}, (*Handler).NoteEdit, url.Values{"note_id": {"n1"}, "text": {"x"}}); body["code"] != float64(200) {
		t.Errorf("ok -> %v, want 200", body["code"])
	}
}

func TestConvert_Statuses(t *testing.T) {
	// validation
	if body := runForm(&stubPO{}, (*Handler).Convert, url.Values{"po_id": {"p1"}, "date": {"2026-09-01"}}); body["code"] != float64(422) {
		t.Errorf("no invoiceNumber -> %v, want 422", body["code"])
	}
	v := url.Values{"po_id": {"p1"}, "invoiceNumber": {"SUP-9"}, "date": {"2026-09-01"}}
	if body := runForm(&stubPO{actionStatus: store.POActionNotApproved}, (*Handler).Convert, v); body["code"] != float64(409) {
		t.Errorf("not approved -> %v, want 409", body["code"])
	}
	if body := runForm(&stubPO{actionStatus: store.POActionConverted}, (*Handler).Convert, v); body["code"] != float64(409) {
		t.Errorf("already converted -> %v, want 409", body["code"])
	}
	if body := runForm(&stubPO{actionStatus: store.POActionNotFound}, (*Handler).Convert, v); body["code"] != float64(404) {
		t.Errorf("not found -> %v, want 404", body["code"])
	}
	s := &stubPO{actionStatus: store.POActionOK, convertInvoiceID: "inv1", actionPO: store.PurchaseOrder{ID: "p1", SupplierID: "s1", Total: 236}}
	body := runForm(s, (*Handler).Convert, v)
	if body["code"] != float64(200) || s.gotConvertNumber != "SUP-9" {
		t.Fatalf("ok -> code=%v number=%q", body["code"], s.gotConvertNumber)
	}
	data := body["data"].(map[string]any)
	inv := data["invoice"].(map[string]any)
	if inv["_id"] != "inv1" || inv["invoiceNumber"] != "SUP-9" || inv["total"] != float64(236) {
		t.Errorf("invoice = %v", inv)
	}
}

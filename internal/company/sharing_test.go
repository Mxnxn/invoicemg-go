package company

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"strings"
	"testing"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

func sharingReq(sess store.Session, values neturl.Values) *http.Request {
	r := httptest.NewRequest("POST", "/", strings.NewReader(values.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r.WithContext(auth.WithSession(r.Context(), sess))
}

func sharingBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, rec.Body.String())
	}
	return body
}

func TestSharing_TurnsOn(t *testing.T) {
	sc := &stubCompanies{setReportsFound: true}
	h := New(sc, &stubUsers{}, &stubSessions{})
	rec := httptest.NewRecorder()
	h.Sharing(rec, sharingReq(store.Session{UID: "u1", CompanyID: "co1", Role: "admin"},
		neturl.Values{"reportsAcrossCompanies": {"true"}}))

	body := sharingBody(t, rec)
	if body["code"] != float64(200) {
		t.Fatalf("code = %v, want 200 (%s)", body["code"], rec.Body.String())
	}
	if !sc.setReportsCalled || sc.setReportsOn != true {
		t.Errorf("SetReportsAcrossCompanies called=%v on=%v, want called with true", sc.setReportsCalled, sc.setReportsOn)
	}
	data := body["data"].(map[string]any)
	if data["reportsAcrossCompanies"] != true {
		t.Errorf("data.reportsAcrossCompanies = %v, want true", data["reportsAcrossCompanies"])
	}
}

// Only the exact string "true" turns it on - "false", a typo or an absent field all resolve to
// off, matching the fail-closed rule the scope resolver follows.
func TestSharing_AnythingButTrueIsOff(t *testing.T) {
	for _, v := range []string{"false", "1", "yes", "TRUE", ""} {
		sc := &stubCompanies{setReportsFound: true}
		h := New(sc, &stubUsers{}, &stubSessions{})
		rec := httptest.NewRecorder()
		h.Sharing(rec, sharingReq(store.Session{UID: "u1", CompanyID: "co1", Role: "admin"},
			neturl.Values{"reportsAcrossCompanies": {v}}))
		if sc.setReportsOn != false {
			t.Errorf("value %q -> on=%v, want false", v, sc.setReportsOn)
		}
		data := sharingBody(t, rec)["data"].(map[string]any)
		if data["reportsAcrossCompanies"] != false {
			t.Errorf("value %q -> data.reportsAcrossCompanies=%v, want false", v, data["reportsAcrossCompanies"])
		}
	}
}

func TestSharing_NoCompanyOnSession(t *testing.T) {
	sc := &stubCompanies{setReportsFound: true}
	h := New(sc, &stubUsers{}, &stubSessions{})
	rec := httptest.NewRecorder()
	h.Sharing(rec, sharingReq(store.Session{UID: "u1", Role: "admin"},
		neturl.Values{"reportsAcrossCompanies": {"true"}}))

	body := sharingBody(t, rec)
	if body["code"] != float64(404) || body["message"] != "No company found for this account." {
		t.Errorf("got code=%v msg=%v, want 404 no-company", body["code"], body["message"])
	}
	if sc.setReportsCalled {
		t.Error("must not touch the store when the session has no company")
	}
}

func TestSharing_CompanyGone(t *testing.T) {
	sc := &stubCompanies{setReportsFound: false} // store reports no such owner-scoped company
	h := New(sc, &stubUsers{}, &stubSessions{})
	rec := httptest.NewRecorder()
	h.Sharing(rec, sharingReq(store.Session{UID: "u1", CompanyID: "co1", Role: "admin"},
		neturl.Values{"reportsAcrossCompanies": {"true"}}))

	body := sharingBody(t, rec)
	if body["code"] != float64(404) || body["message"] != "Company not found." {
		t.Errorf("got code=%v msg=%v, want 404 not-found", body["code"], body["message"])
	}
}

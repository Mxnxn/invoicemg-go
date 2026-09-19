package settings

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

// stubSettings records what the handler asked of the store and hands back canned answers.
type stubSettings struct {
	get        json.RawMessage
	getErr     error
	set        json.RawMessage
	setErr     error
	gotOwner   store.ID
	gotSection store.SettingsSection
	gotValue   json.RawMessage
	setCalled  bool
}

func (s *stubSettings) Get(_ context.Context, ownerID store.ID, section store.SettingsSection) (json.RawMessage, error) {
	s.gotOwner = ownerID
	s.gotSection = section
	return s.get, s.getErr
}

func (s *stubSettings) Set(_ context.Context, ownerID store.ID, section store.SettingsSection, value json.RawMessage) (json.RawMessage, error) {
	s.setCalled = true
	s.gotOwner = ownerID
	s.gotSection = section
	s.gotValue = value
	return s.set, s.setErr
}

func decode(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("not json: %v (%s)", err, string(raw))
	}
	return body
}

// hasDataKey reports whether the wire body carried a "data" key at all - the difference between
// `data:null` (present, value null) and an omitted key, which the read routes must get right.
func hasDataKey(raw []byte) bool {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return false
	}
	_, ok := probe["data"]
	return ok
}

// The owner is personId for an employee session and uid for an admin one (personId || uid).
func TestRead_OwnerIsPersonThenUID(t *testing.T) {
	// employee session: personId wins
	s := &stubSettings{get: nil}
	req := httptest.NewRequest("POST", "/settings/appearance", nil)
	req = req.WithContext(auth.WithSession(req.Context(), store.Session{UID: "admin1", PersonID: "emp1"}))
	rec := httptest.NewRecorder()
	New(s).Appearance(rec, req)
	if s.gotOwner != "emp1" {
		t.Errorf("owner = %q, want the personId emp1", s.gotOwner)
	}

	// admin session: no personId, falls back to uid
	s2 := &stubSettings{get: nil}
	req2 := httptest.NewRequest("POST", "/settings/appearance", nil)
	req2 = req2.WithContext(auth.WithSession(req2.Context(), store.Session{UID: "admin1"}))
	rec2 := httptest.NewRecorder()
	New(s2).Appearance(rec2, req2)
	if s2.gotOwner != "admin1" {
		t.Errorf("owner = %q, want the uid admin1", s2.gotOwner)
	}
}

// No row yet answers `{code:200, message:"Operation successful.", status:true, data:null}` -
// the data key present and null, not omitted, matching Node's `data: row ? ... : null`.
func TestRead_NoRowSendsNull(t *testing.T) {
	s := &stubSettings{get: nil}
	req := httptest.NewRequest("POST", "/settings/appearance", nil)
	req = req.WithContext(auth.WithSession(req.Context(), store.Session{UID: "u1"}))
	rec := httptest.NewRecorder()
	New(s).Appearance(rec, req)

	if !hasDataKey(rec.Body.Bytes()) {
		t.Fatal("data key must be present (and null), not omitted")
	}
	body := decode(t, rec.Body.Bytes())
	if body["code"] != float64(200) || body["message"] != "Operation successful." || body["status"] != true {
		t.Fatalf("envelope: %v", body)
	}
	if body["data"] != nil {
		t.Errorf("data = %v, want null", body["data"])
	}
	if s.gotSection != store.SettingsAppearance {
		t.Errorf("section = %q, want appearance", s.gotSection)
	}
}

// A stored blob comes back verbatim as the data object.
func TestRead_ReturnsBlob(t *testing.T) {
	s := &stubSettings{get: json.RawMessage(`{"fontScale":1.25,"accent":"violet"}`)}
	req := httptest.NewRequest("POST", "/settings/tables", nil)
	req = req.WithContext(auth.WithSession(req.Context(), store.Session{UID: "u1"}))
	rec := httptest.NewRecorder()
	New(s).Tables(rec, req)

	if s.gotSection != store.SettingsTables {
		t.Errorf("section = %q, want tables", s.gotSection)
	}
	body := decode(t, rec.Body.Bytes())
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("data not an object: %v", body["data"])
	}
	if data["fontScale"] != float64(1.25) || data["accent"] != "violet" {
		t.Errorf("data round-trip wrong: %v", data)
	}
}

func TestRead_StoreErrorIs500(t *testing.T) {
	s := &stubSettings{getErr: context.DeadlineExceeded}
	req := httptest.NewRequest("POST", "/settings/appearance", nil)
	req = req.WithContext(auth.WithSession(req.Context(), store.Session{UID: "u1"}))
	rec := httptest.NewRecorder()
	New(s).Appearance(rec, req)
	if body := decode(t, rec.Body.Bytes()); body["code"] != float64(500) {
		t.Errorf("store error should be 500: %v", body)
	}
}

func postUpdate(t *testing.T, s *stubSettings, field, value string, omit bool) *httptest.ResponseRecorder {
	t.Helper()
	vals := neturl.Values{}
	if !omit {
		vals.Set(field, value)
	}
	req := httptest.NewRequest("POST", "/settings/"+field+"/update", strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(auth.WithSession(req.Context(), store.Session{UID: "u1"}))
	rec := httptest.NewRecorder()
	if field == "appearance" {
		New(s).AppearanceUpdate(rec, req)
	} else {
		New(s).TablesUpdate(rec, req)
	}
	return rec
}

// A valid object is stored and echoed with "Settings saved.".
func TestUpdate_Success(t *testing.T) {
	s := &stubSettings{set: json.RawMessage(`{"fontScale":2}`)}
	rec := postUpdate(t, s, "appearance", `{"fontScale":2}`, false)

	body := decode(t, rec.Body.Bytes())
	if body["code"] != float64(200) || body["message"] != "Settings saved." || body["status"] != true {
		t.Fatalf("envelope: %v", body)
	}
	if !s.setCalled || s.gotSection != store.SettingsAppearance {
		t.Errorf("store.Set not called for appearance: %+v", s)
	}
	if string(s.gotValue) != `{"fontScale":2}` {
		t.Errorf("stored value = %s, want the posted object", s.gotValue)
	}
	data := body["data"].(map[string]any)
	if data["fontScale"] != float64(2) {
		t.Errorf("data = %v", data)
	}
}

// The empty object is a valid object, so it is accepted (Node's `{}` case).
func TestUpdate_EmptyObjectAccepted(t *testing.T) {
	s := &stubSettings{set: json.RawMessage(`{}`)}
	rec := postUpdate(t, s, "tables", `{}`, false)
	body := decode(t, rec.Body.Bytes())
	if body["code"] != float64(200) || !s.setCalled {
		t.Fatalf("`{}` should be accepted: %v", body)
	}
	if s.gotSection != store.SettingsTables {
		t.Errorf("section = %q, want tables", s.gotSection)
	}
}

// Everything that is not a non-array object is 422 "Invalid request." with status:false, and
// the store is never touched - Node validates before writing.
func TestUpdate_NonObjectsAre422(t *testing.T) {
	cases := map[string]struct {
		value string
		omit  bool
	}{
		"absent":     {"", true},
		"empty":      {"", false},
		"malformed":  {"{bad", false},
		"null":       {"null", false},
		"array":      {"[]", false},
		"number":     {"5", false},
		"string":     {`"hi"`, false},
		"boolean":    {"true", false},
		"whitespace": {"   ", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := &stubSettings{}
			rec := postUpdate(t, s, "appearance", tc.value, tc.omit)
			body := decode(t, rec.Body.Bytes())
			if body["code"] != float64(422) || body["message"] != "Invalid request." || body["status"] != false {
				t.Errorf("%s (%q) should be 422: %v", name, tc.value, body)
			}
			if s.setCalled {
				t.Errorf("%s reached the store; validation must precede the write", name)
			}
		})
	}
}

func TestUpdate_StoreErrorIs500(t *testing.T) {
	s := &stubSettings{setErr: context.DeadlineExceeded}
	rec := postUpdate(t, s, "appearance", `{"a":1}`, false)
	if body := decode(t, rec.Body.Bytes()); body["code"] != float64(500) {
		t.Errorf("store error should be 500: %v", body)
	}
}

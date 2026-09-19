package whatsapp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

func tplHandler(graphBase string, c store.Companies) *Handler {
	return &Handler{companies: c, graphBase: graphBase, httpClient: &http.Client{Timeout: 5 * time.Second}}
}

func doTemplates(t *testing.T, h *Handler) map[string]any {
	t.Helper()
	r := httptest.NewRequest("POST", "/", nil)
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	h.Templates(rec, r)
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("not json: %v (%s)", err, rec.Body.String())
	}
	return body
}

func tplConfigured() *stubCompanies {
	return &stubCompanies{found: true, company: store.Company{WaAPIToken: "tok", WaBusinessAccountID: "BIZ"}}
}

func TestTemplates_Unconfigured(t *testing.T) {
	// no token
	if b := doTemplates(t, tplHandler("", &stubCompanies{found: true, company: store.Company{WaBusinessAccountID: "BIZ"}})); b["code"] != float64(422) {
		t.Errorf("missing token should be 422: %v", b)
	}
	// no business account id
	if b := doTemplates(t, tplHandler("", &stubCompanies{found: true, company: store.Company{WaAPIToken: "tok"}})); b["code"] != float64(422) {
		t.Errorf("missing business id should be 422: %v", b)
	}
}

func TestTemplates_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// the businessAccountId and version must be in the path, and the bearer token present
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("auth header = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[
			{"id":"1","name":"job_ready","language":"en","status":"APPROVED","category":"UTILITY",
			 "components":[
				{"type":"HEADER","format":"TEXT","text":"Your order"},
				{"type":"BODY","text":"Hello {{1}}, your job {{2}} is ready."},
				{"type":"BUTTONS","buttons":[{"type":"URL","url":"https://x.test/{{1}}"}]}
			 ]},
			{"id":"2","name":"plain","language":"en","status":"PENDING","category":"MARKETING",
			 "components":[
				{"type":"HEADER","format":"IMAGE"},
				{"type":"BODY","text":"No variables here."},
				{"type":"BUTTONS","buttons":[{"type":"QUICK_REPLY","text":"Stop"}]}
			 ]}
		]}`))
	}))
	defer srv.Close()

	body := doTemplates(t, tplHandler(srv.URL, tplConfigured()))
	if body["code"] != float64(200) || body["status"] != true {
		t.Fatalf("envelope: %v", body)
	}
	rows := body["data"].([]any)
	if len(rows) != 2 {
		t.Fatalf("want 2 templates, got %d", len(rows))
	}
	t0 := rows[0].(map[string]any)
	if t0["id"] != "1" || t0["name"] != "job_ready" || t0["status"] != "APPROVED" {
		t.Errorf("t0 basics: %v", t0)
	}
	if t0["headerText"] != "Your order" || t0["bodyText"] != "Hello {{1}}, your job {{2}} is ready." {
		t.Errorf("t0 text: %v", t0)
	}
	if t0["bodyVariables"] != float64(2) {
		t.Errorf("t0 bodyVariables = %v, want 2", t0["bodyVariables"])
	}
	if t0["hasUrlButton"] != true || t0["urlButtonHasVariable"] != true {
		t.Errorf("t0 url button flags: %v", t0)
	}

	t1 := rows[1].(map[string]any)
	// non-TEXT header -> empty headerText; no {{n}} -> 0 vars; no URL button
	if t1["headerText"] != "" || t1["bodyVariables"] != float64(0) {
		t.Errorf("t1 header/vars: %v", t1)
	}
	if t1["hasUrlButton"] != false || t1["urlButtonHasVariable"] != false {
		t.Errorf("t1 url flags: %v", t1)
	}
}

func TestTemplates_MetaError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"code":100,"message":"Unsupported get request."}}`))
	}))
	defer srv.Close()
	b := doTemplates(t, tplHandler(srv.URL, tplConfigured()))
	if b["code"] != float64(502) || b["message"] != "Unsupported get request." {
		t.Errorf("meta error should surface as 502 with its message: %v", b)
	}
}

func TestTemplates_TransportError(t *testing.T) {
	// a base that no server is listening on -> transport error -> generic 502
	b := doTemplates(t, tplHandler("http://127.0.0.1:0", tplConfigured()))
	if b["code"] != float64(502) || b["message"] != "Couldn't load templates from WhatsApp." {
		t.Errorf("transport error should be generic 502: %v", b)
	}
}

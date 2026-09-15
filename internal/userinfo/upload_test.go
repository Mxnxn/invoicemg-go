package userinfo

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"testing"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

func multipartReq(t *testing.T, field, filename, contentType string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="`+field+`"; filename="`+filename+`"`)
	h.Set("Content-Type", contentType)
	pw, _ := mw.CreatePart(h)
	pw.Write(content)
	mw.Close()
	return &buf, mw.FormDataContentType()
}

func TestUpload(t *testing.T) {
	dir := t.TempDir()
	u := &stubUsers{user: store.User{Name: "Owner", Email: "o@x.test"}}
	c := &stubCompanies{updateFound: true, updated: store.Company{ID: "co1"}}
	h := New(u, c, dir)

	body, ctype := multipartReq(t, "userDP", "logo.png", "image/png", []byte("\x89PNG fake data"))
	r := httptest.NewRequest("POST", "/", body)
	r.Header.Set("Content-Type", ctype)
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	h.Upload(rec, r)
	var out map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out["code"] != float64(200) || out["message"] != "Upload successful" {
		t.Fatalf("upload: %v", out)
	}
	if c.gotPatch.URL == nil || *c.gotPatch.URL == "" {
		t.Fatalf("company url not patched: %+v", c.gotPatch)
	}
	// file was written under the uploads dir
	if _, err := os.Stat(filepath.Join(dir, *c.gotPatch.URL)); err != nil {
		t.Errorf("uploaded file missing: %v", err)
	}
	// filename shape: millis_logo.png
	if got := *c.gotPatch.URL; filepath.Ext(got) != ".png" || !bytesContains(got, "_logo.png") {
		t.Errorf("filename shape: %s", got)
	}

	// non-image -> 200 Invalid request., nothing patched
	c2 := &stubCompanies{updateFound: true}
	body2, ctype2 := multipartReq(t, "userDP", "note.txt", "text/plain", []byte("hi"))
	r2 := httptest.NewRequest("POST", "/", body2)
	r2.Header.Set("Content-Type", ctype2)
	r2 = r2.WithContext(auth.WithSession(r2.Context(), store.Session{UID: "u1", CompanyID: "co1"}))
	rec2 := httptest.NewRecorder()
	New(u, c2, dir).Upload(rec2, r2)
	json.Unmarshal(rec2.Body.Bytes(), &out)
	if out["message"] != "Invalid request." || out["status"] != false {
		t.Errorf("non-image: %v", out)
	}
	if c2.gotPatch.URL != nil {
		t.Errorf("non-image should not patch: %+v", c2.gotPatch)
	}
}

func bytesContains(s, sub string) bool { return bytes.Contains([]byte(s), []byte(sub)) }

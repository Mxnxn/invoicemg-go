package wastage

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type stub struct {
	list []store.Wastage
	got  store.ID
}

func (s *stub) List(_ context.Context, c store.ID) ([]store.Wastage, error) {
	s.got = c
	return s.list, nil
}

func TestGetAll(t *testing.T) {
	s := &stub{list: []store.Wastage{{ID: "w1", CompanyID: "co1", MaterialName: "Vinyl", Total: 2, Rate: 45}}}
	r := httptest.NewRequest("POST", "/", nil)
	r = r.WithContext(auth.WithSession(r.Context(), store.Session{CompanyID: "co1"}))
	rec := httptest.NewRecorder()
	New(s).GetAll(rec, r)
	var b map[string]any
	json.Unmarshal(rec.Body.Bytes(), &b)
	if s.got != "co1" || b["code"] != float64(200) {
		t.Fatalf("scope/code wrong: %v %v", s.got, b["code"])
	}
	row := b["data"].([]any)[0].(map[string]any)
	if row["material_name"] != "Vinyl" || row["rate"] != float64(45) {
		t.Errorf("row wrong: %v", row)
	}
}

package httpx

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// jsonForm builds a *Form from an application/json body, the way axios posts a plain object.
func jsonForm(t *testing.T, body string) *Form {
	t.Helper()
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	f, err := ReadForm(req)
	if err != nil {
		t.Fatalf("ReadForm: %v", err)
	}
	return f
}

// A JSON body must feed the scalar accessors just like a form body: a string comes back
// unquoted, a number and a boolean render as the text Node's String()/Number() would produce.
func TestReadForm_JSONScalars(t *testing.T) {
	f := jsonForm(t, `{"client_id":"abc123","qty":5,"rate":2.5,"flag":true,"keep":"false"}`)

	if got := f.String("client_id"); got != "abc123" {
		t.Errorf("String(client_id)=%q want abc123", got)
	}
	if got := f.Int("qty", 0); got != 5 {
		t.Errorf("Int(qty)=%d want 5", got)
	}
	if got := f.Float("rate", 0); got != 2.5 {
		t.Errorf("Float(rate)=%v want 2.5", got)
	}
	if !f.Bool("flag") {
		t.Errorf("Bool(flag) want true")
	}
	if f.NotFalse("keep") {
		t.Errorf(`NotFalse(keep) want false for "false"`)
	}
	if !f.Has("client_id") {
		t.Errorf("Has(client_id) want true")
	}
	if f.Has("missing") {
		t.Errorf("Has(missing) want false")
	}
}

// JSON null is falsy in `if (!req.body.x)`, so it must read as absent, not as the string "null".
func TestReadForm_JSONNullIsMissing(t *testing.T) {
	f := jsonForm(t, `{"client_id":null}`)
	if f.Has("client_id") {
		t.Errorf("Has(client_id) want false for JSON null")
	}
	if got := f.String("client_id"); got != "" {
		t.Errorf("String(client_id)=%q want empty for JSON null", got)
	}
}

// An empty object and a bare (empty) body are both "every field missing" with no error - the
// same as the /unit/list call that posts {} today.
func TestReadForm_JSONEmpty(t *testing.T) {
	for _, body := range []string{`{}`, ``, `   `} {
		f := jsonForm(t, body)
		if f.Has("anything") {
			t.Errorf("body %q: Has(anything) want false", body)
		}
		if got := f.String("anything"); got != "" {
			t.Errorf("body %q: String=%q want empty", body, got)
		}
	}
}

// Form.JSON works on an already-parsed JSON body too, not only on a JSON string inside a form
// field - a route that reads `rows` must behave the same whichever encoding reached it.
func TestReadForm_JSONArrayField(t *testing.T) {
	f := jsonForm(t, `{"rows":[{"n":1},{"n":2},{"n":3}]}`)
	var rows []row
	if err := f.JSON("rows", &rows); err != nil {
		t.Fatalf("JSON(rows): %v", err)
	}
	if len(rows) != 3 || rows[2].N != 3 {
		t.Fatalf("got %+v", rows)
	}

	// Absent array field: strict JSON errors, lenient JSONOr falls back - same as the form path.
	f = jsonForm(t, `{}`)
	if err := f.JSON("rows", &rows); err == nil {
		t.Errorf("JSON(rows) on absent field want error")
	}
	var alloc []int
	if err := f.JSONOr("allocations", "[]", &alloc); err != nil {
		t.Errorf("JSONOr(allocations) on absent field want no error, got %v", err)
	}
	if len(alloc) != 0 {
		t.Errorf("JSONOr fallback want empty, got %v", alloc)
	}
}

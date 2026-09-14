package httpx

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// Form reads a request body the way multer's upload.none() does.
//
// Every route in the Node app is a POST, and the React client sends FormData for most of them -
// even for reads. So a Go handler that only understood JSON would answer 422 to every real
// request from the browser while looking perfectly correct in a curl test written with -d.
//
// Three encodings are accepted:
//   - multipart/form-data - the FormData the browser posts for most routes;
//   - application/x-www-form-urlencoded - what some maintenance scripts post;
//   - application/json - what axios posts whenever a view hands it a plain object rather than a
//     FormData, e.g. `axios.post("/unit/list", {})` or `.post("/batch-receive/lookups/open-jobs",
//     {client_id})`. `/unit/list` is already live here and only passes parity because it ignores
//     its body; a JSON-body route that actually reads a field would find nothing without this.
//
// A JSON body is decoded once into per-field raw messages, so scalar reads (String/Int/Float)
// and JSON() both work regardless of which encoding a given caller used.
type Form struct {
	r        *http.Request
	jsonBody map[string]json.RawMessage
}

// maxMemory is what stays in RAM before multipart spills to a temp file. 8MB is well above
// any field this API takes; the two upload routes that accept real files set their own.
const maxMemory = 8 << 20

// ReadForm never returns a nil Form, even when parsing fails.
//
// A DELETE carries no body at all, so parsing it "fails" in the ordinary course of events -
// and a handler that reads the id from the path and only falls back to the form would
// nil-pointer panic on the error path. r.FormValue is safe to call on an unparsed request
// (it returns ""), so an empty Form is a perfectly good answer; the error is there for the
// callers that genuinely require a body.
func ReadForm(r *http.Request) (*Form, error) {
	form := &Form{r: r}

	contentType := r.Header.Get("Content-Type")
	switch {
	case strings.HasPrefix(contentType, "multipart/form-data"):
		if err := r.ParseMultipartForm(maxMemory); err != nil {
			return form, err
		}
	case strings.HasPrefix(contentType, "application/json"):
		// LimitReader caps the read the way body-parser's 100kb-ish default caps Node; #25 will
		// formalise the exact limit and the overflow response. An empty body (a bare POST with a
		// JSON content-type but nothing sent) leaves jsonBody nil, which reads as every field
		// missing - the same as an unparsed form.
		body, err := io.ReadAll(io.LimitReader(r.Body, maxMemory))
		if err != nil {
			return form, err
		}
		if len(bytes.TrimSpace(body)) == 0 {
			return form, nil
		}
		if err := json.Unmarshal(body, &form.jsonBody); err != nil {
			return form, err
		}
	default:
		if err := r.ParseForm(); err != nil {
			return form, err
		}
	}
	return form, nil
}

// String returns a field, trimmed. Missing and empty are the same thing here, which matches
// how the Node routes check them: `if (!req.body.client_id)`.
func (f *Form) String(name string) string {
	return strings.TrimSpace(f.field(name))
}

// field is the untrimmed scalar value of a form field, from whichever encoding the request
// used. For multipart and urlencoded it is r.FormValue; for a JSON body it is the field coerced
// to the string Node's `String(req.body.x)` would produce - a JSON string unquoted, a number or
// boolean rendered as text, and a JSON null read as absent (falsy), so `if (!x)` still holds.
func (f *Form) field(name string) string {
	if f.jsonBody == nil {
		return f.r.FormValue(name)
	}
	msg, ok := f.jsonBody[name]
	if !ok {
		return ""
	}
	s := string(msg)
	if s == "null" {
		return ""
	}
	if len(s) > 0 && s[0] == '"' {
		var str string
		if json.Unmarshal(msg, &str) == nil {
			return str
		}
	}
	return s
}

// Has reports whether a field arrived with a non-empty value.
func (f *Form) Has(name string) bool { return f.String(name) != "" }

// Int returns a field as a whole number, with a fallback for missing or unparseable input.
//
// Unparseable and missing deliberately give the same answer. `Number(undefined) || 0` and
// `Number("abc") || 0` are both 0 in JavaScript, and a Go handler that 422'd on a stray
// character would refuse a request Node accepts.
func (f *Form) Int(name string, fallback int) int {
	v, err := strconv.Atoi(f.String(name))
	if err != nil {
		return fallback
	}
	return v
}

// Float is Int's counterpart for money and quantities.
func (f *Form) Float(name string, fallback float64) float64 {
	v, err := strconv.ParseFloat(f.String(name), 64)
	if err != nil {
		return fallback
	}
	return v
}

// Bool follows JavaScript's truthiness for the values a form can actually carry.
//
// A checkbox posts "true"/"false" as TEXT, and in JavaScript the string "false" is truthy - so
// `if (req.body.flag)` is true for both. The Node routes work around this by comparing
// explicitly (`req.body.x !== "false"`), and this reproduces that comparison rather than
// Go's strconv.ParseBool, which would read "false" as false and change behaviour.
func (f *Form) Bool(name string) bool {
	v := strings.ToLower(f.String(name))
	return v == "true" || v == "1" || v == "yes" || v == "on"
}

// NotFalse is the other half of that quirk: fields defaulting to true unless explicitly
// switched off, spelled `x !== false && x !== "false"` in the Node routes.
func (f *Form) NotFalse(name string) bool {
	v := strings.ToLower(f.String(name))
	return v != "false" && v != "0"
}

// Missing returns the names of required fields that did not arrive, so a handler can answer
// the same 422 the Node route answers without a stack of if-statements.
func (f *Form) Missing(names ...string) []string {
	var missing []string
	for _, name := range names {
		if !f.Has(name) {
			missing = append(missing, name)
		}
	}
	return missing
}

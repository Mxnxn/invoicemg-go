package httpx

import "encoding/json"

// JSON-encoded fields inside multipart form values.
//
// A dozen Node routes carry structured input as a JSON string inside a single form field
// rather than as separate fields - Lifecycle's `rows` and `queueOrder`, Invoice's `entry_ids`,
// BatchReceive/Invoice/SupplierPayment's `allocations`, Person's `permissions`. The frontend
// posts FormData, so these arrive as one string that the Node handler runs through
// `JSON.parse`. A Go handler that read them as plain strings would silently ignore every row.
//
// The two spellings in the Node source both matter, and they behave differently on a missing
// or empty field:
//
//	JSON.parse(req.body.rows)              // strict  - throws on absent/empty/malformed
//	JSON.parse(req.body.allocations || "[]")  // lenient - absent/empty means the fallback
//
// so there are two helpers, and each is faithful to one of them. Both read the RAW form value,
// never the trimmed String(): `JSON.parse("  ")` throws in JavaScript because "  " is a truthy
// string, and trimming it to "" first would turn a Node error into a Go success.

// JSON parses a JSON-encoded field into dst, mirroring `JSON.parse(req.body.name)` with no
// fallback. An absent or empty field is an error, exactly as `JSON.parse(undefined)` and
// `JSON.parse("")` both throw; a malformed field is an error too. The handler turns that error
// into whichever envelope the matching Node route's catch block sends - the decision is the
// handler's, as with store.ErrBadID.
func (f *Form) JSON(name string, dst any) error {
	return json.Unmarshal(f.fieldJSON(name), dst)
}

// JSONOr parses a JSON-encoded field into dst, mirroring `JSON.parse(req.body.name || fallback)`.
// An absent or empty field falls back to `fallback` (almost always "[]"), so it is not an
// error; a non-empty but malformed field still errors, because in JavaScript a non-empty
// string is truthy and reaches JSON.parse, which throws.
func (f *Form) JSONOr(name, fallback string, dst any) error {
	raw := f.fieldJSON(name)
	if len(raw) == 0 {
		raw = []byte(fallback)
	}
	return json.Unmarshal(raw, dst)
}

// fieldJSON is the raw JSON bytes of a field, the direct equivalent of `req.body.name` before
// JSON.parse. For multipart and urlencoded that is the untrimmed form value (the field carries
// a JSON string the browser put there); for a JSON body it is the field's own raw message, so
// JSON()/JSONOr() work on an already-parsed body too - an absent field gives empty bytes, which
// JSON() errors on (strict) and JSONOr() replaces with the fallback.
//
// The value is deliberately NOT trimmed: `JSON.parse("  ")` throws in JavaScript because "  "
// is truthy, so the fallback fires only on a genuinely empty value (len 0), never on whitespace,
// which must reach the parser and error the way Node's does.
func (f *Form) fieldJSON(name string) []byte {
	if f.jsonBody != nil {
		return []byte(f.jsonBody[name])
	}
	return []byte(f.r.FormValue(name))
}

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
	return json.Unmarshal([]byte(f.raw(name)), dst)
}

// JSONOr parses a JSON-encoded field into dst, mirroring `JSON.parse(req.body.name || fallback)`.
// An absent or empty field falls back to `fallback` (almost always "[]"), so it is not an
// error; a non-empty but malformed field still errors, because in JavaScript a non-empty
// string is truthy and reaches JSON.parse, which throws.
func (f *Form) JSONOr(name, fallback string, dst any) error {
	raw := f.raw(name)
	if raw == "" {
		raw = fallback
	}
	return json.Unmarshal([]byte(raw), dst)
}

// raw is the untrimmed form value, the direct equivalent of `req.body.name`. String() trims
// for the `if (!req.body.x)` checks the rest of the handlers do; JSON parsing must not, or the
// whitespace-only case diverges from Node.
func (f *Form) raw(name string) string {
	return f.r.FormValue(name)
}

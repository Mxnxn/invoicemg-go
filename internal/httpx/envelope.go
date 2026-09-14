// Package httpx holds the wire contract, in both the shapes this service has to speak.
//
// The Node API answers every request with HTTP 200 and puts the real outcome in a `code`
// inside the body. The React client branches on `data.code` and never on the HTTP status, so
// while the two services run side by side a Go handler that answered a real 401 would be read
// by the frontend as a SUCCESSFUL request carrying no data - a blank screen with no error.
//
// That is preserved as StyleLegacy, and StyleREST puts the outcome back in the status line
// where it belongs. One handler serves both; see style.go.
package httpx

import (
	"encoding/json"
	"log"
	"net/http"
)

// Envelope is the response shape every route returns.
//
// The Node routes are not consistent about `status`, and parity matters more than tidiness:
// /sheet/only answers {code,data,message} with no `status`, while /sheet/open-jobs answers
// {code,message,status,data}. Reproducing each route's exact shape is the point - a client that
// checks `status` on a route that never sent one gets `undefined`, which is falsy, which reads
// as failure.
//
// Data needs THREE distinct wire outcomes, which a plain `json:",omitempty"` cannot express -
// see MarshalJSON:
//
//   - absent    - the key is not sent at all. Every error envelope, and any route that sends
//     no data. This is Data left unset (a nil interface).
//   - null      - the key is sent with the value null. /settings/* send `data: row?…:null` and
//     lifecycle sends `data: job||null`. This is Data set to the Null sentinel.
//   - a value   - including `[]` for an empty list. omitempty is the trap here: it drops an
//     empty slice, so a list route that carefully built a non-nil `[]T{}` (as /sheet/open-jobs
//     does) would still omit the key and answer a different shape than Node's `data: []`.
type Envelope struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
	Status  *bool  `json:"status,omitempty"`
	// Data's presence and shape are governed by MarshalJSON, not by this tag - the tag is kept
	// only so the field reads as part of the wire shape at a glance.
	Data any `json:"data,omitempty"`
}

// nullData is the one value whose presence in Data means "send data: null" rather than "omit
// data". A typed marker is the only way to tell an unset Data (nil interface) apart from a
// deliberate null, since both would otherwise be a nil `any`.
type nullData struct{}

func (nullData) MarshalJSON() ([]byte, error) { return []byte("null"), nil }

// Null is what a handler puts in Envelope.Data to send an explicit `data: null` - the shape
// /settings/* and the lifecycle job||null routes answer. Leaving Data unset omits the key
// instead; the two are different bytes and the parity harness treats them as different.
var Null any = nullData{}

// MarshalJSON emits code/message/status the ordinary way and then decides data's presence by
// hand: a nil interface omits the key, anything else is marshalled as-is (so the Null sentinel
// becomes null, a non-nil empty slice becomes [], a struct becomes an object). A nil *typed*
// slice still marshals to null - handlers that mean [] must hand in a non-nil slice, which the
// list routes already do with make(T, 0, n).
func (e Envelope) MarshalJSON() ([]byte, error) {
	// A distinct type so this does not recurse back into MarshalJSON. code/message/status keep
	// their existing tags, including the omitempty on message and status.
	type head struct {
		Code    int    `json:"code"`
		Message string `json:"message,omitempty"`
		Status  *bool  `json:"status,omitempty"`
	}
	out, err := json.Marshal(head{Code: e.Code, Message: e.Message, Status: e.Status})
	if err != nil {
		return nil, err
	}
	if e.Data == nil {
		return out, nil // key absent
	}
	data, err := json.Marshal(e.Data)
	if err != nil {
		return nil, err
	}
	// Splice "data" in before the closing brace. head always has at least "code", so out is
	// never the empty object "{}" and the comma is always correct.
	spliced := make([]byte, 0, len(out)+len(data)+len(`,"data":`))
	spliced = append(spliced, out[:len(out)-1]...)
	spliced = append(spliced, `,"data":`...)
	spliced = append(spliced, data...)
	spliced = append(spliced, '}')
	return spliced, nil
}

// True is a helper for the routes that do send `status`.
func True() *bool { b := true; return &b }

// False is a helper for the routes that do send `status`.
func False() *bool { b := false; return &b }

// Write sends an envelope, with the HTTP status the configured style calls for.
//
// In legacy style that is always 200 and the outcome lives in the body; in rest style the
// status carries the outcome and the body keeps `code` as well, so a client can be moved
// across one call at a time rather than all at once. See style.go for why both exist.
func Write(w http.ResponseWriter, env Envelope) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	status := http.StatusOK
	if Style() == StyleREST {
		status = httpStatusFor(env.Code)
	}
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(env); err != nil {
		// The status line is already sent, so there is nothing to tell the client. Log it:
		// a serialisation failure here means a handler built something json cannot encode.
		log.Printf("httpx: encoding response: %v", err)
	}
}

// OK is the success envelope the Node routes send, word for word. The message is not
// decoration - some callers show it - so it matches "Operation successful." exactly.
func OK(w http.ResponseWriter, data any) {
	Write(w, Envelope{Code: 200, Message: "Operation successful.", Status: True(), Data: data})
}

// Unauthorized matches Helpers/TokenHelper.js.
func Unauthorized(w http.ResponseWriter, message string) {
	if message == "" {
		message = "Unauthorized."
	}
	Write(w, Envelope{Code: 401, Message: message, Status: False()})
}

// Forbidden matches Helpers/RoleHelper.js.
func Forbidden(w http.ResponseWriter) {
	Write(w, Envelope{Code: 403, Message: "Forbidden.", Status: False()})
}

// Invalid matches the 422 the Node routes send for a missing or unusable field.
func Invalid(w http.ResponseWriter, message string) {
	if message == "" {
		message = "Invalid request."
	}
	Write(w, Envelope{Code: 422, Message: message, Status: False()})
}

// Internal matches the 500 envelope. The underlying error is logged, never sent: the Node
// routes answer a bare "Internal Error" and leaking a driver message to a browser tells an
// attacker about the schema.
func Internal(w http.ResponseWriter, err error) {
	if err != nil {
		log.Printf("httpx: internal error: %v", err)
	}
	Write(w, Envelope{Code: 500, Message: "Internal Error", Status: False()})
}

// LogSwallowed records an error a handler deliberately ignores.
//
// Several Node routes swallow a failure on purpose - /unit/list seeds defaults and continues
// if the seed fails, because the company keeps whatever units it has and a broken seed must
// not break the list. Reproducing that behaviour silently would make the Go service harder to
// debug than the one it replaces, so the decision is kept and the evidence is not.
func LogSwallowed(what string, err error) {
	if err != nil {
		log.Printf("httpx: continuing after a failure in %s: %v", what, err)
	}
}

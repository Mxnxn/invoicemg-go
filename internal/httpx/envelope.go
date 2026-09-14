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
// Data is `any` and omitted when nil, because the Node routes are not consistent about it and
// parity matters more than tidiness: /sheet/only answers {code,data,message} with no `status`,
// while /sheet/open-jobs answers {code,message,status,data}. Reproducing each route's exact
// shape is the point - a client that checks `status` on a route that never sent one gets
// `undefined`, which is falsy, which reads as failure.
type Envelope struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
	Status  *bool  `json:"status,omitempty"`
	Data    any    `json:"data,omitempty"`
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

// Package httpx holds the wire contract this service shares with the Node API.
//
// That contract is unusual and deliberately preserved: business errors come back as HTTP 200
// with a `code` inside the body. The React client branches on `data.code` and never on the
// HTTP status (its global axios interceptor raises a toast for any code != 200), so a Go
// handler that answered a real 401 would be read by the frontend as a SUCCESSFUL request
// carrying no data - a blank screen with no error. Everything here exists to make that
// impossible to get wrong by accident.
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

// Write sends an envelope. Always HTTP 200 - see the package comment.
func Write(w http.ResponseWriter, env Envelope) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
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

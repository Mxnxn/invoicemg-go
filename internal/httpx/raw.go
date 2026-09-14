package httpx

import (
	"log"
	"net/http"
)

// Escape hatches from the envelope, for the handful of Node routes that do not answer with the
// {code,message,status} body - and, since scripts/parity.js compares the HTTP status line as
// well as the body, for the ones that answer a real HTTP status where the rest of the app
// answers 200. All three ignore the API style on purpose: these routes carry real HTTP
// semantics that a caller (Meta, a browser) depends on, so their status is never rewritten.

// Raw sends a non-envelope body with an explicit status and content type:
//
//   - GET /whatsapp/webhook echoes hub.challenge back as a bare string with HTTP 200. Express's
//     res.send(string) defaults to text/html, so a faithful caller passes that content type.
//   - GET /docs serves an HTML page via res.type("html").send(...).
func Raw(w http.ResponseWriter, status int, contentType string, body []byte) {
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		log.Printf("httpx: writing raw response: %v", err)
	}
}

// Status is Express res.sendStatus(code): the status code with its standard message as a plain
// text body ("OK", "Forbidden", "Internal Server Error"). POST /whatsapp/webhook acknowledges
// with 200 and rejects with 403/500 this way, and Meta reads the status line, not a body.
func Status(w http.ResponseWriter, code int) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(code)
	if _, err := w.Write([]byte(http.StatusText(code))); err != nil {
		log.Printf("httpx: writing status response: %v", err)
	}
}

// WriteStatus sends the ordinary envelope body but forces the HTTP status line. A few Node
// routes answer a real HTTP status even though the app otherwise answers 200 - Statistics does
// res.status(401).json({code:401,...}). A legacy-style Write would answer 200 there and diverge,
// because parity.js compares the status; this pins it. The body still goes through the envelope
// marshaller, so data null/[]/absent behave exactly as in Write.
func WriteStatus(w http.ResponseWriter, httpStatus int, env Envelope) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(httpStatus)
	writeBody(w, env)
}

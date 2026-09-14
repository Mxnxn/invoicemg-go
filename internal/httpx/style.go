package httpx

// Two API styles, because this service answers to two different worlds.
//
// LEGACY is what the Node API does and what the React client currently expects: every response
// is HTTP 200, and the real outcome is a `code` inside the body. It is wrong as HTTP - a
// failed request that says 200 is invisible to every proxy, load balancer, log aggregator,
// retry policy and browser devtools panel between here and the user - but it is what the
// client reads. While the two services run side by side, a Go route that answered a real 401
// would be read by the client as a SUCCESSFUL request that returned no data: a blank screen
// with no error at all.
//
// REST is the correct surface: the HTTP status carries the outcome, and the body still carries
// `code` so the same handler serves both and nothing has to be written twice.
//
// The style is chosen once at startup, by API_STYLE, and never changes afterwards. The
// sideways stack runs legacy because parity is the whole point of it; the all-in-one local
// stack runs rest, because there the frontend is ours to move.
const (
	StyleLegacy = "legacy"
	StyleREST   = "rest"
)

// style is package-level because every handler calls httpx.Write directly, and threading a
// writer through all of them to carry one boot-time constant would be ceremony for no gain.
// Written once, before the server starts listening, and only read afterwards - so there is no
// race to guard against.
var style = StyleLegacy

// SetStyle is called once from main, before ListenAndServe.
func SetStyle(s string) {
	if s == StyleREST {
		style = StyleREST
		return
	}
	style = StyleLegacy
}

func Style() string { return style }

// httpStatusFor maps an envelope code onto the HTTP status it should have had all along.
//
// The codes in this application are already HTTP status codes - 401, 403, 404, 409, 422, 500 -
// which is the clearest sign that the envelope was always an HTTP status wearing a disguise.
// Anything unrecognised becomes 500 rather than being passed through, so a typo in a handler
// cannot invent a status.
func httpStatusFor(code int) int {
	switch code {
	case 200, 201, 202, 204, 400, 401, 403, 404, 409, 410, 422, 429, 500, 502, 503:
		return code
	default:
		return 500
	}
}

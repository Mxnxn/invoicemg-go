package client

import (
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

// NotifyPreference is POST /client/notify-preference: the remembered answer to "tell this
// customer when a job-id is raised (or updated)?". The FIELD is chosen by `kind` ("updated" ->
// notifyOnUpdate, anything else -> notifyOnCreate); the VALUE comes from the notifyOnCreate form
// field when present, else `value`. "clear" stores null ("ask each time"), "true" stores true,
// anything else stores false - Node does not validate the value, and neither do we. Company-
// scoped; a missing id is 422, a miss is 404. Returns BOTH flags.
func (h *Handler) NotifyPreference(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)
	form, _ := httpx.ReadForm(r)

	clientID := form.String("client_id")
	if clientID == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	field := "notifyOnCreate"
	if form.String("kind") == "updated" {
		field = "notifyOnUpdate"
	}
	// raw = req.body.notifyOnCreate !== undefined ? req.body.notifyOnCreate : req.body.value
	raw := form.String("value")
	if form.Present("notifyOnCreate") {
		raw = form.String("notifyOnCreate")
	}
	var value *bool
	if raw != "clear" {
		b := raw == "true"
		value = &b
	}

	onCreate, onUpdate, found, err := h.store.SetNotifyPreference(ctx, sess.CompanyID, store.ID(clientID), field, value)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Customer not found.", Status: httpx.False()})
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Status: httpx.True(), Data: notifyPair{
		NotifyOnCreate: onCreate, NotifyOnUpdate: onUpdate,
	}})
}

// NotifyPreferences is GET /client/notify-preferences: the customers and their answers, firm-
// sorted. `?all=true` returns every customer (the tab for CHOOSING an answer); otherwise only
// those who have answered either flag (the tab for REVIEWING answers).
func (h *Handler) NotifyPreferences(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := auth.MustFrom(ctx)

	answeredOnly := r.URL.Query().Get("all") != "true"
	list, err := h.store.NotifyPreferences(ctx, sess.CompanyID, answeredOnly)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out := make([]notifyRow, 0, len(list))
	for _, c := range list {
		out = append(out, notifyRow{
			ID: string(c.ID), ClientName: c.ClientName, ClientFirm: c.ClientFirm, ClientPhone: c.ClientPhone,
			NotifyOnCreate: c.NotifyOnCreate, NotifyOnUpdate: c.NotifyOnUpdate,
		})
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Status: httpx.True(), Data: out})
}

// notifyPair is the {notifyOnCreate, notifyOnUpdate} body /notify-preference returns. omitempty on
// a *bool omits a nil flag (unanswered/cleared, indistinguishable here) and keeps &false, so the
// wire matches Node's `{notifyOnCreate: client.notifyOnCreate, ...}` for every value the client
// can act on.
type notifyPair struct {
	NotifyOnCreate *bool `json:"notifyOnCreate,omitempty"`
	NotifyOnUpdate *bool `json:"notifyOnUpdate,omitempty"`
}

// notifyRow is the /notify-preferences projection: the customer plus both flags.
type notifyRow struct {
	ID             string `json:"_id"`
	ClientName     string `json:"clientName"`
	ClientFirm     string `json:"clientFirm"`
	ClientPhone    string `json:"clientPhone"`
	NotifyOnCreate *bool  `json:"notifyOnCreate,omitempty"`
	NotifyOnUpdate *bool  `json:"notifyOnUpdate,omitempty"`
}

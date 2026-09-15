// Package sheet serves POST /sheet/get from routes/Sheet.js: one day-sheet's entries grouped
// into a card per client. /sheet/only is served by the day handler; /sheet/update, /remove and
// /getall are empty stubs in Node and are intentionally not ported.
package sheet

import (
	"encoding/json"
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/entry"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type Handler struct{ store store.Sheets }

func New(s store.Sheets) *Handler { return &Handler{store: s} }

// Get is POST /sheet/get (admin): the sheet's entries collapsed to one card per client, each
// card carrying that client's entries. The response adds a top-level `date` beside `data`.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	sid := form.String("sid")
	if sid == "" {
		httpx.Invalid(w, "")
		return
	}
	date, entries, found, err := h.store.Get(r.Context(), sess.CompanyID, store.ID(sid))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Sheet not found.", Status: httpx.False()})
		return
	}

	// One card per client, in first-seen order; each card keeps that client's entries.
	type card struct {
		ID         string           `json:"_id"`
		UID        string           `json:"uid"`
		ClientName string           `json:"clientName"`
		ClientFirm string           `json:"clientFirm"`
		Entries    []map[string]any `json:"entries"`
	}
	cards := make([]*card, 0)
	byClient := map[string]*card{}
	for _, e := range entries {
		if e.Client == nil {
			continue // a populated client that no longer resolves is dropped, as Node drops it
		}
		cid := string(e.Client.ID)
		c := byClient[cid]
		if c == nil {
			c = &card{
				ID: cid, UID: string(e.UID),
				ClientName: e.Client.ClientName, ClientFirm: e.Client.ClientFirm,
				Entries: []map[string]any{},
			}
			byClient[cid] = c
			cards = append(cards, c)
		}
		c.Entries = append(c.Entries, entry.EntryJSON(e))
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code": 200, "data": cards, "date": date, "message": "Operation successful.",
	})
}

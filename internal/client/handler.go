// Package client serves the customer routes from routes/Client.js. Only the two list reads are
// ported so far - /client/getall and /client/only - both of which WIDEN by sharing (#1): they
// return the acting company's clients, legacy null-company rows, and clients shared with it. The
// writes (add/update/remove) are not ported, and when they are they must use company-only scope
// and never this widened read.
package client

import (
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type Handler struct{ store store.Clients }

func New(s store.Clients) *Handler { return &Handler{store: s} }

// Getall is POST /client/getall: the full customer list for the picker, each row flagged
// `borrowed` when it is shared in from another of the owner's companies. Note the message is
// "Operation successful" with NO trailing period, and there is no `status` field - both exactly
// as routes/Client.js sends them.
func (h *Handler) Getall(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	list, err := h.store.Visible(r.Context(), sess.UID, sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out := make([]getallDTO, 0, len(list))
	for _, c := range list {
		out = append(out, getallDTO{
			ID:            string(c.ID),
			UID:           string(c.UID),
			CompanyID:     idPtr(c.CompanyID),
			ClientName:    c.ClientName,
			ClientFirm:    c.ClientFirm,
			ClientPhone:   c.ClientPhone,
			ClientGST:     c.ClientGST,
			ClientAddress: c.ClientAddress,
			Sharing:       sharingOf(c.Sharing),
			Borrowed:      borrowed(c, sess.CompanyID),
		})
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful", Data: out})
}

// Only is POST /client/only: a lighter list (no sharing, no borrowed flag) that adds
// openingBalance. Same widened scope, same message quirk, same absence of `status`.
func (h *Handler) Only(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	list, err := h.store.Visible(r.Context(), sess.UID, sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out := make([]onlyDTO, 0, len(list))
	for _, c := range list {
		out = append(out, onlyDTO{
			ID:             string(c.ID),
			UID:            string(c.UID),
			ClientName:     c.ClientName,
			ClientFirm:     c.ClientFirm,
			ClientPhone:    c.ClientPhone,
			ClientGST:      c.ClientGST,
			ClientAddress:  c.ClientAddress,
			OpeningBalance: c.OpeningBalance,
		})
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful", Data: out})
}

// borrowed is Helpers/SharedRecords.js isBorrowed: visible here but owned elsewhere - the
// record's company is not the acting one, yet it is shared with the acting one.
func borrowed(c store.Client, companyID store.ID) bool {
	if c.CompanyID == companyID || c.Sharing == nil {
		return false
	}
	for _, id := range *c.Sharing {
		if id == companyID {
			return true
		}
	}
	return false
}

// idPtr renders an id as a JSON string, or null for the empty id - a legacy client whose
// company_id is null must send company_id:null, not "".
func idPtr(id store.ID) *string {
	if id == "" {
		return nil
	}
	s := string(id)
	return &s
}

// sharingOf preserves the nil-vs-empty distinction: a legacy record with no sharing field omits
// the key (nil pointer), a stamped record sends {companies:[...]} even when the list is empty.
func sharingOf(ids *[]store.ID) *sharingDTO {
	if ids == nil {
		return nil
	}
	out := make([]string, 0, len(*ids))
	for _, id := range *ids {
		out = append(out, string(id))
	}
	return &sharingDTO{Companies: out}
}

type getallDTO struct {
	ID            string      `json:"_id"`
	UID           string      `json:"uid"`
	CompanyID     *string     `json:"company_id"`
	ClientName    string      `json:"clientName"`
	ClientFirm    string      `json:"clientFirm"`
	ClientPhone   string      `json:"clientPhone"`
	ClientGST     string      `json:"clientGST"`
	ClientAddress string      `json:"clientAddress"`
	Sharing       *sharingDTO `json:"sharing,omitempty"`
	Borrowed      bool        `json:"borrowed"`
}

type sharingDTO struct {
	Companies []string `json:"companies"`
}

type onlyDTO struct {
	ID             string  `json:"_id"`
	UID            string  `json:"uid"`
	ClientName     string  `json:"clientName"`
	ClientFirm     string  `json:"clientFirm"`
	ClientPhone    string  `json:"clientPhone"`
	ClientGST      string  `json:"clientGST"`
	ClientAddress  string  `json:"clientAddress"`
	OpeningBalance float64 `json:"openingBalance"`
}

// Package statistics serves the dashboard reads from routes/Statistics.js - /stats/get (square
// feet produced per material for a month, by paid/unpaid, optionally one client) and
// /stats/clients. Behind the dashboard feature. XLSX download/exports are not ported.
package statistics

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type Handler struct{ store store.Statistics }

func New(s store.Statistics) *Handler { return &Handler{store: s} }

var months = []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

// Clients is GET /stats/clients - {code, data, status}, no message.
func (h *Handler) Clients(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	list, err := h.store.Clients(r.Context(), sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, c := range list {
		out = append(out, map[string]any{"_id": string(c.ID), "clientName": c.ClientName, "clientFirm": c.ClientFirm})
	}
	httpx.WriteStatus(w, 200, httpx.Envelope{Code: 200, Status: httpx.True(), Data: out})
}

// Get is POST /stats/get - square feet produced per material, in the requested month+year.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	dateStr := form.String("date")
	flag := form.String("flag")
	if dateStr == "" || flag == "" {
		// Node answers a real 401 here.
		httpx.WriteStatus(w, 401, httpx.Envelope{Code: 401, Message: "Invalid Request.", Status: httpx.False()})
		return
	}
	reqDate, ok := parseJSDate(dateStr)
	if !ok {
		httpx.WriteStatus(w, 401, httpx.Envelope{Code: 401, Message: "Invalid Request.", Status: httpx.False()})
		return
	}
	monthName := months[int(reqDate.Month())-1]
	paid := flag == "PAID"
	clientID := store.ID(form.String("cid"))

	entries, err := h.store.StatEntries(r.Context(), sess.CompanyID, monthName, paid, clientID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	// stats is an ordered-insensitive map: Total plus a key per material.
	stats := map[string]float64{"Total": 0}
	for _, e := range entries {
		ed, ok := parseJSDate(e.Date)
		if !ok || ed.Year() != reqDate.Year() {
			continue
		}
		// square feet: length*width*qty, but only for by-dimension rows (absent flag = true).
		var sqft float64
		if e.HasDimensions == nil || *e.HasDimensions {
			sqft = math.Floor(numStr(e.Length)*numStr(e.Width)*e.Qty*100+0.5) / 100
		}
		stats["Total"] += sqft
		stats[e.Material] += sqft
	}

	data := map[string]any{"stats": stats, "requestedDate": monthName + " " + strconv.Itoa(reqDate.Year())}
	if clientID != "" {
		if c, err := h.store.Client(r.Context(), sess.CompanyID, clientID); err == nil {
			data["customer"] = map[string]any{"_id": string(c.ID), "clientName": c.ClientName, "clientFirm": c.ClientFirm}
		} else {
			data["customer"] = nil
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "data": data, "status": true})
}

func numStr(s string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return f
}

// parseJSDate accepts YYYY-MM-DD and the JS Date toString / ISO forms the client may send.
func parseJSDate(s string) (time.Time, bool) {
	for _, layout := range []string{"2006-01-02", time.RFC3339, "Mon Jan 02 2006", "Jan 02 2006", "2006-01-02T15:04:05.000Z07:00"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

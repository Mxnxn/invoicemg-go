package statistics

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/exportfile"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
	"github.com/mxnxn/invoicemg-go/internal/xlsxexport"
)

// getName is Helpers/GetName: spaces to underscores.
func getName(name string) string {
	out := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		if name[i] == ' ' {
			out = append(out, '_')
		} else {
			out = append(out, name[i])
		}
	}
	return string(out)
}

// sinceForMode maps an export mode to the earliest createdAt to include, matching Node's
// new Date(year, month - N): tm = start of this month, ltm = 3 months back, lsm = 6 back, ft =
// no lower bound. ok is false for an unknown mode.
func sinceForMode(mode string, now time.Time) (time.Time, bool) {
	y, m := now.Year(), int(now.Month())-1 // 0-indexed month, as JS getMonth()
	back := func(n int) time.Time {
		return time.Date(y, time.Month(m-n+1), 1, 0, 0, 0, 0, now.Location())
	}
	switch mode {
	case "tm":
		return back(0), true
	case "ltm":
		return back(3), true
	case "lsm":
		return back(6), true
	case "ft":
		return time.Time{}, true
	default:
		return time.Time{}, false
	}
}

// ddmmyy formats an entry date as DD/MM/YYYY for the sheet. The entry's `date` is a plain string
// (usually YYYY-MM-DD); anything unparseable is passed through unchanged.
func ddmmyy(raw string) string {
	if t, err := time.Parse("2006-01-02", raw[:min(len(raw), 10)]); err == nil {
		return t.Format("02/01/2006")
	}
	return raw
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Exports is GET /stats/exports/{uid}/{mode}: build a client's line-items .xlsx for a time
// window and return the filename. 404 for an unknown client, 204 when the window has no entries,
// 400 for an unknown mode.
func (h *Handler) Exports(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	uid := r.PathValue("uid")
	if uid == "" {
		httpx.WriteStatus(w, http.StatusUnauthorized, httpx.Envelope{Code: 401, Message: "Invalid Request.", Status: httpx.False()})
		return
	}
	client, found, err := h.clients.Get(r.Context(), sess.CompanyID, store.ID(uid))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Client not found.", Status: httpx.False()})
		return
	}
	since, ok := sinceForMode(r.PathValue("mode"), time.Now())
	if !ok {
		httpx.Write(w, httpx.Envelope{Code: 400, Message: "Bad Request!", Status: httpx.False()})
		return
	}

	entries, err := h.entries.SinceForClient(r.Context(), sess.UID, sess.CompanyID, store.ID(uid), since)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if len(entries) == 0 {
		httpx.Write(w, httpx.Envelope{Code: 204, Message: "No Data Found!", Status: httpx.False()})
		return
	}

	rows := make([]xlsxexport.EntryRow, 0, len(entries))
	for _, e := range entries {
		invoiced := "Not Invoiced"
		if e.HasIssued && e.IssuedInvoiceID != "" {
			invoiced = e.IssuedInvoiceID
		}
		rows = append(rows, xlsxexport.EntryRow{
			Date: ddmmyy(e.Date), Material: e.Material, Description: e.Description,
			Length: parseNum(e.Length), Width: parseNum(e.Width), Qty: e.Qty, Rate: e.Rate,
			Amount: e.Amount, Total: e.Amount - e.Advance, Advance: e.Advance, Invoiced: invoiced,
		})
	}

	name := exportfile.Name(string(sess.CompanyID), getName(client.ClientFirm))
	if err := os.MkdirAll(h.exportsDir, 0o755); err != nil {
		httpx.Internal(w, err)
		return
	}
	if err := xlsxexport.WriteClientEntries(filepath.Join(h.exportsDir, name+".xlsx"), rows); err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: name + ".xlsx", Status: httpx.True()})
}

func parseNum(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// Download is GET /stats/download/{fname}: stream a generated export, guarded exactly like the
// invoice download - the company id in the filename authorises it, a bad name 404s.
func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	path := exportfile.ResolvePath(h.exportsDir, r.PathValue("fname"), string(sess.CompanyID))
	if path == "" {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Export not found.", Status: httpx.False()})
		return
	}
	if _, err := os.Stat(path); err != nil {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Export not found.", Status: httpx.False()})
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	http.ServeFile(w, r, path)
}

package invoice

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/entrymath"
	"github.com/mxnxn/invoicemg-go/internal/exportfile"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
	"github.com/mxnxn/invoicemg-go/internal/xlsxexport"
)

// getName is Helpers/GetName: spaces to underscores, for the file label.
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

// Export is GET /invoice/export/{cid}/{uid}: build a client's invoice-ledger .xlsx under the
// exports directory and return its filename. The {uid} path segment is ignored (Node reads only
// cid); the company scope comes from the session. 402 without a cid, 404 for an unknown client.
func (h *Handler) Export(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	cid := r.PathValue("cid")
	if cid == "" {
		httpx.Write(w, httpx.Envelope{Code: 402, Message: "Bad Request"})
		return
	}
	client, found, err := h.clients.Get(r.Context(), sess.CompanyID, store.ID(cid))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Client not found.", Status: httpx.False()})
		return
	}

	list, err := h.invoices.List(r.Context(), sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	rows := make([]xlsxexport.LedgerRow, 0)
	var totalDue float64
	for _, inv := range list {
		if inv.Client == nil || string(inv.Client.ID) != cid {
			continue
		}
		var nonTaxed, entryReceived, taxed float64
		for _, e := range inv.Entries {
			if e.Total == 0 {
				entryReceived += e.Advance * 1.18
			} else {
				entryReceived += e.Advance
			}
			nonTaxed += e.Amount
			taxed += e.Amount * 1.18
		}
		if inv.TotalAmount != 0 {
			totalDue += inv.TotalAmount - inv.Amount
		}
		rows = append(rows, xlsxexport.LedgerRow{
			Date:          inv.Date,
			InvoiceNumber: inv.InvoiceID,
			NonTaxedValue: nonTaxed,
			TaxedValue:    entrymath.RoundOffWithAmount(taxed),
			EntryReceived: entrymath.RoundOffWithAmount(entryReceived),
			Due:           entrymath.RoundOffWithAmount(taxed) - entrymath.RoundOffWithAmount(inv.Amount),
		})
	}

	name := exportfile.Name(string(sess.CompanyID), getName(client.ClientFirm))
	if err := os.MkdirAll(h.exportsDir, 0o755); err != nil {
		httpx.Internal(w, err)
		return
	}
	if err := xlsxexport.WriteClientLedger(filepath.Join(h.exportsDir, name+".xlsx"), client.ClientFirm, rows, entrymath.RoundOffWithAmount(totalDue)); err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: name + ".xlsx", Status: httpx.True()})
}

// Download is GET /invoice/download/{fname}: stream a previously generated export. The filename
// carries the owning company id, so a name that doesn't belong to this session, isn't a plain
// .xlsx, or would escape the directory resolves to nothing and answers 404.
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

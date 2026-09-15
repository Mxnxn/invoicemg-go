// Package ledger serves POST /ledger/client from routes/Ledger.js - a single client's running
// statement (bills from invoices, receipts from invoice_received + batch_receives), balanced
// over an optional [from,to] window. Behind the ledger feature.
//
// Two Node quirks are reproduced exactly: money fields go out as RoundOff strings (JS
// (Math.round(x*100)/100).toFixed(2), which for a negative balance rounds toward +Inf), and
// the opening balance is computed from the transactions before `from` starting at zero - the
// client's stored openingBalance is never consulted.
package ledger

import (
	"math"
	"net/http"
	"sort"
	"strconv"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type Handler struct{ store store.Ledger }

func New(s store.Ledger) *Handler { return &Handler{store: s} }

// roundOff is Node's RoundOff: (Math.round(amt*100)/100).toFixed(2). JS Math.round rounds a
// half toward +Inf (Math.round(-0.5) === -0), so floor(x*100+0.5) matches it for negatives too.
func roundOff(amt float64) string {
	return strconv.FormatFloat(math.Floor(amt*100+0.5)/100, 'f', 2, 64)
}

// billOrNull is Node's `e.bill || null`: the raw number, or null when it is zero.
func billOrNull(v float64) any {
	if v == 0 {
		return nil
	}
	return v
}

func (h *Handler) Client(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)

	clientID := form.String("client_id")
	if clientID == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}

	data, err := h.store.ClientStatement(r.Context(), sess.CompanyID, store.ID(clientID))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !data.Found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Client not found.", Status: httpx.False()})
		return
	}

	// Same-day bills sort ahead of same-day receipts (Seq), so a payment reduces the balance
	// the bill just raised. SliceStable keeps invoices before received before batch on a tie.
	txns := data.Txns
	sort.SliceStable(txns, func(i, j int) bool {
		if txns[i].Date != txns[j].Date {
			return txns[i].Date < txns[j].Date
		}
		return txns[i].Seq < txns[j].Seq
	})

	from := form.String("from")
	to := form.String("to")

	var openingBalance float64
	for _, e := range txns {
		if from != "" && e.Date < from {
			openingBalance += e.Bill - e.Receipt
		}
	}

	balance := openingBalance
	sr := 1
	rows := make([]map[string]any, 0, len(txns)+1)
	if from != "" {
		rows = append(rows, map[string]any{
			"sr": sr, "date": from, "type": "Opening Balance", "invoiceNo": "",
			"bill": nil, "receipt": nil, "balance": roundOff(balance),
		})
		sr++
	}
	for _, e := range txns {
		if from != "" && e.Date < from {
			continue
		}
		if to != "" && e.Date > to {
			continue
		}
		balance += e.Bill - e.Receipt
		rows = append(rows, map[string]any{
			"sr": sr, "date": e.Date, "type": e.Type, "invoiceNo": e.InvoiceNo,
			"bill": billOrNull(e.Bill), "receipt": billOrNull(e.Receipt), "balance": roundOff(balance),
		})
		sr++
	}
	closingBalance := roundOff(balance)

	// Balance across the client's entire history, unbounded by `to`.
	var current float64
	for _, e := range txns {
		current += e.Bill - e.Receipt
	}

	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: map[string]any{
		"clientName":     data.ClientName,
		"clientFirm":     data.ClientFirm,
		"clientGST":      data.ClientGST,
		"clientAddress":  data.ClientAddress,
		"openingBalance": roundOff(openingBalance),
		"closingBalance": closingBalance,
		"currentBalance": roundOff(current),
		"rows":           rows,
	}})
}

// Package purchasereport serves the payables reports from routes/PurchaseReport.js: every
// supplier's outstanding balance (/dues) and one supplier's running ledger (/supplier). Behind
// the purchase_invoices feature. Money goes out as numbers (Node's roundMoney), not RoundOff
// strings - the mirror of routes/Ledger.js on the payables side.
package purchasereport

import (
	"math"
	"net/http"
	"sort"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type Handler struct{ store store.PurchaseReport }

func New(s store.PurchaseReport) *Handler { return &Handler{store: s} }

// roundMoney is Node's Math.round(x*100)/100; floor(x*100+0.5) matches JS rounding for
// negatives too (an overpaid supplier can go below zero).
func roundMoney(v float64) float64 { return math.Floor(v*100+0.5) / 100 }

// numOrNull is Node's `v || null`: the number, or null when zero.
func numOrNull(v float64) any {
	if v == 0 {
		return nil
	}
	return v
}

// Dues is POST /purchase-report/dues: one row per supplier with purchase activity, sorted by
// what is owed descending, plus company totals. A supplier deleted after being invoiced is
// dropped (it can't be labelled), exactly as Node filters.
func (h *Handler) Dues(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	data, err := h.store.SupplierDues(r.Context(), sess.UID, sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}

	type agg struct{ billed, paid float64 }
	byID := map[string]*agg{}
	order := []string{}
	for _, inv := range data.Invoices {
		key := string(inv.SupplierID)
		if key == "" {
			continue
		}
		a := byID[key]
		if a == nil {
			a = &agg{}
			byID[key] = a
			order = append(order, key)
		}
		a.billed += inv.Total
		a.paid += inv.Amount
	}

	type row struct {
		id     string
		billed float64
		paid   float64
		due    float64
		info   store.SupplierInfo
	}
	rows := make([]row, 0, len(order))
	for _, key := range order {
		info, ok := data.Suppliers[key]
		if !ok {
			continue // supplier gone - can't label it
		}
		a := byID[key]
		rows = append(rows, row{
			id: key, billed: roundMoney(a.billed), paid: roundMoney(a.paid),
			due: roundMoney(a.billed - a.paid), info: info,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].due > rows[j].due })

	out := make([]map[string]any, 0, len(rows))
	var sumBilled, sumPaid, sumDue float64
	outstanding := 0
	for _, rw := range rows {
		out = append(out, map[string]any{
			"supplierId": rw.id, "billed": rw.billed, "paid": rw.paid, "due": rw.due,
			"supplierName": rw.info.Name, "supplierFirm": rw.info.Firm, "supplierPhone": rw.info.Phone,
		})
		sumBilled += rw.billed
		sumPaid += rw.paid
		if rw.due > 0 {
			sumDue += rw.due
			outstanding++
		}
	}
	totals := map[string]any{
		"billed": roundMoney(sumBilled), "paid": roundMoney(sumPaid),
		"due": roundMoney(sumDue), "outstandingSuppliers": outstanding,
	}
	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: map[string]any{"rows": out, "totals": totals}})
}

// Supplier is POST /purchase-report/supplier: one supplier's purchase invoices and payments
// merged chronologically into a running balance, over an optional [from,to] window. A same-day
// bill sorts ahead of the payment that settles it (Seq tiebreak).
func (h *Handler) Supplier(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	supplierID := form.String("supplier_id")
	if supplierID == "" {
		httpx.Write(w, httpx.Envelope{Code: 422, Message: "Invalid request.", Status: httpx.False()})
		return
	}
	data, err := h.store.SupplierStatement(r.Context(), sess.UID, sess.CompanyID, store.ID(supplierID))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if !data.Found {
		httpx.Write(w, httpx.Envelope{Code: 404, Message: "Supplier not found.", Status: httpx.False()})
		return
	}

	type entry struct {
		date      string
		typ       string
		reference string
		bill      float64
		payment   float64
		seq       int
	}
	entries := make([]entry, 0, len(data.Invoices)+len(data.Payments))
	for _, inv := range data.Invoices {
		entries = append(entries, entry{date: store.NormalizeDate(inv.Date), typ: "Purchase Invoice", reference: inv.InvoiceNumber, bill: inv.Total, seq: 0})
	}
	for _, pay := range data.Payments {
		entries = append(entries, entry{date: store.NormalizeDate(pay.Date), typ: "Payment", reference: pay.Note, payment: pay.Amount, seq: 1})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].date != entries[j].date {
			return entries[i].date < entries[j].date
		}
		return entries[i].seq < entries[j].seq
	})

	from := form.String("from")
	to := form.String("to")

	var openingBalance float64
	for _, e := range entries {
		if from != "" && e.date < from {
			openingBalance += e.bill - e.payment
		}
	}

	balance := openingBalance
	sr := 1
	rows := make([]map[string]any, 0, len(entries)+1)
	if from != "" {
		rows = append(rows, map[string]any{
			"sr": sr, "date": from, "type": "Opening Balance", "reference": "",
			"bill": nil, "payment": nil, "balance": roundMoney(balance),
		})
		sr++
	}
	for _, e := range entries {
		if from != "" && e.date < from {
			continue
		}
		if to != "" && e.date > to {
			continue
		}
		balance += e.bill - e.payment
		rows = append(rows, map[string]any{
			"sr": sr, "date": e.date, "type": e.typ, "reference": e.reference,
			"bill": numOrNull(e.bill), "payment": numOrNull(e.payment), "balance": roundMoney(balance),
		})
		sr++
	}

	var current float64
	for _, e := range entries {
		current += e.bill - e.payment
	}

	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: map[string]any{
		"supplierName":    data.Supplier.Name,
		"supplierFirm":    data.Supplier.Firm,
		"supplierGST":     data.Supplier.GST,
		"supplierAddress": data.Supplier.Address,
		"openingBalance":  roundMoney(openingBalance),
		"closingBalance":  roundMoney(balance),
		"currentBalance":  roundMoney(current),
		"rows":            rows,
	}})
}

// Package gstreport serves POST /gst-report from routes/GstReport.js - the GST summary of sales
// (invoices) and purchases, bucketed by tax slab, over a date range. Behind the gst_report
// feature. A faithful port of the route's buildRow/slab math.
package gstreport

import (
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"strconv"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

type Handler struct{ store store.Analytics }

func New(s store.Analytics) *Handler { return &Handler{store: s} }

func r2(n float64) float64 { return math.Floor(n*100+0.5) / 100 }

func inRange(date, from, to string) bool {
	if from != "" && date < from {
		return false
	}
	if to != "" && date > to {
		return false
	}
	return true
}

type slabAmt struct{ cgst, sgst, igst, gst float64 }

type totals struct {
	billAmount, finalBill float64
	perSlab               map[float64]*slabAmt
}

func newTotals() *totals { return &totals{perSlab: map[float64]*slabAmt{}} }

// buildRow ports the route's buildRow: sums a document's lines into per-slab GST, and folds the
// same into the running totals; slabs seen are recorded in slabSet.
func buildRow(meta map[string]any, lines []store.GstLine, t *totals, slabSet map[float64]bool) map[string]any {
	perSlab := map[float64]*slabAmt{}
	var billAmount, finalBill float64
	for _, line := range lines {
		amount := line.Amount
		slab := r2(line.Cgst + line.Sgst + line.Igst)
		billAmount += amount
		finalBill += amount * (1 + slab/100)
		if slab <= 0 {
			continue
		}
		slabSet[slab] = true
		cgstAmt := amount * (line.Cgst / 100)
		sgstAmt := amount * (line.Sgst / 100)
		igstAmt := amount * (line.Igst / 100)
		add(perSlab, slab, cgstAmt, sgstAmt, igstAmt)
		add(t.perSlab, slab, cgstAmt, sgstAmt, igstAmt)
	}
	t.billAmount += billAmount
	t.finalBill += finalBill
	meta["billAmount"] = r2(billAmount)
	meta["finalBill"] = r2(finalBill)
	meta["perSlab"] = roundPerSlab(perSlab)
	return meta
}

func add(m map[float64]*slabAmt, slab, cgst, sgst, igst float64) {
	s := m[slab]
	if s == nil {
		s = &slabAmt{}
		m[slab] = s
	}
	s.cgst += cgst
	s.sgst += sgst
	s.igst += igst
	s.gst += cgst + sgst
}

func slabKey(slab float64) string { return strconv.FormatFloat(slab, 'g', -1, 64) }

func roundPerSlab(perSlab map[float64]*slabAmt) map[string]any {
	out := map[string]any{}
	for slab, s := range perSlab {
		out[slabKey(slab)] = map[string]any{"cgst": r2(s.cgst), "sgst": r2(s.sgst), "igst": r2(s.igst), "gst": r2(s.gst)}
	}
	return out
}

func (h *Handler) Report(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)
	from := store.NormalizeDate(form.String("from"))
	to := store.NormalizeDate(form.String("to"))

	sales, err := h.store.GstSales(r.Context(), sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	purchases, err := h.store.GstPurchases(r.Context(), sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}

	slabSet := map[float64]bool{}
	salesTotals := newTotals()
	salesRows := []map[string]any{}
	sr := 1
	for _, doc := range sales {
		if !inRange(doc.Date, from, to) || len(doc.Lines) == 0 {
			continue
		}
		salesRows = append(salesRows, buildRow(map[string]any{
			"sr": sr, "invoiceNo": doc.InvoiceNo, "invoiceDate": doc.Date, "customerName": doc.PartyName, "gstNo": doc.GstNo,
		}, doc.Lines, salesTotals, slabSet))
		sr++
	}
	purchaseTotals := newTotals()
	purchaseRows := []map[string]any{}
	sr = 1
	for _, doc := range purchases {
		if !inRange(doc.Date, from, to) || len(doc.Lines) == 0 {
			continue
		}
		purchaseRows = append(purchaseRows, buildRow(map[string]any{
			"sr": sr, "invoiceNo": doc.InvoiceNo, "invoiceDate": doc.Date, "customerName": doc.PartyName, "gstNo": doc.GstNo,
		}, doc.Lines, purchaseTotals, slabSet))
		sr++
	}

	slabs := make([]float64, 0, len(slabSet))
	for s := range slabSet {
		slabs = append(slabs, s)
	}
	sort.Float64s(slabs)

	summary := make([]map[string]any, 0, len(slabs))
	var sumCollected, sumPaid, sumNet float64
	for _, slab := range slabs {
		col := salesTotals.perSlab[slab]
		if col == nil {
			col = &slabAmt{}
		}
		pd := purchaseTotals.perSlab[slab]
		if pd == nil {
			pd = &slabAmt{}
		}
		colTotal := r2(col.cgst + col.sgst + col.igst)
		paidTotal := r2(pd.cgst + pd.sgst + pd.igst)
		net := r2(colTotal - paidTotal)
		sumCollected += colTotal
		sumPaid += paidTotal
		sumNet += net
		summary = append(summary, map[string]any{
			"slab":           slab,
			"collected":      map[string]any{"cgst": r2(col.cgst), "sgst": r2(col.sgst), "igst": r2(col.igst), "total": colTotal},
			"paid":           map[string]any{"cgst": r2(pd.cgst), "sgst": r2(pd.sgst), "igst": r2(pd.igst), "total": paidTotal},
			"toPayOrCollect": net,
		})
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code": 200, "message": "Operation successful.",
		"data": map[string]any{
			"slabs":        slabs,
			"sales":        map[string]any{"rows": salesRows, "totals": map[string]any{"billAmount": r2(salesTotals.billAmount), "finalBill": r2(salesTotals.finalBill), "perSlab": roundPerSlab(salesTotals.perSlab)}},
			"purchase":     map[string]any{"rows": purchaseRows, "totals": map[string]any{"billAmount": r2(purchaseTotals.billAmount), "finalBill": r2(purchaseTotals.finalBill), "perSlab": roundPerSlab(purchaseTotals.perSlab)}},
			"summary":      summary,
			"summaryTotal": map[string]any{"collected": r2(sumCollected), "paid": r2(sumPaid), "toPayOrCollect": r2(sumNet)},
		},
	})
}

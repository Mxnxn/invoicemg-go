package bank

import (
	"net/http"

	"github.com/mxnxn/invoicemg-go/internal/auth"
	"github.com/mxnxn/invoicemg-go/internal/bankledger"
	"github.com/mxnxn/invoicemg-go/internal/httpx"
	"github.com/mxnxn/invoicemg-go/internal/store"
)

// Report is POST /bank/report (routes/Bank.js): per-bank cash movement, computed by the pure
// bankledger over the company's batch receipts, supplier payments and expenses, seeded with each
// bank's carried-in opening balance. Optional bank_id / from / to filters; totals summed
// server-side so the footer cannot disagree with the rows.
func (h *Handler) Report(w http.ResponseWriter, r *http.Request) {
	sess := auth.MustFrom(r.Context())
	form, _ := httpx.ReadForm(r)

	data, err := h.store.ReportData(r.Context(), sess.CompanyID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}

	openings := make(map[string]float64, len(data.Banks))
	names := make(map[string]string, len(data.Banks))
	banksDTO := make([]reportBank, 0, len(data.Banks))
	for _, b := range data.Banks {
		openings[string(b.ID)] = b.OpeningBalance
		names[string(b.ID)] = b.Name
		banksDTO = append(banksDTO, reportBank{ID: string(b.ID), Name: b.Name, OpeningBalance: b.OpeningBalance})
	}

	ledger := bankledger.Compute(bankledger.Sources{
		BatchReceives:    toInputs(data.BatchReceives),
		SupplierPayments: toInputs(data.SupplierPayments),
		Expenses:         toInputs(data.Expenses),
	}, form.String("from"), form.String("to"), openings)

	filter := form.String("bank_id")
	rows := make([]reportRow, 0, len(ledger))
	var totals reportTotals
	for _, row := range ledger {
		if filter != "" && row.BankID != filter {
			continue
		}
		name := "Deleted bank"
		if row.BankID == bankledger.Unassigned {
			name = "Unassigned"
		} else if n, ok := names[row.BankID]; ok {
			name = n
		}
		rows = append(rows, reportRow{Row: row, BankName: name})
		totals.Opening += row.Opening
		totals.Credits += row.Credits
		totals.Debits += row.Debits
		totals.Closing += row.Closing
		totals.Current += row.Current
	}

	httpx.Write(w, httpx.Envelope{Code: 200, Message: "Operation successful.", Data: reportData{
		Rows: rows, Totals: totals, Banks: banksDTO,
	}})
}

func toInputs(txns []store.BankTxn) []bankledger.Input {
	out := make([]bankledger.Input, 0, len(txns))
	for _, t := range txns {
		out = append(out, bankledger.Input{BankID: string(t.BankID), Date: t.Date, Amount: t.Amount, Note: t.Note})
	}
	return out
}

// reportRow embeds the ledger row and adds the bank's display name; embedding keeps the ledger
// fields flat in the JSON, matching Node's {...row, bankName}.
type reportRow struct {
	bankledger.Row
	BankName string `json:"bankName"`
}

type reportTotals struct {
	Opening float64 `json:"opening"`
	Credits float64 `json:"credits"`
	Debits  float64 `json:"debits"`
	Closing float64 `json:"closing"`
	Current float64 `json:"current"`
}

type reportBank struct {
	ID             string  `json:"_id"`
	Name           string  `json:"name"`
	OpeningBalance float64 `json:"openingBalance"`
}

type reportData struct {
	Rows   []reportRow  `json:"rows"`
	Totals reportTotals `json:"totals"`
	Banks  []reportBank `json:"banks"`
}

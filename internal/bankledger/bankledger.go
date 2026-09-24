// Package bankledger ports Helpers/BankLedger.js - the per-bank cash-movement computation behind
// the Bank Report. Pure (no store), the same shape as clientdues/cashflow: the caller passes the
// rows, gets per-bank summaries + in-range transaction lists back, so the summary and the list can
// never disagree about a balance.
//
// Signs, as specified:
//
//	Batch Receive (customer money in) -> credit (+)
//	Supplier Payment (money out)      -> debit  (-)
//	Expense                           -> debit  (-)
//
// InvoiceReceived is deliberately NOT counted (it settles invoices, it is not a bank transfer).
package bankledger

import (
	"math"
	"sort"
)

// Unassigned buckets rows whose bank_id was never set (the older /client/batchUpdate flow left it
// null). They still moved real money, so dropping them would make the report disagree with actual
// cash movement.
const Unassigned = "unassigned"

func roundMoney(v float64) float64 { return math.Floor(v*100+0.5) / 100 }

// Input is one source row (a batch receipt, supplier payment or expense).
type Input struct {
	BankID string
	Date   string
	Amount float64
	Note   string
}

// Sources are the three cash-moving collections for a company.
type Sources struct {
	BatchReceives    []Input
	SupplierPayments []Input
	Expenses         []Input
}

// Txn is one row in a bank's transaction list. Opening marks the synthetic carried-in row.
type Txn struct {
	BankID  string  `json:"bankId"`
	Date    string  `json:"date"`
	Type    string  `json:"type"`
	Note    string  `json:"note"`
	Amount  float64 `json:"amount"`
	Balance float64 `json:"balance"`
	Opening bool    `json:"opening,omitempty"`
}

// Row is one bank's summary plus its in-range transactions.
type Row struct {
	BankID   string  `json:"bankId"`
	Opening  float64 `json:"opening"`
	Credits  float64 `json:"credits"`
	Debits   float64 `json:"debits"`
	Closing  float64 `json:"closing"`
	Current  float64 `json:"current"`
	Rows     []Txn   `json:"rows"`
}

// ToTransactions flattens the three sources into one signed, date-sorted list.
func ToTransactions(s Sources) []Txn {
	out := make([]Txn, 0, len(s.BatchReceives)+len(s.SupplierPayments)+len(s.Expenses))
	for _, r := range s.BatchReceives {
		out = append(out, Txn{BankID: bankKey(r.BankID), Date: r.Date, Type: "Batch Receive", Note: r.Note, Amount: roundMoney(r.Amount)})
	}
	for _, r := range s.SupplierPayments {
		out = append(out, Txn{BankID: bankKey(r.BankID), Date: r.Date, Type: "Supplier Payment", Note: r.Note, Amount: -roundMoney(r.Amount)})
	}
	for _, r := range s.Expenses {
		out = append(out, Txn{BankID: bankKey(r.BankID), Date: r.Date, Type: "Expense", Note: r.Note, Amount: -roundMoney(r.Amount)})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out
}

func bankKey(id string) string {
	if id == "" {
		return Unassigned
	}
	return id
}

// Compute returns one summary per bank with any activity, plus the in-range transactions.
// openingBalances is bankId -> carried-in rupees (predates every transaction: lands in opening
// whatever the range, and in current). Opening = everything strictly before `from`; current = the
// whole history regardless of the range (a range ending in the past still reports today's worth).
func Compute(s Sources, from, to string, openingBalances map[string]float64) []Row {
	all := ToTransactions(s)

	type acc struct {
		opening, credits, debits, current float64
		rows                              []Txn
	}
	banks := map[string]*acc{}
	order := []string{}
	bucket := func(id string) *acc {
		a, ok := banks[id]
		if !ok {
			a = &acc{rows: []Txn{}}
			banks[id] = a
			order = append(order, id)
		}
		return a
	}

	// Seeded before the transactions so a bank whose only balance is carried in still appears -
	// but only non-zero ones, so accounts nothing ever moved through don't fill the report.
	for id, amount := range openingBalances {
		v := roundMoney(amount)
		if v == 0 {
			continue
		}
		a := bucket(id)
		a.opening += v
		a.current += v
	}
	// Deterministic seed order (map ranging is random in Go) so ties in the final sort are stable.
	sort.SliceStable(order, func(i, j int) bool { return order[i] < order[j] })

	for _, tx := range all {
		a := bucket(tx.BankID)
		a.current += tx.Amount
		if from != "" && tx.Date < from {
			a.opening += tx.Amount
			continue
		}
		if to != "" && tx.Date > to {
			continue
		}
		if tx.Amount >= 0 {
			a.credits += tx.Amount
		} else {
			a.debits += -tx.Amount
		}
		a.rows = append(a.rows, tx)
	}

	out := make([]Row, 0, len(order))
	for _, id := range order {
		a := banks[id]
		opening := roundMoney(a.opening)
		balance := opening
		rows := make([]Txn, 0, len(a.rows)+1)
		for _, tx := range a.rows {
			balance += tx.Amount
			tx.Balance = roundMoney(balance)
			rows = append(rows, tx)
		}
		// Prepended after credits/debits are totalled, so the list's starting balance is visible
		// without being counted as period movement. Skipped at zero (noise, not information).
		if opening != 0 {
			rows = append([]Txn{{BankID: id, Type: "Opening Balance", Amount: opening, Balance: opening, Opening: true}}, rows...)
		}
		out = append(out, Row{
			BankID: id, Opening: opening, Credits: roundMoney(a.credits), Debits: roundMoney(a.debits),
			Closing: roundMoney(balance), Current: roundMoney(a.current), Rows: rows,
		})
	}
	// Richest first; stable so equal-current banks keep their (id-sorted) seed order.
	sort.SliceStable(out, func(i, j int) bool { return out[i].Current > out[j].Current })
	return out
}

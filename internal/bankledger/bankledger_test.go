package bankledger

import "testing"

// Values captured from Helpers/BankLedger.test.js.

func TestToTransactions_Signs(t *testing.T) {
	tx := ToTransactions(Sources{
		BatchReceives:    []Input{{BankID: "b1", Date: "2026-05-10", Amount: 1000}},
		SupplierPayments: []Input{{BankID: "b1", Date: "2026-05-11", Amount: 400}},
		Expenses:         []Input{{BankID: "b1", Date: "2026-05-12", Amount: 100, Note: "Fuel"}},
	})
	want := map[string]float64{"Batch Receive": 1000, "Supplier Payment": -400, "Expense": -100}
	for _, tr := range tx {
		if want[tr.Type] != tr.Amount {
			t.Errorf("%s amount = %v, want %v", tr.Type, tr.Amount, want[tr.Type])
		}
	}
}

func TestToTransactions_DateOrderAcrossSources(t *testing.T) {
	tx := ToTransactions(Sources{
		BatchReceives: []Input{{BankID: "b1", Date: "2026-05-20", Amount: 1000}},
		Expenses:      []Input{{BankID: "b1", Date: "2026-05-01", Amount: 100}},
	})
	if tx[0].Date != "2026-05-01" || tx[1].Date != "2026-05-20" {
		t.Errorf("dates = %v, %v; want sorted", tx[0].Date, tx[1].Date)
	}
}

func TestCompute_BalancesNoRange(t *testing.T) {
	rows := Compute(Sources{
		BatchReceives:    []Input{{BankID: "b1", Date: "2026-05-10", Amount: 1000}},
		SupplierPayments: []Input{{BankID: "b1", Date: "2026-05-11", Amount: 400}},
		Expenses:         []Input{{BankID: "b1", Date: "2026-05-12", Amount: 100}},
	}, "", "", nil)
	b := rows[0]
	if b.Credits != 1000 || b.Debits != 500 || b.Current != 500 || b.Opening != 0 {
		t.Errorf("got credits=%v debits=%v current=%v opening=%v; want 1000/500/500/0", b.Credits, b.Debits, b.Current, b.Opening)
	}
}

func TestCompute_BeforeFromBecomesOpening(t *testing.T) {
	rows := Compute(Sources{
		BatchReceives:    []Input{{BankID: "b1", Date: "2026-01-01", Amount: 5000}, {BankID: "b1", Date: "2026-05-10", Amount: 1000}},
		SupplierPayments: []Input{{BankID: "b1", Date: "2026-05-11", Amount: 400}},
	}, "2026-05-01", "2026-05-31", nil)
	b := rows[0]
	if b.Opening != 5000 || b.Credits != 1000 || b.Closing != 5600 || b.Current != 5600 {
		t.Errorf("got opening=%v credits=%v closing=%v current=%v; want 5000/1000/5600/5600", b.Opening, b.Credits, b.Closing, b.Current)
	}
}

func TestCompute_CurrentIgnoresRange(t *testing.T) {
	rows := Compute(Sources{
		BatchReceives: []Input{{BankID: "b1", Date: "2026-05-10", Amount: 1000}, {BankID: "b1", Date: "2026-09-09", Amount: 7}},
	}, "2026-05-01", "2026-05-31", nil)
	b := rows[0]
	if b.Closing != 1000 || b.Current != 1007 {
		t.Errorf("got closing=%v current=%v; want 1000/1007", b.Closing, b.Current)
	}
}

func TestCompute_RunningBalancePerRow(t *testing.T) {
	rows := Compute(Sources{
		BatchReceives:    []Input{{BankID: "b1", Date: "2026-05-10", Amount: 1000}},
		SupplierPayments: []Input{{BankID: "b1", Date: "2026-05-11", Amount: 400}},
	}, "", "", nil)
	got := []float64{}
	for _, tx := range rows[0].Rows {
		got = append(got, tx.Balance)
	}
	if len(got) != 2 || got[0] != 1000 || got[1] != 600 {
		t.Errorf("row balances = %v, want [1000 600]", got)
	}
}

func TestCompute_MultipleBanksSortedByCurrent(t *testing.T) {
	rows := Compute(Sources{
		BatchReceives: []Input{{BankID: "b1", Date: "2026-05-10", Amount: 100}, {BankID: "b2", Date: "2026-05-10", Amount: 900}},
	}, "", "", nil)
	if len(rows) != 2 || rows[0].BankID != "b2" {
		t.Errorf("got %d rows, first %q; want 2 with b2 first (richest)", len(rows), rows[0].BankID)
	}
}

func TestCompute_NullBankBucketsUnassigned(t *testing.T) {
	rows := Compute(Sources{
		BatchReceives: []Input{{BankID: "", Date: "2026-05-10", Amount: 250}},
	}, "", "", nil)
	if rows[0].BankID != Unassigned || rows[0].Current != 250 {
		t.Errorf("got %q/%v, want unassigned/250", rows[0].BankID, rows[0].Current)
	}
}

func TestCompute_OpeningBalanceSeedsRowAndSyntheticTxn(t *testing.T) {
	rows := Compute(Sources{
		BatchReceives: []Input{{BankID: "b1", Date: "2026-05-10", Amount: 100}},
	}, "", "", map[string]float64{"b1": 500, "b2": 0}) // b2's zero is skipped
	b := rows[0]
	if b.BankID != "b1" || b.Opening != 500 || b.Current != 600 {
		t.Errorf("got %q opening=%v current=%v; want b1/500/600", b.BankID, b.Opening, b.Current)
	}
	if len(b.Rows) != 2 || !b.Rows[0].Opening || b.Rows[0].Type != "Opening Balance" || b.Rows[0].Balance != 500 {
		t.Errorf("first row should be the synthetic opening (500): %+v", b.Rows)
	}
	// The zero-balance b2 must not appear.
	if len(rows) != 1 {
		t.Errorf("rows = %d, want 1 (zero opening skipped)", len(rows))
	}
}

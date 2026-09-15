package xlsxexport

import (
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestWriteClientLedger(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.xlsx")
	rows := []LedgerRow{
		{Date: "2026-09-10", InvoiceNumber: "INV/1", NonTaxedValue: 1000, TaxedValue: 1180, EntryReceived: 500, Due: 680},
	}
	if err := WriteClientLedger(path, "Acme Signs", rows, 680); err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if v, _ := f.GetCellValue(sheet, "A7"); v != "Acme Signs" {
		t.Errorf("title cell = %q", v)
	}
	if v, _ := f.GetCellValue(sheet, "A9"); v != "Date" {
		t.Errorf("header cell = %q", v)
	}
	if v, _ := f.GetCellValue(sheet, "C11"); v != "INV/1" {
		t.Errorf("data invoice cell = %q", v)
	}
}

func TestWriteClientEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "entries.xlsx")
	rows := []EntryRow{
		{Date: "10/09/2026", Material: "Vinyl", Description: "Banner", Length: 3, Width: 4, Qty: 2, Rate: 50, Amount: 1200, Total: 200, Advance: 1000, Invoiced: "Not Invoiced"},
	}
	if err := WriteClientEntries(path, rows); err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if v, _ := f.GetCellValue(sheet, "A7"); v != "Date" {
		t.Errorf("header = %q", v)
	}
	if v, _ := f.GetCellValue(sheet, "C9"); v != "Vinyl" {
		t.Errorf("material = %q", v)
	}
	if v, _ := f.GetCellValue(sheet, "Y9"); v != "Not Invoiced" {
		t.Errorf("invoiced = %q", v)
	}
}

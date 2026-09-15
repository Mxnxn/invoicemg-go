// Package xlsxexport writes the two generated spreadsheets the API offers, the Go port of
// Excel/LedgerExcel.js (a client's invoice ledger) and Excel/DataToExcel.js (a client's line
// items). The .xlsx bytes are not part of the wire contract - the routes only return the
// filename and the file is streamed back on download - so these reproduce the column layout and
// data faithfully with excelize, not byte-for-byte with exceljs.
package xlsxexport

import (
	"fmt"

	"github.com/xuri/excelize/v2"
)

const sheet = "ExampleSheet"

// LedgerRow is one invoice line of the client ledger export (LedgerExcel.addDataToRow).
type LedgerRow struct {
	Date          string
	InvoiceNumber string
	NonTaxedValue float64
	TaxedValue    float64
	EntryReceived float64
	Due           float64
}

// WriteClientLedger writes the invoice-ledger workbook to path: a title band with the client
// name, a header row, one row per invoice, a totals row carrying the total due, and a footer.
func WriteClientLedger(path, clientName string, rows []LedgerRow, totalDue float64) error {
	f := excelize.NewFile()
	defer f.Close()
	idx, err := f.NewSheet(sheet)
	if err != nil {
		return err
	}
	f.SetActiveSheet(idx)
	_ = f.DeleteSheet("Sheet1")

	border, err := borderStyle(f, false)
	if err != nil {
		return err
	}
	head, err := borderStyle(f, true)
	if err != nil {
		return err
	}

	merge := func(a, b, value string, style int) {
		_ = f.MergeCell(sheet, a, b)
		_ = f.SetCellValue(sheet, a, value)
		_ = f.SetCellStyle(sheet, a, b, style)
	}
	mergeNum := func(a, b string, value float64, style int) {
		_ = f.MergeCell(sheet, a, b)
		_ = f.SetCellValue(sheet, a, value)
		_ = f.SetCellStyle(sheet, a, b, style)
	}

	// Title band with the client name (LedgerExcel createHeader: A7:L8).
	merge("A7", "L8", clientName, head)

	// Column heads at row 9-10.
	cols := []struct{ a, b, label string }{
		{"A9", "B10", "Date"}, {"C9", "D10", "Invoice Number"}, {"E9", "F10", "Non-Tax Amount"},
		{"G9", "H10", "Total"}, {"I9", "J10", "Received"}, {"K9", "L10", "Due/Remaining"},
	}
	for _, c := range cols {
		merge(c.a, c.b, c.label, head)
	}

	row := 11
	for _, r := range rows {
		merge(fmt.Sprintf("A%d", row), fmt.Sprintf("B%d", row), r.Date, border)
		merge(fmt.Sprintf("C%d", row), fmt.Sprintf("D%d", row), r.InvoiceNumber, border)
		mergeNum(fmt.Sprintf("E%d", row), fmt.Sprintf("F%d", row), r.NonTaxedValue, border)
		mergeNum(fmt.Sprintf("G%d", row), fmt.Sprintf("H%d", row), r.TaxedValue, border)
		mergeNum(fmt.Sprintf("I%d", row), fmt.Sprintf("J%d", row), r.EntryReceived, border)
		mergeNum(fmt.Sprintf("K%d", row), fmt.Sprintf("L%d", row), r.Due, border)
		row++
	}

	// Totals row (addTotalsRow): blank A..J, totalDue in K..L.
	merge(fmt.Sprintf("A%d", row), fmt.Sprintf("J%d", row+1), "", head)
	mergeNum(fmt.Sprintf("K%d", row), fmt.Sprintf("L%d", row+1), totalDue, head)
	row += 2

	// Footer.
	merge(fmt.Sprintf("A%d", row), fmt.Sprintf("L%d", row+1), "THANK YOU", head)

	return f.SaveAs(path)
}

// EntryRow is one line item of the client-entries export (DataToExcel.addDataToRow).
type EntryRow struct {
	Date        string
	Material    string
	Description string
	Length      float64
	Width       float64
	Qty         float64
	Rate        float64
	Amount      float64
	Total       float64 // amount - advance
	Advance     float64
	Invoiced    string // invoice number, or "Not Invoiced"
}

// WriteClientEntries writes the line-items workbook to path: a header row and one row per entry.
func WriteClientEntries(path string, rows []EntryRow) error {
	f := excelize.NewFile()
	defer f.Close()
	idx, err := f.NewSheet(sheet)
	if err != nil {
		return err
	}
	f.SetActiveSheet(idx)
	_ = f.DeleteSheet("Sheet1")

	border, err := borderStyle(f, false)
	if err != nil {
		return err
	}
	head, err := borderStyle(f, true)
	if err != nil {
		return err
	}

	merge := func(a, b, value string, style int) {
		_ = f.MergeCell(sheet, a, b)
		_ = f.SetCellValue(sheet, a, value)
		_ = f.SetCellStyle(sheet, a, b, style)
	}
	mergeNum := func(a, b string, value float64, style int) {
		_ = f.MergeCell(sheet, a, b)
		_ = f.SetCellValue(sheet, a, value)
		_ = f.SetCellStyle(sheet, a, b, style)
	}

	// Column heads at row 7-8 (DataToExcel createColumnHead(7)).
	cols := []struct{ a, b, label string }{
		{"A7", "B8", "Date"}, {"C7", "F8", "Material"}, {"G7", "J8", "Description"},
		{"K7", "L8", "Length (SQ.ft)"}, {"M7", "N8", "Width (SQ.ft)"}, {"O7", "P8", "Qty"},
		{"Q7", "R8", "Rate"}, {"S7", "T8", "Amount"}, {"U7", "V8", "Total"},
		{"W7", "X8", "Advance"}, {"Y7", "Z8", "Invoiced"},
	}
	for _, c := range cols {
		merge(c.a, c.b, c.label, head)
	}

	row := 9
	for _, r := range rows {
		merge(fmt.Sprintf("A%d", row), fmt.Sprintf("B%d", row), r.Date, border)
		merge(fmt.Sprintf("C%d", row), fmt.Sprintf("F%d", row), r.Material, border)
		merge(fmt.Sprintf("G%d", row), fmt.Sprintf("J%d", row), r.Description, border)
		mergeNum(fmt.Sprintf("K%d", row), fmt.Sprintf("L%d", row), r.Length, border)
		mergeNum(fmt.Sprintf("M%d", row), fmt.Sprintf("N%d", row), r.Width, border)
		mergeNum(fmt.Sprintf("O%d", row), fmt.Sprintf("P%d", row), r.Qty, border)
		mergeNum(fmt.Sprintf("Q%d", row), fmt.Sprintf("R%d", row), r.Rate, border)
		mergeNum(fmt.Sprintf("S%d", row), fmt.Sprintf("T%d", row), r.Amount, border)
		mergeNum(fmt.Sprintf("U%d", row), fmt.Sprintf("V%d", row), r.Total, border)
		mergeNum(fmt.Sprintf("W%d", row), fmt.Sprintf("X%d", row), r.Advance, border)
		merge(fmt.Sprintf("Y%d", row), fmt.Sprintf("Z%d", row), r.Invoiced, border)
		row++
	}

	return f.SaveAs(path)
}

// borderStyle is the thin-bordered, centered cell style the sheets use; bold for headers.
func borderStyle(f *excelize.File, bold bool) (int, error) {
	return f.NewStyle(&excelize.Style{
		Border: []excelize.Border{
			{Type: "top", Color: "8F8F8F", Style: 1},
			{Type: "left", Color: "8F8F8F", Style: 1},
			{Type: "bottom", Color: "8F8F8F", Style: 1},
			{Type: "right", Color: "8F8F8F", Style: 1},
		},
		Alignment: &excelize.Alignment{Vertical: "center", Horizontal: "center"},
		Font:      &excelize.Font{Bold: bold},
	})
}

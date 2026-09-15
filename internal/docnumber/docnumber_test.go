package docnumber

import (
	"testing"
	"time"
)

func TestFinancialYear(t *testing.T) {
	cases := map[string]string{
		"2026-09-15": "26-27", // September -> FY starts 2026
		"2026-04-01": "26-27", // April 1 is the first day of the new FY
		"2026-03-31": "25-26", // March 31 is the last day of the prior FY
		"2026-01-10": "25-26", // January belongs to the FY that started last year
		"2027-04-01": "27-28",
	}
	for in, want := range cases {
		d, _ := time.Parse("2006-01-02", in)
		if got := FinancialYear(d); got != want {
			t.Errorf("FinancialYear(%s) = %q, want %q", in, got, want)
		}
	}
}

// Values captured from Helpers/DocumentNumbering.nextDocumentNumber with now in FY 26-27.
func TestNext(t *testing.T) {
	now, _ := time.Parse("2006-01-02", "2026-09-15")

	if got := Next(nil, "QT-", now); got != "MG/26-27/QT-00001" {
		t.Errorf("empty -> %q, want MG/26-27/QT-00001", got)
	}

	mixed := []string{
		"MG/26-27/QT-00003", "MG/26-27/QT-00001",
		"MG/25-26/QT-00009",  // different FY, ignored
		"MG/26-27/INV-00050", // different code, ignored
		"garbage",
	}
	if got := Next(mixed, "QT-", now); got != "MG/26-27/QT-00004" {
		t.Errorf("mixed -> %q, want MG/26-27/QT-00004", got)
	}
}

// Jobs use the empty code, so their prefix is MG/<FY>/ with no sub-code.
func TestNext_NoCode(t *testing.T) {
	now, _ := time.Parse("2006-01-02", "2026-09-15")
	if got := Next([]string{"MG/26-27/00007"}, "", now); got != "MG/26-27/00008" {
		t.Errorf("no-code -> %q, want MG/26-27/00008", got)
	}
}

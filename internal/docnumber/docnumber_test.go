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

func strptr(s string) *string { return &s }

func TestFormatDocumentNumber(t *testing.T) {
	now := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC) // FY 26-27
	cases := []struct {
		f    Format
		seq  int
		want string
	}{
		{DefaultFormats["invoice"], 1, "INV/26-27/000001"},
		{DefaultFormats["job"], 1, "JOB/26-27/000001"},
		{DefaultFormats["quotation"], 1, "QT/26-27/000001"},
		{DefaultFormats["purchase"], 1, "PURINV/26-27/000001"},
		{Format{Prefix: "INV", Year: "none", Pad: 6, Separator: "/"}, 1, "INV/000001"},
		{Format{Prefix: "INV", Year: "yy", Pad: 4, Separator: "-"}, 5, "INV-26-0005"},
		{Format{Prefix: "INV", Year: "yyyy", Pad: 3, Separator: "/"}, 42, "INV/2026/042"},
		{Format{Prefix: "", Year: "none", Pad: 4, Separator: "/"}, 7, "0007"}, // no prefix, no year
	}
	for _, c := range cases {
		if got := FormatDocumentNumber(c.f, c.seq, now); got != c.want {
			t.Errorf("FormatDocumentNumber(%+v, %d) = %q, want %q", c.f, c.seq, got, c.want)
		}
	}
}

func TestNormalizeFormat(t *testing.T) {
	fb := DefaultFormats["invoice"] // {INV, fy, 6, /}

	// all nil -> fallback unchanged
	if got := NormalizeFormat(RawFormat{}, fb); got != fb {
		t.Errorf("empty raw = %+v, want fallback %+v", got, fb)
	}
	// prefix trimmed; separator taken as-is; year kept when valid
	got := NormalizeFormat(RawFormat{Prefix: strptr("  ABC  "), Year: strptr("yyyy"), Separator: strptr("-")}, fb)
	if got.Prefix != "ABC" || got.Year != "yyyy" || got.Separator != "-" || got.Pad != 6 {
		t.Errorf("normalise = %+v", got)
	}
	// empty prefix is kept as "", not replaced by fallback
	if got := NormalizeFormat(RawFormat{Prefix: strptr("")}, fb); got.Prefix != "" {
		t.Errorf("empty prefix should stay empty, got %q", got.Prefix)
	}
	// invalid year falls back
	if got := NormalizeFormat(RawFormat{Year: strptr("banana")}, fb); got.Year != "fy" {
		t.Errorf("invalid year should fall back, got %q", got.Year)
	}
	// pad: clamp low/high, round, and NaN -> fallback; ""(Number 0) -> clamped to 1
	pads := map[string]int{"0": 1, "1": 1, "6": 6, "8": 8, "99": 8, "6.7": 7, "abc": 6, "": 1}
	for in, want := range pads {
		if got := NormalizeFormat(RawFormat{Pad: strptr(in)}, fb); got.Pad != want {
			t.Errorf("pad %q -> %d, want %d", in, got.Pad, want)
		}
	}
}

// Package docnumber is the shared user-facing document-numbering scheme, ported from
// Helpers/DocumentNumbering.js: MG/<financial-year>/<code><00001>, where the financial year runs
// April 1 - March 31 (e.g. 26-27 for Apr 2026 - Mar 2027) and the sequence restarts at 1 each new
// year. Each document type passes its own code ("QT-" quotations, "INV-" invoices; jobs use "")
// so their sequences never collide, but all three share this one rollover so they can't drift.
package docnumber

import (
	"math"
	"strconv"
	"strings"
	"time"
)

// --- Per-company configurable formats (Helpers/DocumentNumbering.js 9a-9d) ---
//
// The newer scheme: a format is three plain fields per document type, giving e.g.
// INV/26-27/000001. This lives alongside the legacy MG scheme (Next, above) - the config routes
// read and write these; wiring the doc-creation routes to honour them is a separate step.

const (
	yearFY   = "fy"
	yearYY   = "yy"
	yearYYYY = "yyyy"
	yearNone = "none"
	padMin   = 1
	padMax   = 8
)

// Format is one document type's numbering rule.
type Format struct {
	Prefix    string `json:"prefix"`
	Year      string `json:"year"` // "fy" | "yy" | "yyyy" | "none"
	Pad       int    `json:"pad"`
	Separator string `json:"separator"`
}

// RawFormat is a partial format as it arrives (form fields or stored JSON): a nil field means
// "not supplied", so the fallback's value is used. Pad is a raw string, coerced JS-Number-style.
type RawFormat struct {
	Prefix    *string
	Year      *string
	Pad       *string
	Separator *string
}

// DefaultFormats is DEFAULT_FORMATS: what a fresh company starts each series with. Five kinds -
// the update route accepts all five; the read route shows the first four.
var DefaultFormats = map[string]Format{
	"invoice":       {Prefix: "INV", Year: yearFY, Pad: 6, Separator: "/"},
	"job":           {Prefix: "JOB", Year: yearFY, Pad: 6, Separator: "/"},
	"quotation":     {Prefix: "QT", Year: yearFY, Pad: 6, Separator: "/"},
	"purchase":      {Prefix: "PURINV", Year: yearFY, Pad: 6, Separator: "/"},
	"purchaseOrder": {Prefix: "PO", Year: yearFY, Pad: 6, Separator: "/"},
}

// NormalizeFormat is normalizeFormat: fill each unsupplied field from fallback; keep prefix
// trimmed; keep year only if it is one of the four valid values; coerce pad JS-Number-style and
// clamp to 1..8; separator taken as-is.
func NormalizeFormat(in RawFormat, fallback Format) Format {
	f := fallback
	if in.Prefix != nil {
		f.Prefix = strings.TrimSpace(*in.Prefix)
	}
	if in.Year != nil {
		switch *in.Year {
		case yearFY, yearYY, yearYYYY, yearNone:
			f.Year = *in.Year
		}
	}
	if in.Pad != nil {
		if n, ok := jsNumber(*in.Pad); ok {
			p := int(math.Floor(n + 0.5)) // Math.round: half toward +Inf
			if p < padMin {
				p = padMin
			}
			if p > padMax {
				p = padMax
			}
			f.Pad = p
		}
	}
	if in.Separator != nil {
		f.Separator = *in.Separator
	}
	return f
}

// jsNumber mirrors JavaScript's Number(): a blank string is 0, an unparseable one is NaN (ok
// false), everything else its float value.
func jsNumber(s string) (float64, bool) {
	t := strings.TrimSpace(s)
	if t == "" {
		return 0, true
	}
	v, err := strconv.ParseFloat(t, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func yearPart(f Format, now time.Time) string {
	switch f.Year {
	case yearFY:
		return FinancialYear(now)
	case yearYY:
		return pad2(now.Year())
	case yearYYYY:
		return strconv.Itoa(now.Year())
	default:
		return ""
	}
}

// BuildPrefix is buildPrefix: everything before the sequence, including the trailing separator.
func BuildPrefix(f Format, now time.Time) string {
	parts := make([]string, 0, 2)
	if f.Prefix != "" {
		parts = append(parts, f.Prefix)
	}
	if yp := yearPart(f, now); yp != "" {
		parts = append(parts, yp)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, f.Separator) + f.Separator
}

// FormatDocumentNumber is formatDocumentNumber: the prefix plus the zero-padded sequence.
func FormatDocumentNumber(f Format, sequence int, now time.Time) string {
	seq := strconv.Itoa(sequence)
	for len(seq) < f.Pad {
		seq = "0" + seq
	}
	return BuildPrefix(f, now) + seq
}

// FinancialYear is currentFinancialYear: the "YY-YY" label of the FY containing t. April (month
// index 3) or later belongs to the year that starts this calendar year; Jan-Mar to the prior one.
func FinancialYear(t time.Time) string {
	startYear := t.Year()
	if int(t.Month()) < 4 { // Jan, Feb, Mar
		startYear--
	}
	return pad2(startYear) + "-" + pad2(startYear+1)
}

func pad2(year int) string {
	s := strconv.Itoa(year)
	if len(s) >= 2 {
		return s[len(s)-2:]
	}
	return s
}

// Next is nextDocumentNumber: the next free MG/<FY>/<code>NNNNN for `now`, one past the highest
// existing number that shares this exact FY+code prefix. Numbers from other years or codes are
// ignored, so the sequence is per (FY, code). The counter is zero-padded to five digits.
func Next(existing []string, code string, now time.Time) string {
	prefix := "MG/" + FinancialYear(now) + "/" + code
	maxN := 0
	for _, value := range existing {
		if !strings.HasPrefix(value, prefix) {
			continue
		}
		n, err := strconv.Atoi(value[len(prefix):])
		if err == nil && n > maxN {
			maxN = n
		}
	}
	return prefix + pad5(maxN+1)
}

func pad5(n int) string {
	s := strconv.Itoa(n)
	for len(s) < 5 {
		s = "0" + s
	}
	return s
}

// Package docnumber is the shared user-facing document-numbering scheme, ported from
// Helpers/DocumentNumbering.js: MG/<financial-year>/<code><00001>, where the financial year runs
// April 1 - March 31 (e.g. 26-27 for Apr 2026 - Mar 2027) and the sequence restarts at 1 each new
// year. Each document type passes its own code ("QT-" quotations, "INV-" invoices; jobs use "")
// so their sequences never collide, but all three share this one rollover so they can't drift.
package docnumber

import (
	"strconv"
	"strings"
	"time"
)

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

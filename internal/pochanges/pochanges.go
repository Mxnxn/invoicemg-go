// Package pochanges ports Helpers/PoChanges.js - the pure logic behind purchase-order approval.
//
// fingerprint answers "is this still the document somebody approved" (stored AT approval, compared
// later, so it survives edits and reverts); diff answers "what did this save change" (audit log
// only). Using a per-save diff for both would get the first wrong: a PO edited 100 -> 150 -> 100 is
// the approved document again, yet every step's diff reported a change. Deliberately pure.
package pochanges

import (
	"sort"
	"strconv"
	"strings"
)

// FingerprintFields are the row fields that revoke approval when they change: product, quantity and
// the three that decide what is owed. Description/unit/hsn/gst are deliberately excluded - a typo
// fix must not need a second approval.
var FingerprintFields = []string{"material", "qty", "rate", "discount", "charges"}

// Row is one purchase-order line as the fingerprint/diff read it.
type Row struct {
	Material    string
	Description string
	Unit        string
	Hsn         string
	Qty         float64
	Rate        float64
	Discount    float64
	Charges     float64
	Gst         float64
}

// PO is the price-bearing content the fingerprint and diff operate on.
type PO struct {
	SupplierID string
	Date       string
	Total      float64
	Rows       []Row
}

// normNum renders a number the way JS String(Number) does (no trailing zeros).
func normNum(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

// normStr matches Node's norm on a string: trimmed, and collapsed to canonical numeric form when it
// parses as a number (so "2.50" and 2.5 fingerprint the same).
func normStr(s string) string {
	t := strings.TrimSpace(s)
	if t == "" {
		return ""
	}
	if n, err := strconv.ParseFloat(t, 64); err == nil {
		return normNum(n)
	}
	return t
}

func (r Row) fieldNorm(field string) string {
	switch field {
	case "material":
		return normStr(r.Material)
	case "description":
		return normStr(r.Description)
	case "unit":
		return normStr(r.Unit)
	case "hsn":
		return normStr(r.Hsn)
	case "qty":
		return normNum(r.Qty)
	case "rate":
		return normNum(r.Rate)
	case "discount":
		return normNum(r.Discount)
	case "charges":
		return normNum(r.Charges)
	case "gst":
		return normNum(r.Gst)
	}
	return ""
}

func rowSignature(r Row) string {
	parts := make([]string, len(FingerprintFields))
	for i, f := range FingerprintFields {
		parts[i] = f + "=" + r.fieldNorm(f)
	}
	return strings.Join(parts, ",")
}

// Fingerprint is the canonical string for a PO's price-bearing content. Row signatures are SORTED
// before joining, so moving a row up the list is not a change.
func Fingerprint(po PO) string {
	sigs := make([]string, 0, len(po.Rows))
	for _, r := range po.Rows {
		sigs = append(sigs, rowSignature(r))
	}
	sort.Strings(sigs)
	return "supplier=" + normStr(po.SupplierID) + ";total=" + normNum(po.Total) + ";rows=[" + strings.Join(sigs, "|") + "]"
}

// SendFingerprint is the approval fingerprint plus the order date: moving a delivery date does not
// revoke a financial approval, but it is exactly the news a supplier needs.
func SendFingerprint(po PO) string {
	return Fingerprint(po) + ";date=" + normStr(po.Date)
}

// Change is one field-level difference for the audit log.
type Change struct {
	Field string `json:"field"`
	From  string `json:"from"`
	To    string `json:"to"`
}

func rowLabel(index int, field string) string {
	base := "rows[" + strconv.Itoa(index+1) + "]"
	if field == "" {
		return base
	}
	return base + "." + field
}

func describeRow(r *Row) string {
	if r == nil {
		return ""
	}
	parts := make([]string, len(FingerprintFields))
	for i, f := range FingerprintFields {
		parts[i] = r.fieldNorm(f)
	}
	return strings.Join(parts, " / ")
}

// Diff reports the field-by-field changes between two versions (more than the fingerprint covers - a
// changed delivery date is logged though it does not revoke approval).
func Diff(before, after PO) []Change {
	changes := []Change{}

	if normStr(before.SupplierID) != normStr(after.SupplierID) {
		changes = append(changes, Change{Field: "supplier_id", From: normStr(before.SupplierID), To: normStr(after.SupplierID)})
	}
	if normStr(before.Date) != normStr(after.Date) {
		changes = append(changes, Change{Field: "date", From: normStr(before.Date), To: normStr(after.Date)})
	}
	if normNum(before.Total) != normNum(after.Total) {
		changes = append(changes, Change{Field: "total", From: normNum(before.Total), To: normNum(after.Total)})
	}

	max := len(before.Rows)
	if len(after.Rows) > max {
		max = len(after.Rows)
	}
	rowFields := append(append([]string{}, FingerprintFields...), "description", "unit", "hsn", "gst")
	for i := 0; i < max; i++ {
		var ra, rb *Row
		if i < len(before.Rows) {
			ra = &before.Rows[i]
		}
		if i < len(after.Rows) {
			rb = &after.Rows[i]
		}
		if ra == nil || rb == nil {
			changes = append(changes, Change{Field: rowLabel(i, ""), From: describeRow(ra), To: describeRow(rb)})
			continue
		}
		for _, f := range rowFields {
			if ra.fieldNorm(f) != rb.fieldNorm(f) {
				changes = append(changes, Change{Field: rowLabel(i, f), From: ra.fieldNorm(f), To: rb.fieldNorm(f)})
			}
		}
	}
	return changes
}

// IsAwaitingApproval is the single definition of "pending" used by the approval queue: a draft, not
// yet converted, with at least one row (/approve refuses an empty PO, so the queue must not list one).
func IsAwaitingApproval(state string, converted bool, rowCount int) bool {
	if state == "" {
		state = "draft"
	}
	return state == "draft" && !converted && rowCount > 0
}

package pochanges

import "testing"

func TestFingerprint_StableAcrossRowOrderAndStringNumberForms(t *testing.T) {
	a := PO{SupplierID: "s1", Total: 250, Rows: []Row{
		{Material: "Steel", Qty: 2, Rate: 100},
		{Material: "Bolt", Qty: 1, Rate: 50},
	}}
	// Same content, rows reordered, numbers as they'd arrive from a form (no effect after norm).
	b := PO{SupplierID: "s1", Total: 250, Rows: []Row{
		{Material: "Bolt", Qty: 1, Rate: 50},
		{Material: "Steel", Qty: 2, Rate: 100},
	}}
	if Fingerprint(a) != Fingerprint(b) {
		t.Errorf("row reorder changed fingerprint:\n a=%s\n b=%s", Fingerprint(a), Fingerprint(b))
	}
	// A rate change DOES change it.
	c := PO{SupplierID: "s1", Total: 300, Rows: []Row{{Material: "Steel", Qty: 2, Rate: 150}, {Material: "Bolt", Qty: 1, Rate: 50}}}
	if Fingerprint(a) == Fingerprint(c) {
		t.Error("a rate change must change the fingerprint")
	}
}

func TestSendFingerprint_IncludesDate(t *testing.T) {
	a := PO{SupplierID: "s1", Total: 100, Date: "2026-09-01", Rows: []Row{{Material: "X", Qty: 1, Rate: 100}}}
	b := PO{SupplierID: "s1", Total: 100, Date: "2026-09-05", Rows: []Row{{Material: "X", Qty: 1, Rate: 100}}}
	if Fingerprint(a) != Fingerprint(b) {
		t.Error("a date change must NOT change the approval fingerprint")
	}
	if SendFingerprint(a) == SendFingerprint(b) {
		t.Error("a date change MUST change the send fingerprint")
	}
}

func TestDiff_RowsAndTopLevel(t *testing.T) {
	before := PO{SupplierID: "s1", Date: "2026-09-01", Total: 100, Rows: []Row{{Material: "A", Qty: 1, Rate: 100, Description: "old"}}}
	after := PO{SupplierID: "s1", Date: "2026-09-02", Total: 150, Rows: []Row{{Material: "A", Qty: 1, Rate: 150, Description: "new"}}}
	changes := Diff(before, after)
	fields := map[string]Change{}
	for _, c := range changes {
		fields[c.Field] = c
	}
	if _, ok := fields["date"]; !ok {
		t.Error("date change should be logged")
	}
	if fields["total"].From != "100" || fields["total"].To != "150" {
		t.Errorf("total change wrong: %+v", fields["total"])
	}
	if _, ok := fields["rows[1].rate"]; !ok {
		t.Error("row rate change should be logged with 1-based label")
	}
	if _, ok := fields["rows[1].description"]; !ok {
		t.Error("row description change should be logged")
	}
}

func TestDiff_RowAddedReportedOnce(t *testing.T) {
	before := PO{Rows: []Row{{Material: "A", Qty: 1, Rate: 100}}}
	after := PO{Rows: []Row{{Material: "A", Qty: 1, Rate: 100}, {Material: "B", Qty: 2, Rate: 50}}}
	changes := Diff(before, after)
	count := 0
	for _, c := range changes {
		if c.Field == "rows[2]" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("an added row should be one change, got %d (%+v)", count, changes)
	}
}

func TestIsAwaitingApproval(t *testing.T) {
	if !IsAwaitingApproval("draft", false, 1) {
		t.Error("a draft with rows, not converted, is pending")
	}
	if IsAwaitingApproval("approved", false, 1) {
		t.Error("approved is not pending")
	}
	if IsAwaitingApproval("draft", true, 1) {
		t.Error("converted is not pending")
	}
	if IsAwaitingApproval("draft", false, 0) {
		t.Error("no rows is not pending")
	}
	if !IsAwaitingApproval("", false, 2) {
		t.Error("empty state defaults to draft")
	}
}

package joblifecycle

import "testing"

func boolPtr(b bool) *bool { return &b }

// The same job used to capture the Node reference values (see the commit message).
func sampleJob() Job {
	return Job{
		Unlocked: false,
		Rows: []Row{
			{ID: "r1", RowID: "JOB/1-000001", Material: "Vinyl", Description: "Decals", Qty: 10,
				HasDimensions: boolPtr(false), Length: "1", Width: "1", Rate: 45, Cgst: 9, Sgst: 9,
				Queue: "Done", EntryID: "e1"},
			{ID: "r2", RowID: "JOB/1-000002", Material: "Flex", Description: "Banner", Qty: 1,
				HasDimensions: boolPtr(true), Length: "2", Width: "3", Rate: 2500, Cgst: 9, Sgst: 9,
				Queue: "Printing", EntryID: ""},
		},
	}
}

func TestDeriveInvoiceState(t *testing.T) {
	s := DeriveInvoiceState(sampleJob().Rows, map[string]bool{"e1": true})
	if s.State != "partial" || s.InvoicedRows != 1 || s.TotalRows != 2 || s.ConvertedRows != 1 {
		t.Fatalf("got %+v, want partial/1/2/1", s)
	}
	if IsReadyForInvoice(sampleJob().Rows, s.State) {
		t.Error("a partial job is not ready for invoice")
	}
}

func TestLockState(t *testing.T) {
	l := LockState(false, true)
	if l.CanEditValues || l.CanEditQueue || l.CanDeleteJob || l.CanDeleteRow {
		t.Errorf("invoiced+locked must forbid all edits: %+v", l)
	}
	if !LockState(true, true).CanEditValues {
		t.Error("unlocked invoiced job may edit values")
	}
}

// The signature must be byte-identical to Node's - this is what decides whether a customer gets
// a "job changed" message, so a divergence is a real, visible bug.
func TestRowSignature_MatchesNode(t *testing.T) {
	if got := RowSignature(sampleJob().Rows); got != "ed1a04b5f0b6580eb4e4038b672e654440d68024" {
		t.Fatalf("signature = %s, want the Node sha1", got)
	}
}

func TestAlert_Created(t *testing.T) {
	a := Alert(sampleJob(), "created")
	if a.SentBefore || a.Count != 0 || !a.CanSend || a.IsUpdate || a.Changed || a.PendingRows != 0 {
		t.Errorf("created state wrong: %+v", a)
	}
	if a.Signature != "ed1a04b5f0b6580eb4e4038b672e654440d68024" {
		t.Errorf("created signature wrong: %s", a.Signature)
	}
}

func TestAlert_Done(t *testing.T) {
	a := Alert(sampleJob(), "done")
	// not every row is Done (r2 is Printing), so canSend is false, but one done row is unsent.
	if a.CanSend {
		t.Error("done canSend must be false when not every row is Done")
	}
	if !a.Changed || a.PendingRows != 1 {
		t.Errorf("done changed/pending wrong: %+v", a)
	}
	if len(a.DoneRowIDs) != 1 || a.DoneRowIDs[0] != "r1" {
		t.Errorf("doneRowIds wrong: %v", a.DoneRowIDs)
	}
}

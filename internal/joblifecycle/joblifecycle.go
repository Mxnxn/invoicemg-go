// Package joblifecycle is the pure derivation logic behind /lifecycle/jobs/list, ported from
// Helpers/JobInvoiceState.js, Helpers/JobLock.js, Helpers/JobDoneAlert.js and
// Helpers/JobAlertState.js. It computes, from a job and the set of invoiced entry ids, the
// derived state the Jobs board reads: invoiceState, lock, and the per-channel alert state -
// including the sha1 row signature that decides whether a "job changed" message is offered.
//
// It is pure (no store/driver), the same shape as jobmath/entrymath, and every tricky value is
// pinned in the test against what the real Node helpers printed.
package joblifecycle

import (
	"crypto/sha1"
	"encoding/hex"
	"math"
	"sort"
	"strconv"
	"strings"
)

const doneStage = "Done"

// inFlightOrArrived: statuses that block a resend of the created channel (Helpers/JobAlertState).
var inFlightOrArrived = map[string]bool{"accepted": true, "sent": true, "delivered": true, "read": true}

// Row is the alert/invoice-relevant subset of a job row.
type Row struct {
	ID            string
	RowID         string
	Material      string
	Description   string
	Qty           float64
	HasDimensions *bool
	Length        string
	Width         string
	Rate          float64
	Cgst          float64
	Sgst          float64
	Igst          float64
	Discount      float64
	Queue         string
	EntryID       string // "" when not converted
}

// Channel is one alert channel's stored state (created or done) - only the fields the
// derivation logic reads. The dates (sentAt/statusAt) are added by the caller, which formats
// them, so they stay out of this pure package.
type Channel struct {
	Status    string
	Error     string
	Count     int
	RowIDs    []string
	Signature string
}

// Job is the derivation input.
type Job struct {
	Unlocked bool
	Rows     []Row
	Created  Channel
	Done     Channel
}

// --- invoice state (Helpers/JobInvoiceState.js) ---

type InvoiceState struct {
	State         string // "none" | "partial" | "invoiced"
	InvoicedRows  int
	TotalRows     int
	ConvertedRows int
}

// DeriveInvoiceState mirrors deriveInvoiceState: a job is fully invoiced only when every row is
// (converted rows never invoiced don't count as done), partial when some are, none otherwise.
func DeriveInvoiceState(rows []Row, issued map[string]bool) InvoiceState {
	converted, invoiced := 0, 0
	for _, r := range rows {
		if r.EntryID != "" {
			converted++
			if issued[r.EntryID] {
				invoiced++
			}
		}
	}
	state := "none"
	if len(rows) > 0 && invoiced > 0 {
		if invoiced == len(rows) {
			state = "invoiced"
		} else {
			state = "partial"
		}
	}
	return InvoiceState{State: state, InvoicedRows: invoiced, TotalRows: len(rows), ConvertedRows: converted}
}

// IsReadyForInvoice: every row Done and nothing invoiced yet.
func IsReadyForInvoice(rows []Row, state string) bool {
	if len(rows) == 0 || state != "none" {
		return false
	}
	for _, r := range rows {
		if r.Queue != doneStage {
			return false
		}
	}
	return true
}

// RowInvoiced reports whether a row's entry is currently on an invoice.
func RowInvoiced(r Row, issued map[string]bool) bool {
	return r.EntryID != "" && issued[r.EntryID]
}

// --- lock (Helpers/JobLock.js) ---

type Lock struct {
	Invoiced      bool `json:"invoiced"`
	Unlocked      bool `json:"unlocked"`
	CanEditValues bool `json:"canEditValues"`
	CanEditQueue  bool `json:"canEditQueue"`
	CanDeleteJob  bool `json:"canDeleteJob"`
	CanDeleteRow  bool `json:"canDeleteRow"`
}

func LockState(unlocked, invoiced bool) Lock {
	return Lock{
		Invoiced: invoiced, Unlocked: unlocked,
		CanEditValues: !invoiced || unlocked,
		CanEditQueue:  !invoiced || unlocked,
		CanDeleteJob:  !invoiced,
		CanDeleteRow:  !invoiced || unlocked,
	}
}

// --- alert signatures & state (Helpers/JobAlertState.js) ---

// jsNum renders a number the way JavaScript's String(n) does for the magnitudes money and
// quantities reach here: an integer without a decimal point, a fraction in shortest form.
func jsNum(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }

// money is Number.isFinite(n) ? String(n) : "0" - a stable string so a numeric field and a
// form-string field hash the same and an absent field reads as 0.
func money(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "0"
	}
	return jsNum(v)
}

// The separators Node uses are ASCII control chars, chosen so no field value can contain one:
// fields within a row join with US (0x1F), rows join with RS (0x1E). They render invisibly, so
// the source reads like join("") - getting this wrong changes every signature.
const (
	unitSep   = "\x1f" // between a row's fields
	recordSep = "\x1e" // between rows
)

func rowFingerprint(r Row) string {
	byQty := ""
	if r.HasDimensions != nil && !*r.HasDimensions {
		byQty = "byQty" // appended ONLY when false, never as a plain field
	}
	fields := []string{
		r.ID, r.RowID, r.Material, r.Description, jsNum(r.Qty), byQty,
		r.Length, r.Width, jsNum(r.Rate), money(r.Cgst), money(r.Sgst), money(r.Igst), money(r.Discount),
	}
	return strings.Join(fields, unitSep)
}

// RowSignature is sha1 of the row fingerprints, sorted then joined with RS - identical to Node's.
func RowSignature(rows []Row) string {
	parts := make([]string, 0, len(rows))
	for _, r := range rows {
		parts = append(parts, rowFingerprint(r))
	}
	sort.Strings(parts)
	sum := sha1.Sum([]byte(strings.Join(parts, recordSep)))
	return hex.EncodeToString(sum[:])
}

func doneRowIDs(rows []Row) []string {
	out := []string{}
	for _, r := range rows {
		if r.Queue == doneStage {
			out = append(out, r.ID)
		}
	}
	sort.Strings(out)
	return out
}

// AlertState is one channel's derived state, matching JobAlertState.alertState(job, kind). The
// pure logic fields; the caller adds sentAt/statusAt (dates) when it builds the response.
type AlertState struct {
	SentBefore  bool
	Count       int
	Status      string
	Error       string
	CanSend     bool
	IsUpdate    bool
	Changed     bool
	Signature   string
	PendingRows int
	// DoneRowIDs is only set on the done channel.
	DoneRowIDs []string
}

// Alert computes a channel's state. kind is "created" or "done".
func Alert(job Job, kind string) AlertState {
	ch := job.Created
	if kind == "done" {
		ch = job.Done
	}
	base := AlertState{
		SentBefore: ch.Count > 0,
		Count:      ch.Count,
		Status:     ch.Status,
		Error:      ch.Error,
		Signature:  RowSignature(job.Rows),
	}
	if kind == "created" {
		sentBefore := ch.Count > 0
		changed := sentBefore && ch.Signature != base.Signature
		blocked := sentBefore && !changed && inFlightOrArrived[ch.Status]
		base.CanSend = !blocked
		base.IsUpdate = sentBefore
		base.Changed = changed
		base.PendingRows = 0
		return base
	}
	// done
	done := doneRowIDs(job.Rows)
	told := map[string]bool{}
	for _, id := range ch.RowIDs {
		told[id] = true
	}
	pending := 0
	for _, id := range done {
		if !told[id] {
			pending++
		}
	}
	everyDone := len(job.Rows) > 0 && len(done) == len(job.Rows)
	base.CanSend = everyDone && pending > 0
	base.IsUpdate = ch.Count > 0
	base.Changed = pending > 0
	base.PendingRows = pending
	base.DoneRowIDs = done
	return base
}

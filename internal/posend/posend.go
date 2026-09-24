// Package posend ports Helpers/PoSend.js - where a purchase order stands with its supplier. Nothing
// stores a status; the four states derive from the stored send fingerprint (as sent) plus a count,
// so the screen can never say "Sent" about a version the supplier never saw. Pure.
package posend

import "time"

const (
	StatusCreated    = "Created"
	StatusSent       = "Sent"
	StatusModified   = "Modified"
	StatusSentUpdate = "Sent Update"
)

// Input is the stored send state plus the order's current send fingerprint and gating flags.
type Input struct {
	Count         int
	StoredFP      string     // sent.fingerprint AS SENT
	CurrentFP     string     // sendFingerprint(po) now
	Approved      bool       // approval.state == "approved"
	Converted     bool       // purchaseInvoice_id set
	SentAt        *time.Time // sent channel
	ConfirmSentAt *time.Time // confirm channel
}

// State is the derived send state a PO row/detail carries.
type State struct {
	Status      string     `json:"status"`
	CanShare    bool       `json:"canShare"`
	CanConfirm  bool       `json:"canConfirm"`
	IsUpdate    bool       `json:"isUpdate"`
	SentAt      *time.Time `json:"sentAt"`
	Count       int        `json:"count"`
	ConfirmedAt *time.Time `json:"confirmedAt"`
}

// Derive is PoSend.sendState.
func Derive(in Input) State {
	matches := in.Count > 0 && in.StoredFP == in.CurrentFP

	status := StatusCreated
	switch {
	case in.Count == 0:
		status = StatusCreated
	case !matches:
		status = StatusModified
	case in.Count == 1:
		status = StatusSent
	default:
		status = StatusSentUpdate
	}

	// Approval gates both actions; a converted order is finished with.
	open := in.Approved && !in.Converted
	return State{
		Status:      status,
		CanShare:    open && (in.Count == 0 || !matches),
		CanConfirm:  open,
		IsUpdate:    in.Count > 0,
		SentAt:      in.SentAt,
		Count:       in.Count,
		ConfirmedAt: in.ConfirmSentAt,
	}
}

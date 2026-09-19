package store

import "strings"

// QueueStages is Model/Job.QUEUE_STAGES: the ordered job/row queue pipeline. "Completing" a
// stage advances to the next; the last stage has nowhere further to go. It is also the
// DEFAULT_QUEUE_ORDER a company falls back to (Helpers/QueueOrder.js).
var QueueStages = []string{"Created", "Printing", "Ready-to-Pickup", "Done"}

// FirstStage and LastStage are the pinned ends of every pipeline (Helpers/QueueOrder.js): a new
// job starts at FirstStage and the customer alert fires on LastStage, so neither may move.
const (
	FirstStage = "Created"
	LastStage  = "Done"
)

// NormalizeQueueOrder is Helpers/QueueOrder.normalizeQueueOrder: it de-dupes (case-insensitively,
// trimmed), drops blanks, strips any First/Last the caller placed in the interior, and pins
// First at the front and Last at the back. Empty or all-garbage input falls back to QueueStages
// (matching Node's `!interior.length && !seen.size` guard) - but an input of only the two ends
// yields the two-stage pipeline, not the fallback.
func NormalizeQueueOrder(order []string) []string {
	seen := map[string]bool{}
	interior := make([]string, 0, len(order))
	for _, raw := range order {
		stage := strings.TrimSpace(raw)
		if stage == "" {
			continue
		}
		key := strings.ToLower(stage)
		if seen[key] {
			continue
		}
		seen[key] = true
		if key == strings.ToLower(FirstStage) || key == strings.ToLower(LastStage) {
			continue
		}
		interior = append(interior, stage)
	}
	if len(interior) == 0 && len(seen) == 0 {
		out := make([]string, len(QueueStages))
		copy(out, QueueStages)
		return out
	}
	out := make([]string, 0, len(interior)+2)
	out = append(out, FirstStage)
	out = append(out, interior...)
	out = append(out, LastStage)
	return out
}

// RowProgressStates is Model/Job.ROW_PROGRESS_STATES: the per-row progress values.
var RowProgressStates = []string{"Assign", "In Progress", "Complete"}

// nextStage returns the stage after cur and true, or ("", false) if cur is last/unknown.
func nextStage(cur string) (string, bool) {
	for i, s := range QueueStages {
		if s == cur {
			if i < len(QueueStages)-1 {
				return QueueStages[i+1], true
			}
			return "", false
		}
	}
	return "", false
}

// JobTxStatus is the common outcome of a job/row lifecycle transition.
type JobTxStatus int

const (
	JobTxOK            JobTxStatus = iota
	JobTxJobNotFound               // no such job for this owner+company
	JobTxRowNotFound               // the row_id is not on the job
	JobTxNeedsAssignee             // progress cannot advance without an assignee
	JobTxLocked                    // the invoice lock forbids this change (refusedByLock)
)

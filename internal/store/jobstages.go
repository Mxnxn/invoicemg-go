package store

// QueueStages is Model/Job.QUEUE_STAGES: the ordered job/row queue pipeline. "Completing" a
// stage advances to the next; the last stage has nowhere further to go.
var QueueStages = []string{"Created", "Printing", "Ready-to-Pickup", "Done"}

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
)

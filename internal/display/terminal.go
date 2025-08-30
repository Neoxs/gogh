package display

// ExecutionStatus represents the current state of a workflow component
type ExecutionStatus string

const (
	StatusPending ExecutionStatus = "pending"
	StatusRunning ExecutionStatus = "running"
	StatusSuccess ExecutionStatus = "success"
	StatusFailure ExecutionStatus = "failure"
	StatusSkipped ExecutionStatus = "skipped"
)

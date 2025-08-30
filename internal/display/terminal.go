package display

import "time"

// ExecutionStatus represents the current state of a workflow component
type ExecutionStatus string

const (
	StatusPending ExecutionStatus = "pending"
	StatusRunning ExecutionStatus = "running"
	StatusSuccess ExecutionStatus = "success"
	StatusFailure ExecutionStatus = "failure"
	StatusSkipped ExecutionStatus = "skipped"
)

// WorkflowState holds the minimal state needed for display
type WorkflowState struct {
	Name      string
	Status    ExecutionStatus
	StartTime time.Time
	Jobs      map[string]*JobState
	LogPath   string // Path to detailed logs
}

// JobState holds the current state of a job execution
type JobState struct {
	ID        string
	Status    ExecutionStatus
	StartTime time.Time
	EndTime   time.Time
	Steps     []*StepState
}

// StepState holds the current state of a step execution
type StepState struct {
	Name      string
	Status    ExecutionStatus
	StartTime time.Time
	EndTime   time.Time
}

// TerminalDisplay handles real-time workflow status display
type TerminalDisplay struct {
	lastRender time.Time
}

// NewTerminalDisplay creates a new terminal display manager
func NewTerminalDisplay() *TerminalDisplay {
	return &TerminalDisplay{}
}

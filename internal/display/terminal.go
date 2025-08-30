package display

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"
)

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

// UpdateWorkflowState renders the current workflow state to terminal
func (td *TerminalDisplay) UpdateWorkflowState(state *WorkflowState) {
	td.clearScreen()
	td.renderWorkflowTree(state)
	td.lastRender = time.Now()
}

// ShowWorkflowComplete displays final completion status
func (td *TerminalDisplay) ShowWorkflowComplete(state *WorkflowState, totalDuration time.Duration) {
	td.clearScreen()
	td.renderWorkflowTree(state)
	fmt.Printf("\n🎉 Workflow completed successfully in %v\n", totalDuration)
	fmt.Printf("📁 Detailed logs saved to: %s\n", state.LogPath)
}

// ShowWorkflowError displays error status
func (td *TerminalDisplay) ShowWorkflowError(state *WorkflowState, err error) {
	td.clearScreen()
	td.renderWorkflowTree(state)
	fmt.Printf("\n❌ Workflow failed: %v\n", err)
	fmt.Printf("📁 Logs available at: %s\n", state.LogPath)
}

// renderWorkflowTree draws the hierarchical tree view
func (td *TerminalDisplay) renderWorkflowTree(state *WorkflowState) {
	// Workflow header
	duration := td.formatDuration(time.Since(state.StartTime))
	statusIcon := td.getStatusIcon(state.Status)

	fmt.Printf("%s Workflow: %s", statusIcon, state.Name)
	if state.Status == StatusRunning {
		fmt.Printf(" (%s)", duration)
	} else if state.Status == StatusSuccess || state.Status == StatusFailure {
		fmt.Printf(" (%s)", duration)
	}
	fmt.Println()

	// Render jobs in execution order (maintain order for consistent display)
	jobIDs := td.getSortedJobIDs(state.Jobs)

	for i, jobID := range jobIDs {
		job := state.Jobs[jobID]
		isLast := i == len(jobIDs)-1
		td.renderJob(job, isLast)
	}

	// Show current time for context
	fmt.Printf("\n⏰ Last updated: %s", time.Now().Format("15:04:05"))
}

// renderJob draws a single job and its steps
func (td *TerminalDisplay) renderJob(job *JobState, isLastJob bool) {
	// Job line
	jobPrefix := "├──"
	stepPrefix := "│   "
	if isLastJob {
		jobPrefix = "└──"
		stepPrefix = "    "
	}

	statusIcon := td.getStatusIcon(job.Status)
	jobDuration := td.getJobDuration(job)

	fmt.Printf("%s %s %s", jobPrefix, statusIcon, job.ID)
	if jobDuration != "" {
		fmt.Printf(" (%s)", jobDuration)
	}
	fmt.Println()

	// Render steps
	for i, step := range job.Steps {
		isLastStep := i == len(job.Steps)-1
		td.renderStep(step, stepPrefix, isLastStep)
	}
}

// renderStep draws a single step
func (td *TerminalDisplay) renderStep(step *StepState, parentPrefix string, isLastStep bool) {
	stepIcon := "├──"
	if isLastStep {
		stepIcon = "└──"
	}

	statusIcon := td.getStatusIcon(step.Status)
	stepDuration := td.getStepDuration(step)

	fmt.Printf("%s%s %s %s", parentPrefix, stepIcon, statusIcon, step.Name)
	if stepDuration != "" {
		fmt.Printf(" (%s)", stepDuration)
	}
	fmt.Println()
}

// Helper methods

func (td *TerminalDisplay) getStatusIcon(status ExecutionStatus) string {
	switch status {
	case StatusPending:
		return "⏳"
	case StatusRunning:
		return "🔄"
	case StatusSuccess:
		return "✅"
	case StatusFailure:
		return "❌"
	case StatusSkipped:
		return "⏭️"
	default:
		return "❓"
	}
}

func (td *TerminalDisplay) getJobDuration(job *JobState) string {
	switch job.Status {
	case StatusRunning:
		return td.formatDuration(time.Since(job.StartTime))
	case StatusSuccess, StatusFailure:
		if !job.EndTime.IsZero() {
			return td.formatDuration(job.EndTime.Sub(job.StartTime))
		}
		return td.formatDuration(time.Since(job.StartTime))
	default:
		return ""
	}
}

func (td *TerminalDisplay) getStepDuration(step *StepState) string {
	switch step.Status {
	case StatusRunning:
		return td.formatDuration(time.Since(step.StartTime))
	case StatusSuccess, StatusFailure:
		if !step.EndTime.IsZero() {
			return td.formatDuration(step.EndTime.Sub(step.StartTime))
		}
		return td.formatDuration(time.Since(step.StartTime))
	default:
		return ""
	}
}

func (td *TerminalDisplay) formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.0fms", float64(d.Nanoseconds())/1e6)
	} else if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	} else {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
}

func (td *TerminalDisplay) getSortedJobIDs(jobs map[string]*JobState) []string {
	// Simple approach: sort by start time (jobs that started first appear first)
	type jobEntry struct {
		id        string
		startTime time.Time
	}

	var entries []jobEntry
	for id, job := range jobs {
		entries = append(entries, jobEntry{id: id, startTime: job.StartTime})
	}

	// Sort by start time
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].startTime.After(entries[j].startTime) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	var result []string
	for _, entry := range entries {
		result = append(result, entry.id)
	}
	return result
}

func (td *TerminalDisplay) clearScreen() {
	switch runtime.GOOS {
	case "linux", "darwin":
		cmd := exec.Command("clear")
		cmd.Stdout = os.Stdout
		cmd.Run()
	case "windows":
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		cmd.Run()
	}
}

// Utility functions for creating state objects

// NewWorkflowState creates a new workflow state for display
func NewWorkflowState(name, logPath string) *WorkflowState {
	return &WorkflowState{
		Name:      name,
		Status:    StatusRunning,
		StartTime: time.Now(),
		Jobs:      make(map[string]*JobState),
		LogPath:   logPath,
	}
}

// NewJobState creates a new job state
func NewJobState(jobID string) *JobState {
	return &JobState{
		ID:        jobID,
		Status:    StatusPending,
		StartTime: time.Time{}, // Will be set when job actually starts
		Steps:     make([]*StepState, 0),
	}
}

// NewStepState creates a new step state
func NewStepState(stepName string) *StepState {
	return &StepState{
		Name:      stepName,
		Status:    StatusPending,
		StartTime: time.Time{},
	}
}

// State update methods

// UpdateJobStatus updates a job's status and timing
func (ws *WorkflowState) UpdateJobStatus(jobID string, status ExecutionStatus) {
	if job, exists := ws.Jobs[jobID]; exists {
		job.Status = status
		if status == StatusRunning && job.StartTime.IsZero() {
			job.StartTime = time.Now()
		} else if (status == StatusSuccess || status == StatusFailure) && job.EndTime.IsZero() {
			job.EndTime = time.Now()
		}
	}
}

// UpdateStepStatus updates a step's status and timing
func (ws *WorkflowState) UpdateStepStatus(jobID, stepName string, status ExecutionStatus) {
	if job, exists := ws.Jobs[jobID]; exists {
		for _, step := range job.Steps {
			if step.Name == stepName {
				step.Status = status
				if status == StatusRunning && step.StartTime.IsZero() {
					step.StartTime = time.Now()
				} else if (status == StatusSuccess || status == StatusFailure) && step.EndTime.IsZero() {
					step.EndTime = time.Now()
				}
				return
			}
		}
	}
}

// AddJobStep adds a new step to a job
func (ws *WorkflowState) AddJobStep(jobID, stepName string) {
	if job, exists := ws.Jobs[jobID]; exists {
		step := NewStepState(stepName)
		job.Steps = append(job.Steps, step)
	}
}

package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// WorkflowLogger manages logging for an entire workflow execution
type WorkflowLogger struct {
	workflowFile *os.File
	jobLoggers   map[string]*JobLogger
	basePath     string
	mu           sync.RWMutex
}

// JobLogger handles logging for a specific job
type JobLogger struct {
	jobFile *os.File
	jobID   string
	mu      sync.Mutex
}

// LogLevel represents different types of log entries
type LogLevel string

const (
	LogInfo    LogLevel = "INFO"
	LogWarning LogLevel = "WARNING"
	LogError   LogLevel = "ERROR"
	LogDebug   LogLevel = "DEBUG"
)

// NewWorkflowLogger creates a new workflow logger with organized file structure
func NewWorkflowLogger(workflowName, projectDir string) (*WorkflowLogger, error) {
	timestamp := time.Now().Format("2006-01-02-15-04-05")
	basePath := filepath.Join(projectDir, "gogh-logs", fmt.Sprintf("workflow-%s", timestamp))

	// Create logs directory
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Create main workflow log file
	workflowFile, err := os.Create(filepath.Join(basePath, "workflow.log"))
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow log file: %w", err)
	}

	logger := &WorkflowLogger{
		workflowFile: workflowFile,
		jobLoggers:   make(map[string]*JobLogger),
		basePath:     basePath,
	}

	logger.logWorkflowHeader(workflowName)
	return logger, nil
}

// GetJobLogger returns or creates a logger for a specific job
func (wl *WorkflowLogger) GetJobLogger(jobID string) (*JobLogger, error) {
	wl.mu.RLock()
	if logger, exists := wl.jobLoggers[jobID]; exists {
		wl.mu.RUnlock()
		return logger, nil
	}
	wl.mu.RUnlock()

	wl.mu.Lock()
	defer wl.mu.Unlock()

	// Double-check pattern
	if logger, exists := wl.jobLoggers[jobID]; exists {
		return logger, nil
	}

	timestamp := time.Now().Format("2006-01-02-15-04-05")
	jobLogFile := filepath.Join(wl.basePath, fmt.Sprintf("%s-%s.log", jobID, timestamp))

	jobFile, err := os.Create(jobLogFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create job log file: %w", err)
	}

	jobLogger := &JobLogger{
		jobFile: jobFile,
		jobID:   jobID,
	}

	wl.jobLoggers[jobID] = jobLogger
	return jobLogger, nil
}

// LogWorkflowStart logs the beginning of workflow execution
func (wl *WorkflowLogger) LogWorkflowStart(workflowName string) {
	wl.writeWorkflowLog("##[group]Starting workflow execution")
	wl.writeWorkflowLog(fmt.Sprintf("Workflow: %s", workflowName))
	wl.writeWorkflowLog("##[endgroup]")
}

// LogWorkflowComplete logs successful workflow completion
func (wl *WorkflowLogger) LogWorkflowComplete(duration time.Duration) {
	wl.writeWorkflowLog("##[group]Workflow completed successfully")
	wl.writeWorkflowLog(fmt.Sprintf("Total duration: %v", duration))
	wl.writeWorkflowLog("##[endgroup]")
}

// LogWorkflowError logs workflow-level errors
func (wl *WorkflowLogger) LogWorkflowError(err error) {
	wl.writeWorkflowLog("##[error]Workflow failed")
	wl.writeWorkflowLog(fmt.Sprintf("Error: %v", err))
}

// LogExecutionPlan logs the calculated job execution order
func (wl *WorkflowLogger) LogExecutionPlan(executionOrder []string) {
	wl.writeWorkflowLog("##[group]Execution Plan")
	wl.writeWorkflowLog(fmt.Sprintf("Job execution order: %v", executionOrder))
	wl.writeWorkflowLog("##[endgroup]")
}

// JobLogger methods

// LogJobStart logs the beginning of a job
func (jl *JobLogger) LogJobStart(jobID, runsOn string) {
	jl.writeJobLog("##[group]Job Setup")
	jl.writeJobLog(fmt.Sprintf("Job ID: %s", jobID))
	jl.writeJobLog(fmt.Sprintf("Runner: %s", runsOn))
	jl.writeJobLog("##[endgroup]")
}

// LogContainerStart logs Docker container creation
func (jl *JobLogger) LogContainerStart(image, containerID string) {
	jl.writeJobLog("##[group]Container Setup")
	jl.writeJobLog(fmt.Sprintf("Docker image: %s", image))
	jl.writeJobLog(fmt.Sprintf("Container ID: %s", containerID))
	jl.writeJobLog("##[endgroup]")
}

// LogStepStart logs the beginning of a workflow step
func (jl *JobLogger) LogStepStart(stepName, command string) {
	jl.writeJobLog(fmt.Sprintf("##[group]Run %s", stepName))
	if command != "" {
		jl.writeJobLog(command)
	}
	jl.writeJobLog("##[endgroup]")
}

// LogStepOutput logs real-time output from Docker containers
func (jl *JobLogger) LogStepOutput(line string) {
	jl.writeJobLog(line)
}

// LogStepComplete logs step completion with timing
func (jl *JobLogger) LogStepComplete(stepName string, duration time.Duration, exitCode int) {
	if exitCode == 0 {
		jl.writeJobLog(fmt.Sprintf("##[section]Step '%s' completed successfully in %v", stepName, duration))
	} else {
		jl.writeJobLog(fmt.Sprintf("##[error]Step '%s' failed in %v (exit code: %d)", stepName, duration, exitCode))
	}
}

// LogJobComplete logs job completion
func (jl *JobLogger) LogJobComplete(jobID string, duration time.Duration) {
	jl.writeJobLog("##[group]Job Summary")
	jl.writeJobLog(fmt.Sprintf("Job '%s' completed successfully", jobID))
	jl.writeJobLog(fmt.Sprintf("Duration: %v", duration))
	jl.writeJobLog("##[endgroup]")
}

// LogJobError logs job-level errors
func (jl *JobLogger) LogJobError(jobID string, err error) {
	jl.writeJobLog(fmt.Sprintf("##[error]Job '%s' failed", jobID))
	jl.writeJobLog(fmt.Sprintf("Error: %v", err))
}

// Private helper methods

func (wl *WorkflowLogger) logWorkflowHeader(workflowName string) {
	header := fmt.Sprintf(`
==============================================
GoGH - GitHub Actions Local Runner
==============================================
Workflow: %s
Started:  %s
==============================================
`, workflowName, time.Now().Format("2006-01-02 15:04:05 MST"))

	wl.workflowFile.WriteString(header)
}

func (wl *WorkflowLogger) writeWorkflowLog(message string) {
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.0000000Z")
	line := fmt.Sprintf("%s %s\n", timestamp, message)

	if wl.workflowFile != nil {
		wl.workflowFile.WriteString(line)
		wl.workflowFile.Sync() // Force write to disk
	}
}

func (jl *JobLogger) writeJobLog(message string) {
	jl.mu.Lock()
	defer jl.mu.Unlock()

	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.0000000Z")
	line := fmt.Sprintf("%s %s\n", timestamp, message)

	if jl.jobFile != nil {
		jl.jobFile.WriteString(line)
		jl.jobFile.Sync() // Force write to disk immediately
	}
}

// Close properly closes all log files
func (wl *WorkflowLogger) Close() error {
	wl.mu.Lock()
	defer wl.mu.Unlock()

	// Close all job loggers
	for _, jobLogger := range wl.jobLoggers {
		if err := jobLogger.Close(); err != nil {
			fmt.Printf("Warning: failed to close job logger: %v\n", err)
		}
	}

	// Close workflow log
	if wl.workflowFile != nil {
		wl.writeWorkflowLog("=== Workflow logging completed ===")
		return wl.workflowFile.Close()
	}

	return nil
}

// Close closes the job logger
func (jl *JobLogger) Close() error {
	jl.mu.Lock()
	defer jl.mu.Unlock()

	if jl.jobFile != nil {
		jl.writeJobLog(fmt.Sprintf("=== Job '%s' logging completed ===", jl.jobID))
		return jl.jobFile.Close()
	}

	return nil
}

// GetLogPath returns the base path where logs are stored
func (wl *WorkflowLogger) GetLogPath() string {
	return wl.basePath
}

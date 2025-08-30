package executor

import (
	"fmt"
	"time"

	"github.com/Neoxs/gogh/container"
	"github.com/Neoxs/gogh/internal/display"
	"github.com/Neoxs/gogh/internal/logging"
	"github.com/Neoxs/gogh/internal/workflow"
)

// WorkflowExecutor orchestrates the execution of workflows
type WorkflowExecutor struct {
	workflowDef   *workflow.WorkflowDefinition
	projectDir    string
	logger        *logging.WorkflowLogger
	display       *display.TerminalDisplay
	workflowState *display.WorkflowState
	startTime     time.Time
}

// NewWorkflowExecutor creates a new workflow executor with logging and display
func NewWorkflowExecutor(workflowDef *workflow.WorkflowDefinition, projectDir string) (*WorkflowExecutor, error) {
	// Create workflow logger
	logger, err := logging.NewWorkflowLogger(workflowDef.Name, projectDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow logger: %w", err)
	}

	// Create terminal display
	terminalDisplay := display.NewTerminalDisplay()

	// Create workflow state for display
	workflowState := display.NewWorkflowState(workflowDef.Name, logger.GetLogPath())

	return &WorkflowExecutor{
		workflowDef:   workflowDef,
		projectDir:    projectDir,
		logger:        logger,
		display:       terminalDisplay,
		workflowState: workflowState,
		startTime:     time.Now(),
	}, nil
}

// Execute runs the entire workflow
func (we *WorkflowExecutor) Execute() error {
	// Ensure cleanup
	defer we.logger.Close()

	// Log and display workflow start
	we.logger.LogWorkflowStart(we.workflowDef.Name)
	we.display.UpdateWorkflowState(we.workflowState)

	// Get execution order
	executionOrder, err := we.workflowDef.BuildExecutionPlan()
	if err != nil {
		we.logger.LogWorkflowError(err)
		we.display.ShowWorkflowError(we.workflowState, err)
		return fmt.Errorf("failed to build execution plan: %w", err)
	}

	// Log execution plan
	we.logger.LogExecutionPlan(executionOrder)

	// Initialize job states for display
	for _, jobID := range executionOrder {
		jobState := display.NewJobState(jobID)

		// Pre-populate steps for display
		if job, exists := we.workflowDef.Jobs[jobID]; exists {
			for i, step := range job.Steps {
				stepName := step.Name
				if stepName == "" {
					stepName = fmt.Sprintf("Step %d", i+1)
				}
				jobState.Steps = append(jobState.Steps, display.NewStepState(stepName))
			}
		}

		we.workflowState.Jobs[jobID] = jobState
	}

	// Update display with initial state
	we.display.UpdateWorkflowState(we.workflowState)

	// Execute jobs in sequence (for MVP - no parallelization yet)
	for _, jobID := range executionOrder {
		if err := we.executeJob(jobID); err != nil {
			we.workflowState.Status = display.StatusFailure
			we.logger.LogWorkflowError(err)
			we.display.ShowWorkflowError(we.workflowState, err)
			return fmt.Errorf("job %s failed: %w", jobID, err)
		}
	}

	// Workflow completed successfully
	totalDuration := time.Since(we.startTime)
	we.workflowState.Status = display.StatusSuccess
	we.logger.LogWorkflowComplete(totalDuration)
	we.display.ShowWorkflowComplete(we.workflowState, totalDuration)

	return nil
}

// executeJob runs a single job with integrated logging and display
func (we *WorkflowExecutor) executeJob(jobID string) error {
	job, exists := we.workflowDef.Jobs[jobID]
	if !exists {
		return fmt.Errorf("job %s not found", jobID)
	}

	// Get job logger
	jobLogger, err := we.logger.GetJobLogger(jobID)
	if err != nil {
		return fmt.Errorf("failed to create job logger: %w", err)
	}

	// Update job status to running
	we.workflowState.UpdateJobStatus(jobID, display.StatusRunning)
	we.display.UpdateWorkflowState(we.workflowState)

	// Log job start
	jobLogger.LogJobStart(jobID, job.RunsOn)

	jobStartTime := time.Now()

	// Create job runner
	jobRunner := container.NewJobRunner(job.RunsOn, we.projectDir)

	// Start container
	if err := jobRunner.Start(); err != nil {
		we.workflowState.UpdateJobStatus(jobID, display.StatusFailure)
		jobLogger.LogJobError(jobID, err)
		we.display.UpdateWorkflowState(we.workflowState)
		return fmt.Errorf("failed to start job container: %w", err)
	}

	// Log container start
	jobLogger.LogContainerStart(jobRunner.GetImage(), jobRunner.GetContainerID())

	// Ensure cleanup
	defer func() {
		if err := jobRunner.Stop(); err != nil {
			jobLogger.LogJobError(jobID, fmt.Errorf("failed to stop container: %w", err))
		}
	}()

	// Execute all steps in sequence
	for i, step := range job.Steps {
		stepName := step.Name
		if stepName == "" {
			stepName = fmt.Sprintf("Step %d", i+1)
		}

		// Update step status to running
		we.workflowState.UpdateStepStatus(jobID, stepName, display.StatusRunning)
		we.display.UpdateWorkflowState(we.workflowState)

		// Log step start
		jobLogger.LogStepStart(stepName, step.Run)

		stepStartTime := time.Now()

		// Execute step (we'll need to modify this to pass the logger)
		result, err := jobRunner.RunStep(stepName, step.Run, jobLogger)

		stepDuration := time.Since(stepStartTime)

		if err != nil || !result.Success {
			// Step failed
			we.workflowState.UpdateStepStatus(jobID, stepName, display.StatusFailure)
			we.workflowState.UpdateJobStatus(jobID, display.StatusFailure)

			exitCode := 1
			if result != nil && result.Error != nil {
				exitCode = result.ExitCode
			}

			jobLogger.LogStepComplete(stepName, stepDuration, exitCode)
			jobLogger.LogJobError(jobID, err)
			we.display.UpdateWorkflowState(we.workflowState)

			return fmt.Errorf("step '%s' failed: %w", stepName, err)
		}

		// Step succeeded
		we.workflowState.UpdateStepStatus(jobID, stepName, display.StatusSuccess)
		jobLogger.LogStepComplete(stepName, stepDuration, 0)
		we.display.UpdateWorkflowState(we.workflowState)
	}

	// Job completed successfully
	jobDuration := time.Since(jobStartTime)
	we.workflowState.UpdateJobStatus(jobID, display.StatusSuccess)
	jobLogger.LogJobComplete(jobID, jobDuration)
	we.display.UpdateWorkflowState(we.workflowState)

	return nil
}

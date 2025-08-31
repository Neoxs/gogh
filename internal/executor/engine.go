package executor

import (
	"fmt"
	"time"

	"github.com/Neoxs/gogh/container"
	"github.com/Neoxs/gogh/internal/actions"
	"github.com/Neoxs/gogh/internal/display"
	"github.com/Neoxs/gogh/internal/environment"
	"github.com/Neoxs/gogh/internal/logging"
	"github.com/Neoxs/gogh/internal/workflow"
)

// WorkflowExecutor orchestrates the execution of workflows
type WorkflowExecutor struct {
	workflowDef    *workflow.WorkflowDefinition
	projectDir     string
	logger         *logging.WorkflowLogger
	display        *display.TerminalDisplay
	workflowState  *display.WorkflowState
	actionResolver *actions.ActionResolver
	envManager     *environment.EnvironmentManager
	startTime      time.Time
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

	// Create action resolver
	actionResolver := actions.NewActionResolver(projectDir)

	// Create environment manager
	envManager := environment.NewEnvironmentManager(workflowDef, projectDir)

	return &WorkflowExecutor{
		workflowDef:    workflowDef,
		projectDir:     projectDir,
		logger:         logger,
		display:        terminalDisplay,
		workflowState:  workflowState,
		actionResolver: actionResolver,
		envManager:     envManager,
		startTime:      time.Now(),
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

	// Create GitHub context for actions
	githubContext := we.createGitHubContext()

	// Execute all steps in sequence
	for i, step := range job.Steps {
		stepName := step.Name
		if stepName == "" {
			stepName = fmt.Sprintf("Step %d", i+1)
		}

		// Update step status to running
		we.workflowState.UpdateStepStatus(jobID, stepName, display.StatusRunning)
		we.display.UpdateWorkflowState(we.workflowState)

		stepStartTime := time.Now()

		var stepError error
		var stepSuccess bool

		// Determine if this is an action or run step
		if step.Uses != "" {
			// Handle action step
			stepSuccess, stepError = we.executeActionStep(step, jobRunner, githubContext, jobLogger)
		} else if step.Run != "" {
			// Handle run step
			stepSuccess, stepError = we.executeRunStep(step, jobRunner, jobLogger)
		} else {
			stepError = fmt.Errorf("step has neither 'uses' nor 'run' specified")
			stepSuccess = false
		}

		stepDuration := time.Since(stepStartTime)

		if stepError != nil || !stepSuccess {
			// Step failed
			we.workflowState.UpdateStepStatus(jobID, stepName, display.StatusFailure)
			we.workflowState.UpdateJobStatus(jobID, display.StatusFailure)

			exitCode := 1
			jobLogger.LogStepComplete(stepName, stepDuration, exitCode)
			jobLogger.LogJobError(jobID, stepError)
			we.display.UpdateWorkflowState(we.workflowState)

			return fmt.Errorf("step '%s' failed: %w", stepName, stepError)
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

// executeActionStep handles uses: steps through the action system
func (we *WorkflowExecutor) executeActionStep(step workflow.StepDefinition, jobRunner *container.JobRunner, githubContext actions.GitHubContext, jobLogger *logging.JobLogger) (bool, error) {
	// Convert step inputs to string map
	inputs := make(map[string]string)
	if step.With != nil {
		for key, value := range step.With {
			inputs[key] = fmt.Sprintf("%v", value)
		}
	}

	// Create action context
	actionContext := &actions.ActionContext{
		ActionRef:    step.Uses,
		Inputs:       inputs,
		WorkspaceDir: "/workspace",
		ContainerID:  jobRunner.GetContainerID(),
		GitHub:       githubContext,
		Runner: actions.RunnerContext{
			OS:   "linux",
			Arch: "x64",
			Temp: "/tmp",
			Tool: "/opt/hostedtoolcache",
		},
	}

	// Resolve and execute action
	actionExecutor, err := we.actionResolver.ResolveAction(step.Uses, inputs, actionContext)
	if err != nil {
		jobLogger.LogStepOutput(fmt.Sprintf("Failed to resolve action: %v", err))
		return false, err
	}

	// Log action start
	jobLogger.LogStepStart(step.Name, fmt.Sprintf("uses: %s", step.Uses))

	// Execute action
	result, err := actionExecutor.Execute(actionContext, jobLogger)
	if err != nil {
		return false, err
	}

	if !result.Success {
		return false, result.Error
	}

	// TODO: Handle action outputs for future step dependencies
	return true, nil
}

// executeRunStep handles run: steps through the container system
func (we *WorkflowExecutor) executeRunStep(step workflow.StepDefinition, jobRunner *container.JobRunner, jobLogger *logging.JobLogger) (bool, error) {
	// Log step start
	jobLogger.LogStepStart(step.Name, step.Run)

	// Build env
	stepEnv := we.envManager.BuildStepEnvironment(step.Env)

	// Execute step using existing container logic
	result, err := jobRunner.RunStepInEnvironment(step.Name, step.Run, stepEnv, jobLogger)
	if err != nil || !result.Success {
		return false, err
	}

	return true, nil
}

// createGitHubContext simulates GitHub's runtime context
func (we *WorkflowExecutor) createGitHubContext() actions.GitHubContext {
	// Try to get git information
	repository := we.getGitRepository()
	sha := we.getGitSHA()
	ref := we.getGitRef()

	return actions.GitHubContext{
		Repository: repository,
		SHA:        sha,
		Ref:        ref,
		Workspace:  "/workspace",
		EventName:  "push", // Default to push event
	}
}

// Helper methods to simulate GitHub context
func (we *WorkflowExecutor) getGitRepository() string {
	// Try to get from git remote
	// For now, return a default - you could enhance this to actually parse git config
	return "user/repo"
}

func (we *WorkflowExecutor) getGitSHA() string {
	// Try to get current commit SHA
	// For now, return a placeholder - you could enhance this to run `git rev-parse HEAD`
	return "1234567890abcdef"
}

func (we *WorkflowExecutor) getGitRef() string {
	// Try to get current branch/tag
	// For now, return a default - you could enhance this to run `git branch --show-current`
	return "refs/heads/main"
}

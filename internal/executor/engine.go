package executor

import (
	"fmt"
	"strings"
	"time"

	"github.com/Neoxs/gogh/container"
	"github.com/Neoxs/gogh/internal/actions"
	"github.com/Neoxs/gogh/internal/display"
	"github.com/Neoxs/gogh/internal/environment"
	"github.com/Neoxs/gogh/internal/expressions"
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

// executeJob runs a single job with integrated logging, display, and environment
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

	// Configure environment manager for this job
	we.envManager.SetJobEnvironment(job.Env)

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

	// Handle job-level with: inputs if they exist
	if job.With != nil {
		jobLogger.LogStepOutput("Job-level inputs:")
		stepEnvironment := we.envManager.BuildStepEnvironment(nil) // No step-specific env

		for key, value := range job.With {
			rawValue := fmt.Sprintf("%v", value)
			expandedValue := we.expandInputVariables(rawValue, stepEnvironment)
			jobLogger.LogStepOutput(fmt.Sprintf("  %s: %s", key, expandedValue))
		}
	}

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

		// Build complete environment for this step
		stepEnv := we.envManager.BuildStepEnvironment(step.Env)

		var stepError error
		var stepSuccess bool

		// Determine step type and execute with environment
		if step.Uses != "" {
			// Handle action step
			stepSuccess, stepError = we.executeActionStep(step, jobRunner, stepEnv, jobLogger)
		} else if step.Run != "" {
			// Handle run step with full environment integration
			stepSuccess, stepError = we.executeRunStep(step, jobRunner, stepEnv, jobLogger)
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
func (we *WorkflowExecutor) executeActionStep(step workflow.StepDefinition, jobRunner *container.JobRunner, stepEnv map[string]string, jobLogger *logging.JobLogger) (bool, error) {
	// Build step environment first (needed for input expansion)
	stepEnvironment := we.envManager.BuildStepEnvironment(step.Env)

	// Convert step inputs to string map WITH environment variable expansion
	inputs := make(map[string]string)
	if step.With != nil {
		for key, value := range step.With {
			rawValue := fmt.Sprintf("%v", value)
			// Expand environment variables in the input value using expression evaluator
			expandedValue := we.expandInputVariables(rawValue, stepEnvironment)
			inputs[key] = expandedValue
		}
	}

	// Create GitHub context from environment manager
	githubCtx := we.envManager.GetGitHubContext()

	// Create action context with proper GitHub context
	actionContext := &actions.ActionContext{
		ActionRef:    step.Uses,
		Inputs:       inputs,
		WorkspaceDir: "/workspace",
		ContainerID:  jobRunner.GetContainerID(),
		GitHub: actions.GitHubContext{
			Repository: githubCtx.Repository,
			SHA:        githubCtx.SHA,
			Ref:        githubCtx.Ref,
			Workspace:  githubCtx.Workspace,
			EventName:  githubCtx.EventName,
			Actor:      githubCtx.Actor,
			RunID:      githubCtx.RunID,
			RunNumber:  githubCtx.RunNumber,
			Job:        "", // Actions don't need job context
			Action:     step.Uses,
			ActionPath: "",
		},
		Runner: actions.RunnerContext{
			OS:   "linux",
			Arch: "x64",
			Temp: "/tmp",
			Tool: "/opt/hostedtoolcache",
		},
	}

	// Log the expanded inputs for debugging
	jobLogger.LogStepOutput("Action inputs:")
	for key, value := range inputs {
		jobLogger.LogStepOutput(fmt.Sprintf("  %s: %s", key, value))
	}

	// Resolve and execute action
	actionExecutor, err := we.actionResolver.ResolveAction(step.Uses, inputs, actionContext)
	if err != nil {
		jobLogger.LogStepOutput(fmt.Sprintf("Failed to resolve action: %v", err))
		return false, err
	}

	// Log action start
	jobLogger.LogStepStart(step.Name, fmt.Sprintf("uses: %s", step.Uses))

	// Execute action (actions handle their own environment setup internally)
	result, err := actionExecutor.Execute(actionContext, jobLogger)
	if err != nil {
		return false, err
	}

	if !result.Success {
		return false, result.Error
	}

	return true, nil
}

// executeRunStep handles run: steps with full environment variable support
func (we *WorkflowExecutor) executeRunStep(step workflow.StepDefinition, jobRunner *container.JobRunner, stepEnv map[string]string, jobLogger *logging.JobLogger) (bool, error) {
	// Log step start
	jobLogger.LogStepStart(step.Name, step.Run)

	// This is the key integration: pass the complete environment to the container
	result, err := jobRunner.RunStep(step.Name, step.Run, stepEnv, jobLogger)
	if err != nil || !result.Success {
		return false, err
	}

	return true, nil
}

// expandInputVariables expands environment variables in action input values using expression evaluator
func (we *WorkflowExecutor) expandInputVariables(value string, environment map[string]string) string {
	// Create evaluation context
	githubCtx := we.envManager.GetGitHubContext()
	evalContext := &expressions.EvaluationContext{
		Github: expressions.GitHubContext{
			Repository: githubCtx.Repository,
			SHA:        githubCtx.SHA,
			Ref:        githubCtx.Ref,
			EventName:  githubCtx.EventName,
			Actor:      githubCtx.Actor,
			RunID:      githubCtx.RunID,
			RunNumber:  githubCtx.RunNumber,
			Workspace:  githubCtx.Workspace,
		},
		Env: environment,
		Job: expressions.JobContext{
			Status: "in_progress", // Could be made dynamic
		},
		Runner: expressions.RunnerContext{
			OS:   "Linux",
			Arch: "X64",
		},
		Secrets: make(map[string]string), // TODO: Add secrets support
	}

	// Create evaluator
	evaluator := expressions.NewExpressionEvaluator(evalContext)

	// Find and replace all ${{ ... }} expressions
	return we.replaceExpressions(value, evaluator)
}

// replaceExpressions finds and replaces all expressions in the input string
func (we *WorkflowExecutor) replaceExpressions(input string, evaluator *expressions.ExpressionEvaluator) string {
	result := input

	// Simple approach: find ${{ ... }} patterns and evaluate them
	// This handles multiple expressions in one string like "The ${{ github.event_name }} event triggered this step."
	for {
		start := strings.Index(result, "${{")
		if start == -1 {
			break
		}

		end := strings.Index(result[start:], "}}")
		if end == -1 {
			break
		}
		end = start + end + 2

		expression := result[start:end]
		evaluated, err := evaluator.Evaluate(expression)
		if err != nil {
			// Log error but continue with original expression
			// In production, you might want better error handling
			break
		}

		result = result[:start] + evaluated + result[end:]
	}

	return result
}

package executor

import (
	"fmt"

	"github.com/Neoxs/gogh/container"
	"github.com/Neoxs/gogh/internal/workflow"
)

// WorkflowExecutor orchestrates the execution of workflows
type WorkflowExecutor struct {
	workflowDef *workflow.WorkflowDefinition
	projectDir  string
}

// NewWorkflowExecutor creates a new workflow executor
func NewWorkflowExecutor(workflowDef *workflow.WorkflowDefinition, projectDir string) *WorkflowExecutor {
	return &WorkflowExecutor{
		workflowDef: workflowDef,
		projectDir:  projectDir,
	}
}

// Execute runs the entire workflow
func (we *WorkflowExecutor) Execute() error {
	fmt.Printf("🚀 Executing workflow: %s\n", we.workflowDef.Name)

	// Get execution order
	executionOrder, err := we.workflowDef.BuildExecutionPlan()
	if err != nil {
		return fmt.Errorf("failed to build execution plan: %w", err)
	}

	fmt.Printf("📋 Execution order: %v\n", executionOrder)

	// Execute jobs in sequence (for MVP - no parallelization yet)
	for _, jobID := range executionOrder {
		if err := we.executeJob(jobID); err != nil {
			return fmt.Errorf("job %s failed: %w", jobID, err)
		}
	}

	fmt.Printf("🎉 Workflow completed successfully!\n")
	return nil
}

// executeJob runs a single job
func (we *WorkflowExecutor) executeJob(jobID string) error {
	job, exists := we.workflowDef.Jobs[jobID]
	if !exists {
		return fmt.Errorf("job %s not found", jobID)
	}

	fmt.Printf("\n📦 Starting job: %s\n", jobID)

	// Create job runner
	jobRunner := container.NewJobRunner(job.RunsOn, we.projectDir)

	// Start container
	if err := jobRunner.Start(); err != nil {
		return fmt.Errorf("failed to start job container: %w", err)
	}

	// Ensure cleanup
	defer func() {
		if err := jobRunner.Stop(); err != nil {
			fmt.Printf("Warning: failed to stop container: %v\n", err)
		}
	}()

	// Execute all steps in sequence
	for i, step := range job.Steps {
		stepName := step.Name
		if stepName == "" {
			stepName = fmt.Sprintf("Step %d", i+1)
		}

		result, err := jobRunner.RunStep(stepName, step.Run)
		if err != nil || !result.Success {
			return fmt.Errorf("step '%s' failed: %w", stepName, err)
		}
	}

	fmt.Printf("✅ Job completed: %s\n", jobID)
	return nil
}

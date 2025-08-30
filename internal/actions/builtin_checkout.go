package actions

import (
	"fmt"
	"os/exec"

	"github.com/Neoxs/gogh/internal/logging"
)

// CheckoutAction implements actions/checkout functionality
type CheckoutAction struct{}

func (ca *CheckoutAction) GetName() string {
	return "actions/checkout"
}

func (ca *CheckoutAction) ValidateInputs(inputs map[string]string) error {
	// For now, we'll be permissive and accept any inputs
	// TODO: Please update later
	return nil
}

func (ca *CheckoutAction) Execute(ctx *ActionContext, jobLogger *logging.JobLogger) (*ActionResult, error) {
	jobLogger.LogStepOutput("Setting up workspace for checkout...")

	// Since we already mount the project directory, we mainly need to:
	// 1. Ensure we're in the right directory
	// 2. Set up GitHub environment variables
	// 3. Simulate what checkout normally does

	result := &ActionResult{
		Success: true,
		Outputs: make(map[string]string),
	}

	// Set checkout-specific environment variables in container
	envCommands := []string{
		fmt.Sprintf("export GITHUB_WORKSPACE=%s", ctx.GitHub.Workspace),
		fmt.Sprintf("export GITHUB_REPOSITORY=%s", ctx.GitHub.Repository),
		fmt.Sprintf("export GITHUB_SHA=%s", ctx.GitHub.SHA),
		fmt.Sprintf("export GITHUB_REF=%s", ctx.GitHub.Ref),
	}

	for _, envCmd := range envCommands {
		if err := ca.runInContainer(ctx.ContainerID, envCmd, jobLogger); err != nil {
			result.Success = false
			result.Error = fmt.Errorf("failed to set environment: %w", err)
			return result, err
		}
	}

	// Verify workspace is accessible
	checkCmd := fmt.Sprintf("ls -la %s", ctx.GitHub.Workspace)
	if err := ca.runInContainer(ctx.ContainerID, checkCmd, jobLogger); err != nil {
		jobLogger.LogStepOutput("Warning: Could not verify workspace contents")
	}

	// Set output (path where code was checked out)
	result.Outputs["path"] = ctx.GitHub.Workspace

	jobLogger.LogStepOutput("Checkout completed - workspace is ready")
	return result, nil
}

func (ca *CheckoutAction) runInContainer(containerID, command string, jobLogger *logging.JobLogger) error {
	cmd := exec.Command("docker", "exec", containerID, "bash", "-c", command)
	output, err := cmd.CombinedOutput()

	if len(output) > 0 {
		jobLogger.LogStepOutput(string(output))
	}

	return err
}

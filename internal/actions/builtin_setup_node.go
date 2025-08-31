package actions

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/Neoxs/gogh/internal/logging"
)

// SetupNodeAction implements actions/setup-node functionality
type SetupNodeAction struct{}

func (sna *SetupNodeAction) GetName() string {
	return "actions/setup-node"
}

func (sna *SetupNodeAction) ValidateInputs(inputs map[string]string) error {
	// node-version is the most important input
	if nodeVersion, exists := inputs["node-version"]; exists {
		if nodeVersion == "" {
			return fmt.Errorf("node-version cannot be empty")
		}
	}
	return nil
}

func (sna *SetupNodeAction) Execute(ctx *ActionContext, jobLogger *logging.JobLogger) (*ActionResult, error) {
	nodeVersion := ctx.Inputs["node-version"]
	if nodeVersion == "" {
		nodeVersion = "18" // Default to Node.js 18
	}

	jobLogger.LogStepOutput(fmt.Sprintf("Setting up Node.js %s", nodeVersion))

	result := &ActionResult{
		Success: true,
		Outputs: make(map[string]string),
	}

	// First, update package lists and install prerequisites
	prerequisiteCommands := []string{
		"apt-get update",
		"apt-get install -y curl ca-certificates gnupg",
	}

	for _, cmd := range prerequisiteCommands {
		jobLogger.LogStepOutput(fmt.Sprintf("Installing prerequisites: %s", cmd))
		if err := sna.runInContainer(ctx.ContainerID, cmd, jobLogger); err != nil {
			jobLogger.LogStepOutput(fmt.Sprintf("Warning: prerequisite command failed but continuing: %s", cmd))
			// Continue anyway - some might already be installed
		}
	}

	// Install Node.js using NodeSource repository
	installCommands := []string{
		fmt.Sprintf("curl -fsSL https://deb.nodesource.com/setup_%s.x | bash -", nodeVersion),
		"apt-get install -y nodejs",
	}

	for _, cmd := range installCommands {
		jobLogger.LogStepOutput(fmt.Sprintf("Running: %s", cmd))
		if err := sna.runInContainer(ctx.ContainerID, cmd, jobLogger); err != nil {
			result.Success = false
			result.Error = fmt.Errorf("failed to install Node.js: %w", err)
			return result, err
		}
	}

	// Verify installation and get versions
	nodeVersionOutput, err := sna.getCommandOutput(ctx.ContainerID, "node --version")
	if err != nil {
		result.Success = false
		result.Error = fmt.Errorf("Node.js installation verification failed: %w", err)
		return result, err
	}

	result.Outputs["node-version"] = strings.TrimSpace(nodeVersionOutput)
	jobLogger.LogStepOutput(fmt.Sprintf("Node.js installed: %s", result.Outputs["node-version"]))

	npmVersionOutput, err := sna.getCommandOutput(ctx.ContainerID, "npm --version")
	if err == nil {
		result.Outputs["npm-version"] = strings.TrimSpace(npmVersionOutput)
		jobLogger.LogStepOutput(fmt.Sprintf("npm installed: %s", result.Outputs["npm-version"]))
	}

	// Set up npm cache directory
	cacheSetupCmd := "mkdir -p /home/runner/.npm && npm config set cache /home/runner/.npm"
	sna.runInContainer(ctx.ContainerID, cacheSetupCmd, jobLogger)

	jobLogger.LogStepOutput("Node.js setup completed")
	return result, nil
}

func (sna *SetupNodeAction) runInContainer(containerID, command string, jobLogger *logging.JobLogger) error {
	cmd := exec.Command("docker", "exec", containerID, "bash", "-c", command)
	output, err := cmd.CombinedOutput()

	if len(output) > 0 {
		jobLogger.LogStepOutput(string(output))
	}

	return err
}

func (sna *SetupNodeAction) getCommandOutput(containerID, command string) (string, error) {
	cmd := exec.Command("docker", "exec", containerID, "bash", "-c", command)
	output, err := cmd.Output()
	return string(output), err
}

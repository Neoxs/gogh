package container

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Neoxs/gogh/internal/logging"
)

// JobRunner manages a Docker container for running GitHub Actions jobs
type JobRunner struct {
	containerID  string
	image        string
	workspaceDir string
	projectDir   string
	isRunning    bool
}

// NewJobRunner creates a new job runner for the specified image
func NewJobRunner(runsOn, projectDir string) *JobRunner {
	// Get absolute path of project directory
	absProjectDir, _ := filepath.Abs(projectDir)

	return &JobRunner{
		image:        mapRunnerToImage(runsOn),
		projectDir:   absProjectDir,
		workspaceDir: "/workspace", // Standard workspace inside container
		isRunning:    false,
	}
}

// mapRunnerToImage handles the most common GitHub Actions runner names
func mapRunnerToImage(runsOn string) string {
	switch runsOn {
	case "ubuntu-latest":
		return "ubuntu:latest"
	case "ubuntu-22.04":
		return "ubuntu:22.04"
	case "ubuntu-20.04":
		return "ubuntu:20.04"
	default:
		return runsOn // Pass through anything else (like node:18, python:3.11, etc.)
	}
}

// GetImage returns the Docker image being used
func (jr *JobRunner) GetImage() string {
	return jr.image
}

// GetContainerID returns the current container ID
func (jr *JobRunner) GetContainerID() string {
	return jr.containerID
}

// Start creates and starts the Docker container
func (jr *JobRunner) Start() error {
	if jr.isRunning {
		return fmt.Errorf("container already running")
	}

	// Docker run command with volume mounting
	args := []string{
		"run",
		"-d",                                                       // detached mode
		"--rm",                                                     // auto-remove when stopped
		"-v", fmt.Sprintf("%s:%s", jr.projectDir, jr.workspaceDir), // mount project
		"-w", jr.workspaceDir, // set working directory
		jr.image,
		"sleep", "3600", // keep container alive for 1 hour
	}

	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start container: %v\nOutput: %s", err, string(output))
	}

	jr.containerID = strings.TrimSpace(string(output))
	jr.isRunning = true

	return nil
}

// RunStep executes a single step command inside the container with logging and environment
func (jr *JobRunner) RunStep(stepName, command string, env map[string]string, jobLogger *logging.JobLogger) (*StepResult, error) {
	if !jr.isRunning {
		return nil, fmt.Errorf("container not running")
	}

	result := &StepResult{
		StepName:  stepName,
		Command:   command,
		StartTime: time.Now(),
	}

	// Build Docker exec command with environment variables
	args := []string{"exec"}

	// Add environment variables as -e flags
	for key, value := range env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	// Add container ID and command
	args = append(args, jr.containerID, "bash", "-c", command)

	cmd := exec.Command("docker", args...)

	// Capture both stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		result.Error = err
		return result, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		result.Error = err
		return result, err
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		result.Error = err
		return result, err
	}

	// Stream output directly to logger
	go jr.streamOutputToLogger(stdout, jobLogger)
	go jr.streamOutputToLogger(stderr, jobLogger)

	// Wait for command to complete
	err = cmd.Wait()
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// Get exit code
	result.ExitCode = 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				result.ExitCode = status.ExitStatus()
			} else {
				result.ExitCode = 1
			}
		} else {
			result.ExitCode = 1
		}
		result.Error = err
		result.Success = false
	} else {
		result.Success = true
	}

	return result, nil
}

// RunStepInEnvironment is a convenience method that runs a command with environment setup
func (jr *JobRunner) RunStepInEnvironment(stepName, command string, env map[string]string, jobLogger *logging.JobLogger) (*StepResult, error) {
	// Log environment variables (excluding sensitive ones)
	jr.logEnvironmentVariables(env, jobLogger)

	return jr.RunStep(stepName, command, env, jobLogger)
}

// logEnvironmentVariables logs environment setup (filtering sensitive data)
func (jr *JobRunner) logEnvironmentVariables(env map[string]string, jobLogger *logging.JobLogger) {
	if len(env) == 0 {
		return
	}

	jobLogger.LogStepOutput("Environment variables:")
	for key, value := range env {
		// Filter out potentially sensitive variables
		if jr.isSensitiveVar(key) {
			jobLogger.LogStepOutput(fmt.Sprintf("  %s=***", key))
		} else {
			jobLogger.LogStepOutput(fmt.Sprintf("  %s=%s", key, value))
		}
	}
}

// isSensitiveVar checks if a variable name suggests sensitive content
func (jr *JobRunner) isSensitiveVar(key string) bool {
	sensitivePatterns := []string{
		"TOKEN", "SECRET", "KEY", "PASSWORD", "PASS", "AUTH", "CREDENTIAL",
	}

	upperKey := strings.ToUpper(key)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(upperKey, pattern) {
			return true
		}
	}
	return false
}

// streamOutputToLogger reads from pipe and writes directly to job logger
func (jr *JobRunner) streamOutputToLogger(pipe io.ReadCloser, jobLogger *logging.JobLogger) {
	defer pipe.Close()

	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		line := scanner.Text()
		jobLogger.LogStepOutput(line)
	}
}

// Stop terminates the Docker container
func (jr *JobRunner) Stop() error {
	if !jr.isRunning || jr.containerID == "" {
		return nil
	}

	cmd := exec.Command("docker", "stop", jr.containerID)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop container: %v", err)
	}

	jr.isRunning = false
	jr.containerID = ""
	return nil
}

// StepResult contains the results of running a step
type StepResult struct {
	StepName  string
	Command   string
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	Success   bool
	ExitCode  int
	Error     error
}

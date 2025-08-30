package container

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
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
		image:        mapRunnerToImage(runsOn), // Simple mapping for common cases
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

// Start creates and starts the Docker container
func (jr *JobRunner) Start() error {
	if jr.isRunning {
		return fmt.Errorf("container already running")
	}

	fmt.Printf("🐳 Starting container with image: %s\n", jr.image)

	// Docker run command with volume mounting
	// -v mounts local project directory into container at /workspace
	// -w sets working directory inside container
	// -d runs in detached mode so container stays alive
	// --rm automatically removes container when it stops
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

	fmt.Printf("✅ Container started: %s\n", jr.containerID[:12]) // show short ID
	return nil
}

// RunStep executes a single step command inside the container
func (jr *JobRunner) RunStep(stepName, command string) (*StepResult, error) {
	if !jr.isRunning {
		return nil, fmt.Errorf("container not running")
	}

	fmt.Printf("🔄 Running step: %s\n", stepName)

	result := &StepResult{
		StepName:  stepName,
		Command:   command,
		StartTime: time.Now(),
	}

	// Use docker exec to run command in the existing container
	// -i for interactive (allows input)
	// bash -c to properly handle shell commands with pipes, etc.
	args := []string{
		"exec",
		jr.containerID,
		"bash", "-c", command,
	}

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

	// Stream output as it comes
	go jr.streamOutput(stdout, "STDOUT", &result.Output)
	go jr.streamOutput(stderr, "STDERR", &result.Output)

	// Wait for command to complete
	err = cmd.Wait()
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	if err != nil {
		result.Error = err
		result.Success = false
		fmt.Printf("❌ Step failed: %s (%.2fs)\n", stepName, result.Duration.Seconds())
	} else {
		result.Success = true
		fmt.Printf("✅ Step completed: %s (%.2fs)\n", stepName, result.Duration.Seconds())
	}

	return result, nil
}

// streamOutput reads from pipe and collects output
func (jr *JobRunner) streamOutput(pipe io.ReadCloser, prefix string, output *[]string) {
	defer pipe.Close()

	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		line := scanner.Text()
		*output = append(*output, fmt.Sprintf("[%s] %s", prefix, line))
		// Also print to terminal for real-time feedback
		fmt.Printf("    %s\n", line)
	}
}

// Stop terminates the Docker container
func (jr *JobRunner) Stop() error {
	if !jr.isRunning || jr.containerID == "" {
		return nil
	}

	fmt.Printf("🛑 Stopping container: %s\n", jr.containerID[:12])

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
	Output    []string
	Success   bool
	Error     error
}

// GetOutputLines returns just the actual command output (without prefixes)
func (sr *StepResult) GetOutputLines() []string {
	var lines []string
	for _, line := range sr.Output {
		// Remove the [STDOUT] or [STDERR] prefix
		if strings.HasPrefix(line, "[STDOUT] ") {
			lines = append(lines, line[9:])
		} else if strings.HasPrefix(line, "[STDERR] ") {
			lines = append(lines, line[9:])
		}
	}
	return lines
}

package environment

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Neoxs/gogh/internal/workflow"
)

// EnvironmentManager handles environment variable resolution and context
type EnvironmentManager struct {
	workflowEnv map[string]string
	jobEnv      map[string]string
	githubCtx   GitHubContext
	runnerCtx   RunnerContext
}

// GitHubContext represents GitHub-specific context variables
type GitHubContext struct {
	Repository string
	SHA        string
	Ref        string
	Workspace  string
	EventName  string
	Actor      string
	RunID      string
	RunNumber  string
	Job        string
	Action     string
	ActionPath string
}

// RunnerContext represents runner-specific context variables
type RunnerContext struct {
	OS        string
	Arch      string
	Name      string
	Temp      string
	ToolCache string
}

// NewEnvironmentManager creates a new environment manager
func NewEnvironmentManager(workflowDef *workflow.WorkflowDefinition, projectDir string) *EnvironmentManager {
	githubCtx := createGitHubContext(workflowDef, projectDir)
	runnerCtx := createRunnerContext()

	return &EnvironmentManager{
		workflowEnv: workflowDef.Env,
		githubCtx:   githubCtx,
		runnerCtx:   runnerCtx,
	}
}

// SetJobEnvironment sets job-level environment variables
func (em *EnvironmentManager) SetJobEnvironment(jobEnv map[string]string) {
	em.jobEnv = jobEnv
}

// BuildStepEnvironment builds complete environment for a step with proper precedence
func (em *EnvironmentManager) BuildStepEnvironment(stepEnv map[string]string) map[string]string {
	env := make(map[string]string)

	// 1. Built-in GitHub context (lowest precedence)
	em.addGitHubContextVars(env)
	em.addRunnerContextVars(env)

	// 2. Workflow-level environment variables
	for key, value := range em.workflowEnv {
		env[key] = em.expandVariables(value, env)
	}

	// 3. Job-level environment variables
	for key, value := range em.jobEnv {
		env[key] = em.expandVariables(value, env)
	}

	// 4. Step-level environment variables (highest precedence)
	for key, value := range stepEnv {
		env[key] = em.expandVariables(value, env)
	}

	return env
}

// addGitHubContextVars adds GitHub context variables
func (em *EnvironmentManager) addGitHubContextVars(env map[string]string) {
	env["GITHUB_REPOSITORY"] = em.githubCtx.Repository
	env["GITHUB_SHA"] = em.githubCtx.SHA
	env["GITHUB_REF"] = em.githubCtx.Ref
	env["GITHUB_WORKSPACE"] = em.githubCtx.Workspace
	env["GITHUB_EVENT_NAME"] = em.githubCtx.EventName
	env["GITHUB_ACTOR"] = em.githubCtx.Actor
	env["GITHUB_RUN_ID"] = em.githubCtx.RunID
	env["GITHUB_RUN_NUMBER"] = em.githubCtx.RunNumber
	env["GITHUB_JOB"] = em.githubCtx.Job
	env["GITHUB_ACTION"] = em.githubCtx.Action
	env["GITHUB_ACTION_PATH"] = em.githubCtx.ActionPath

	// Additional convenience variables
	env["CI"] = "true"
	env["GITHUB_ACTIONS"] = "true"
}

// addRunnerContextVars adds runner context variables
func (em *EnvironmentManager) addRunnerContextVars(env map[string]string) {
	env["RUNNER_OS"] = em.runnerCtx.OS
	env["RUNNER_ARCH"] = em.runnerCtx.Arch
	env["RUNNER_NAME"] = em.runnerCtx.Name
	env["RUNNER_TEMP"] = em.runnerCtx.Temp
	env["RUNNER_TOOL_CACHE"] = em.runnerCtx.ToolCache
}

// expandVariables performs basic variable expansion
func (em *EnvironmentManager) expandVariables(value string, currentEnv map[string]string) string {
	result := value

	// Handle ${{ github.* }} context variables
	result = strings.ReplaceAll(result, "${{ github.repository }}", em.githubCtx.Repository)
	result = strings.ReplaceAll(result, "${{ github.sha }}", em.githubCtx.SHA)
	result = strings.ReplaceAll(result, "${{ github.ref }}", em.githubCtx.Ref)
	result = strings.ReplaceAll(result, "${{ github.workspace }}", em.githubCtx.Workspace)
	result = strings.ReplaceAll(result, "${{ github.event_name }}", em.githubCtx.EventName)
	result = strings.ReplaceAll(result, "${{ github.actor }}", em.githubCtx.Actor)
	result = strings.ReplaceAll(result, "${{ github.run_id }}", em.githubCtx.RunID)
	result = strings.ReplaceAll(result, "${{ github.run_number }}", em.githubCtx.RunNumber)

	// Handle ${{ runner.* }} context variables
	result = strings.ReplaceAll(result, "${{ runner.os }}", em.runnerCtx.OS)
	result = strings.ReplaceAll(result, "${{ runner.arch }}", em.runnerCtx.Arch)
	result = strings.ReplaceAll(result, "${{ runner.temp }}", em.runnerCtx.Temp)
	result = strings.ReplaceAll(result, "${{ runner.tool_cache }}", em.runnerCtx.ToolCache)

	// Handle basic $VAR and ${VAR} expansion from current environment
	for envKey, envValue := range currentEnv {
		result = strings.ReplaceAll(result, fmt.Sprintf("$%s", envKey), envValue)
		result = strings.ReplaceAll(result, fmt.Sprintf("${%s}", envKey), envValue)
	}

	return result
}

// GetGitHubContext returns the GitHub context for external use
func (em *EnvironmentManager) GetGitHubContext() GitHubContext {
	return em.githubCtx
}

// Helper functions

func createGitHubContext(workflowDef *workflow.WorkflowDefinition, projectDir string) GitHubContext {
	// Try to extract real git information
	repository := getGitRepository(projectDir)
	sha := getGitSHA(projectDir)
	ref := getGitRef(projectDir)

	return GitHubContext{
		Repository: repository,
		SHA:        sha,
		Ref:        ref,
		Workspace:  "/workspace",
		EventName:  "push", // Default event
		Actor:      getGitActor(projectDir),
		RunID:      fmt.Sprintf("%d", time.Now().Unix()),
		RunNumber:  "1",
		Job:        "", // Will be set per job
		Action:     "", // Will be set per action
		ActionPath: "",
	}
}

func createRunnerContext() RunnerContext {
	return RunnerContext{
		OS:        "Linux",
		Arch:      "X64",
		Name:      "gogh-runner",
		Temp:      "/tmp",
		ToolCache: "/opt/hostedtoolcache",
	}
}

// Git information extraction functions

func getGitRepository(projectDir string) string {
	// Try to get from git remote origin
	cmd := fmt.Sprintf("cd %s && git remote get-url origin 2>/dev/null", projectDir)
	if output := executeCommand(cmd); output != "" {
		// Parse GitHub URL: https://github.com/user/repo.git -> user/repo
		if strings.Contains(output, "github.com") {
			parts := strings.Split(output, "/")
			if len(parts) >= 2 {
				user := parts[len(parts)-2]
				repo := strings.TrimSuffix(parts[len(parts)-1], ".git")
				return fmt.Sprintf("%s/%s", user, repo)
			}
		}
	}

	// Fallback to directory name
	return fmt.Sprintf("local/%s", filepath.Base(projectDir))
}

func getGitSHA(projectDir string) string {
	cmd := fmt.Sprintf("cd %s && git rev-parse HEAD 2>/dev/null", projectDir)
	if output := executeCommand(cmd); output != "" {
		return strings.TrimSpace(output)
	}
	return "0000000000000000000000000000000000000000" // Placeholder SHA
}

func getGitRef(projectDir string) string {
	cmd := fmt.Sprintf("cd %s && git symbolic-ref HEAD 2>/dev/null", projectDir)
	if output := executeCommand(cmd); output != "" {
		return strings.TrimSpace(output)
	}
	return "refs/heads/main" // Default branch
}

func getGitActor(projectDir string) string {
	cmd := fmt.Sprintf("cd %s && git config user.name 2>/dev/null", projectDir)
	if output := executeCommand(cmd); output != "" {
		return strings.TrimSpace(output)
	}
	return "local-user"
}

func executeCommand(command string) string {
	// Execute shell command and return output
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return ""
	}

	// Handle "cd && command" pattern
	if strings.Contains(command, " && ") {
		// For cd && git commands, we need to handle the working directory properly
		parts := strings.Split(command, " && ")
		if len(parts) == 2 && strings.HasPrefix(parts[0], "cd ") {
			workDir := strings.TrimPrefix(parts[0], "cd ")
			gitCommand := parts[1]

			cmd := exec.Command("bash", "-c", gitCommand)
			cmd.Dir = workDir
			if output, err := cmd.Output(); err == nil {
				return string(output)
			}
		}
	}

	return ""
}

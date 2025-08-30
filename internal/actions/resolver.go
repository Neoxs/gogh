package actions

import (
	"fmt"
	"strings"

	"github.com/Neoxs/gogh/internal/logging"
)

// ActionResult represents the output of an action execution
type ActionResult struct {
	Success bool
	Outputs map[string]string
	Error   error
}

// ActionExecutor interface that both built-in and marketplace actions implement
type ActionExecutor interface {
	Execute(ctx *ActionContext, jobLogger *logging.JobLogger) (*ActionResult, error)
	GetName() string
	ValidateInputs(inputs map[string]string) error
}

// ActionContext provides runtime context for action execution
type ActionContext struct {
	// Action configuration
	ActionRef string // e.g., "actions/checkout@v4"
	Inputs    map[string]string

	// Runtime environment
	WorkspaceDir string
	ContainerID  string

	// GitHub context (simulated locally)
	GitHub GitHubContext
	Runner RunnerContext
}

// GitHubContext simulates GitHub's context variables
type GitHubContext struct {
	Repository string // owner/repo
	SHA        string // commit hash
	Ref        string // branch/tag reference
	Workspace  string // workspace path
	EventName  string // push, pull_request, etc.
	// TODO: Include teh rest of github ctx vars
}

// RunnerContext simulates runner environment
type RunnerContext struct {
	OS   string // linux, macos, windows
	Arch string // x64, arm64
	Temp string // temp directory path
	Tool string // tool cache directory
}

// ActionResolver routes action execution to appropriate implementation
type ActionResolver struct {
	builtinActions map[string]ActionExecutor
	cacheDir       string // For future marketplace actions
}

// NewActionResolver creates a new action resolver with built-in actions
func NewActionResolver(projectDir string) *ActionResolver {
	resolver := &ActionResolver{
		builtinActions: make(map[string]ActionExecutor),
		cacheDir:       projectDir + "/.gogh/actions-cache",
	}

	// Register built-in actions
	resolver.registerBuiltinActions()

	return resolver
}

// ResolveAction determines how to execute the given action
func (ar *ActionResolver) ResolveAction(actionRef string, inputs map[string]string, ctx *ActionContext) (ActionExecutor, error) {
	// Check if it's a built-in action first
	if executor, exists := ar.builtinActions[ar.normalizeActionRef(actionRef)]; exists {
		if err := executor.ValidateInputs(inputs); err != nil {
			return nil, fmt.Errorf("invalid inputs for %s: %w", actionRef, err)
		}
		return executor, nil
	}

	// Future: Handle marketplace actions here
	// return ar.resolveMarketplaceAction(actionRef, inputs, ctx)

	return nil, fmt.Errorf("action '%s' not supported (built-in actions available: %s)",
		actionRef, ar.listSupportedActions())
}

// registerBuiltinActions registers all internal action implementations
func (ar *ActionResolver) registerBuiltinActions() {
	// Checkout action
	checkout := &CheckoutAction{}
	ar.builtinActions["actions/checkout"] = checkout

	// Node.js setup action
	setupNode := &SetupNodeAction{}
	ar.builtinActions["actions/setup-node"] = setupNode

	// Add more built-in actions as needed
}

func (ar *ActionResolver) normalizeActionRef(actionRef string) string {
	// Remove version tag for built-in matching: "actions/checkout@v4" -> "actions/checkout"
	parts := strings.Split(actionRef, "@")
	return parts[0]
}

func (ar *ActionResolver) listSupportedActions() string {
	var actions []string
	for actionName := range ar.builtinActions {
		actions = append(actions, actionName)
	}
	return strings.Join(actions, ", ")
}

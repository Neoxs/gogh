package expressions

import (
	"fmt"
	"strings"
)

// EvaluationContext holds all available contexts for expression evaluation
type EvaluationContext struct {
	Github  GitHubContext
	Env     map[string]string
	Job     JobContext
	Runner  RunnerContext
	Secrets map[string]string
	// Add other contexts as needed (steps, matrix, etc.)
}

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

type JobContext struct {
	Status string
	// ... other job context fields
}

type RunnerContext struct {
	OS        string
	Arch      string
	Name      string
	Temp      string
	ToolCache string
}

// ExpressionEvaluator handles GitHub Actions expression evaluation
type ExpressionEvaluator struct {
	context *EvaluationContext
}

// NewExpressionEvaluator creates a new expression evaluator
func NewExpressionEvaluator(ctx *EvaluationContext) *ExpressionEvaluator {
	return &ExpressionEvaluator{
		context: ctx,
	}
}

// Evaluate processes a GitHub Actions expression
func (ee *ExpressionEvaluator) Evaluate(expression string) (string, error) {
	// For now, implement basic ${{ ... }} handling
	// Later, you can integrate a proper expression parser like actionlint

	if !strings.HasPrefix(expression, "${{") || !strings.HasSuffix(expression, "}}") {
		return expression, nil // Not an expression
	}

	// Extract the inner expression
	inner := strings.TrimSpace(expression[3 : len(expression)-2])

	return ee.evaluateExpression(inner)
}

// evaluateExpression handles the core expression evaluation
func (ee *ExpressionEvaluator) evaluateExpression(expr string) (string, error) {
	// Handle simple property access for now
	// This is where you'd integrate a proper parser later

	parts := strings.Split(expr, ".")
	if len(parts) != 2 {
		return expr, fmt.Errorf("unsupported expression format: %s", expr)
	}

	contextName := strings.ToLower(parts[0])
	property := parts[1]

	switch contextName {
	case "github":
		return ee.getGitHubProperty(property)
	case "env":
		if value, exists := ee.context.Env[property]; exists {
			return value, nil
		}
		return "", fmt.Errorf("environment variable %s not found", property)
	case "runner":
		return ee.getRunnerProperty(property)
	default:
		return "", fmt.Errorf("unknown context: %s", contextName)
	}
}

func (ee *ExpressionEvaluator) getGitHubProperty(property string) (string, error) {
	switch property {
	case "repository":
		return ee.context.Github.Repository, nil
	case "sha":
		return ee.context.Github.SHA, nil
	case "ref":
		return ee.context.Github.Ref, nil
	case "event_name":
		return ee.context.Github.EventName, nil
	case "actor":
		return ee.context.Github.Actor, nil
	default:
		return "", fmt.Errorf("unknown github property: %s", property)
	}
}

func (ee *ExpressionEvaluator) getRunnerProperty(property string) (string, error) {
	switch property {
	case "os":
		return ee.context.Runner.OS, nil
	case "arch":
		return ee.context.Runner.Arch, nil
	default:
		return "", fmt.Errorf("unknown runner property: %s", property)
	}
}

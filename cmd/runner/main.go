package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Neoxs/gogh/internal/executor"
	"github.com/Neoxs/gogh/internal/workflow"
	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "runner",
		Short: "Run GitHub Actions workflows locally",
		Long:  "A tool to execute GitHub Actions workflows locally with Docker support",
	}

	var runCmd = &cobra.Command{
		Use:   "run [workflow-file]",
		Short: "Run a workflow file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workflowFile := args[0]
			return runWorkflow(workflowFile)
		},
	}

	rootCmd.AddCommand(runCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runWorkflow(workflowFile string) error {
	// Parse the workflow
	parser := workflow.NewParser()
	workflowDef, err := parser.ParseFile(workflowFile)
	if err != nil {
		return fmt.Errorf("failed to parse workflow: %w", err)
	}

	// Derive project directory from workflow file path
	projectDir, err := getProjectDirectory(workflowFile)
	if err != nil {
		return fmt.Errorf("failed to determine project directory: %w", err)
	}

	// DEBUG: Print what directory we detected
	fmt.Printf("🔍 Detected project directory: %s\n", projectDir)
	fmt.Printf("🔍 Workflow file: %s\n", workflowFile)

	// Create executor with logging and display (now returns error)
	executor, err := executor.NewWorkflowExecutor(workflowDef, projectDir)
	if err != nil {
		return fmt.Errorf("failed to create workflow executor: %w", err)
	}

	// Execute workflow
	return executor.Execute()
}

// getProjectDirectory determines the project root directory from the workflow file path
func getProjectDirectory(workflowFile string) (string, error) {
	// Get absolute path of workflow file
	absWorkflowFile, err := filepath.Abs(workflowFile)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path of workflow file: %w", err)
	}

	// Check if file exists
	if _, err := os.Stat(absWorkflowFile); os.IsNotExist(err) {
		return "", fmt.Errorf("workflow file does not exist: %s", absWorkflowFile)
	}

	workflowDir := filepath.Dir(absWorkflowFile)

	// Look for .github/workflows pattern and find project root
	// Expected pattern: /path/to/project/.github/workflows/workflow.yml
	if strings.HasSuffix(workflowDir, "/.github/workflows") || strings.HasSuffix(workflowDir, "\\.github\\workflows") {
		// Extract project root (remove /.github/workflows)
		projectRoot := filepath.Dir(filepath.Dir(workflowDir))
		return projectRoot, nil
	}

	// If not in standard .github/workflows location, use the workflow file's directory
	// This allows for workflows stored elsewhere
	return workflowDir, nil
}

package main

import (
	"fmt"
	"os"

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
	fmt.Println(`
    __/\\\\\\\\\\\\_______/\\\\\__________/\\\\\\\\\\\\__/\\\________/\\\_        
 ___/\\\//////////______/\\\///\\\______/\\\//////////__\/\\\_______\/\\\_       
  __/\\\_______________/\\\/__\///\\\___/\\\_____________\/\\\_______\/\\\_      
   _\/\\\____/\\\\\\\__/\\\______\//\\\_\/\\\____/\\\\\\\_\/\\\\\\\\\\\\\\\_     
    _\/\\\___\/////\\\_\/\\\_______\/\\\_\/\\\___\/////\\\_\/\\\/////////\\\_    
     _\/\\\_______\/\\\_\//\\\______/\\\__\/\\\_______\/\\\_\/\\\_______\/\\\_   
      _\/\\\_______\/\\\__\///\\\__/\\\____\/\\\_______\/\\\_\/\\\_______\/\\\_  
       _\//\\\\\\\\\\\\/_____\///\\\\\/_____\//\\\\\\\\\\\\/__\/\\\_______\/\\\_ 
        __\////////////_________\/////________\////////////____\///________\///__
                                                                                  `)
	fmt.Printf("🚀 Starting workflow: %s\n", workflowFile)

	// Parse the workflow
	parser := workflow.NewParser()
	workflowDef, err := parser.ParseFile(workflowFile)
	if err != nil {
		return fmt.Errorf("failed to parse workflow: %w", err)
	}

	// Get current directory as project root
	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Create and execute workflow
	executor := executor.NewWorkflowExecutor(workflowDef, projectDir)
	return executor.Execute()
}

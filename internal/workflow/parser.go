package workflow

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Parser handles workflow YAML parsing
// Parser handles workflow YAML parsing
type Parser struct{}

// NewParser creates a new workflow parser
func NewParser() *Parser {
	return &Parser{}
}

// ParseFile reads and parses a workflow YAML file
func (p *Parser) ParseFile(filename string) (*WorkflowDefinition, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow file %s: %w", filename, err)
	}

	return p.Parse(data)
}

// Parse parses workflow YAML data
func (p *Parser) Parse(data []byte) (*WorkflowDefinition, error) {
	var workflow WorkflowDefinition

	if err := yaml.Unmarshal(data, &workflow); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Basic validation
	if workflow.Name == "" {
		return nil, fmt.Errorf("workflow name is required")
	}

	if len(workflow.Jobs) == 0 {
		return nil, fmt.Errorf("workflow must contain at least one job")
	}

	return &workflow, nil
}

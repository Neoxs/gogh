package workflow

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// JobNeeds is a custom type that can handle both single strings and arrays
type JobNeeds []string

// UnmarshalYAML implements custom YAML unmarshaling for needs field
func (jn *JobNeeds) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		// Handle single string: needs: job1
		var singleNeed string
		if err := value.Decode(&singleNeed); err != nil {
			return err
		}
		*jn = JobNeeds{singleNeed}
		return nil

	case yaml.SequenceNode:
		// Handle array: needs: [job1, job2]
		var multipleNeeds []string
		if err := value.Decode(&multipleNeeds); err != nil {
			return err
		}
		*jn = JobNeeds(multipleNeeds)
		return nil

	default:
		return fmt.Errorf("needs must be a string or array of strings")
	}
}

// ToSlice returns the needs as a regular string slice
func (jn JobNeeds) ToSlice() []string {
	return []string(jn)
}

// WorkflowDefinition represents the parsed workflow YAML
type WorkflowDefinition struct {
	Name string                   `yaml:"name"`
	On   map[string]interface{}   `yaml:"on"`
	Env  map[string]string        `yaml:"env,omitempty"`
	Jobs map[string]JobDefinition `yaml:"jobs"`
}

// JobDefinition represents a single job in the workflow
type JobDefinition struct {
	RunsOn string                 `yaml:"runs-on"`
	Needs  JobNeeds               `yaml:"needs"`
	With   map[string]interface{} `yaml:"with,omitempty"` // Action inputs
	Env    map[string]string      `yaml:"env,omitempty"`
	Steps  []StepDefinition       `yaml:"steps"`
}

// StepDefinition represents a single step in a job
type StepDefinition struct {
	Name string                 `yaml:"name"`
	Run  string                 `yaml:"run,omitempty"`
	Uses string                 `yaml:"uses,omitempty"`
	With map[string]interface{} `yaml:"with,omitempty"` // Action inputs
	Env  map[string]string      `yaml:"env,omitempty"`  // Environment variables
}

// BuildExecutionPlan resolves job dependencies and returns execution order
func (w *WorkflowDefinition) BuildExecutionPlan() ([]string, error) {
	if len(w.Jobs) == 0 {
		return nil, fmt.Errorf("no jobs found in workflow")
	}

	graph := make(map[string][]string)
	inDegree := make(map[string]int)

	// Build dependency graph
	for jobID, job := range w.Jobs {
		jobNeeds := job.Needs.ToSlice()
		inDegree[jobID] = len(jobNeeds)
		for _, dependency := range jobNeeds {
			// Validate dependency exists
			if _, exists := w.Jobs[dependency]; !exists {
				return nil, fmt.Errorf("job %s depends on non-existent job %s", jobID, dependency)
			}
			graph[dependency] = append(graph[dependency], jobID)
		}
	}

	// Topological sort using Kahn's algorithm
	var queue []string
	var result []string

	// Find jobs with no dependencies
	for jobID, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, jobID)
		}
	}

	// Process jobs in dependency order
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// Update dependencies
		for _, dependent := range graph[current] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	// Detect circular dependencies
	if len(result) != len(w.Jobs) {
		return nil, fmt.Errorf("circular dependency detected in workflow jobs")
	}

	return result, nil
}

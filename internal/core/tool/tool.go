package tool

import "context"

// Tool is a single executable processing unit in a static pipeline.
type Tool interface {
	// Name returns a unique identifier used by the pipeline registry.
	Name() string
	// Description returns a human-readable description (useful for LLM tool calling).
	Description() string
	// Schema returns the input JSON schema for this tool.
	Schema() map[string]interface{}
	// OutputSchema returns the output JSON schema for this tool (used as function parameters schema).
	// For non-LLM tools this can return nil.
	OutputSchema() map[string]interface{}
	// Validate validates the input payload.
	Validate(input Input) error
	// Execute runs the tool and returns a structured output.
	Execute(ctx context.Context, input Input) (Output, error)
}

package config

// PipelineConfig controls pipeline execution behavior.
type PipelineConfig struct {
	ToolCalling ToolCallingConfig `mapstructure:"tool_calling"`
}

// ToolCallingConfig enables OpenAI-compatible tool calling for structured outputs.
type ToolCallingConfig struct {
	Enabled       bool `mapstructure:"enabled"`
	AllowThinking bool `mapstructure:"allow_thinking"`
}

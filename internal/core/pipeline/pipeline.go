package pipeline

// Step is one tool execution in a pipeline.
// InputMapping maps tool input field -> value expression (e.g. "request.audio", "asr_result.text").
type Step struct {
	ToolName     string
	InputMapping map[string]string
	OutputKey    string
}

// Pipeline is a static, predefined sequence of tools.
type Pipeline struct {
	Name  string
	Steps []Step
}

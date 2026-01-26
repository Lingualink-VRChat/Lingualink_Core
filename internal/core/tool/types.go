package tool

import (
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
)

// Input is the payload passed to a Tool.
type Input struct {
	Data    map[string]interface{}
	Context *PipelineContext
}

// Output is the structured result produced by a Tool.
type Output struct {
	Data     map[string]interface{}
	Metadata map[string]interface{}
}

// PipelineContext carries cross-tool metadata and step outputs.
type PipelineContext struct {
	RequestID       string
	OriginalRequest map[string]interface{}
	StepOutputs     map[string]Output
	Dictionary      []config.DictionaryTerm
	Metrics         map[string]time.Duration
}

// pipeline_select.go contains pipeline tool registry initialization and selection logic.
package audio

import (
	"fmt"

	coreerrors "github.com/Lingualink-VRChat/Lingualink_Core/internal/core/errors"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/pipeline"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/prompt"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/tool"
)

func (p *Processor) ensurePipelineInitialized() error {
	if p.pipelineExec != nil && p.toolRegistry != nil {
		return nil
	}

	reg := tool.NewRegistry()

	if err := reg.Register(tool.NewASRTool(p.asrManager)); err != nil {
		return err
	}

	toolCallingEnabled := p.pipelineConfig.ToolCalling.Enabled
	allowThinking := p.pipelineConfig.ToolCalling.AllowThinking

	if err := reg.Register(tool.NewCorrectTool(p.llmManager, p.promptEngine, toolCallingEnabled, allowThinking)); err != nil {
		return err
	}
	if err := reg.Register(tool.NewTranslateTool(p.llmManager, p.promptEngine, toolCallingEnabled, allowThinking)); err != nil {
		return err
	}
	if err := reg.Register(tool.NewCorrectTranslateTool(p.llmManager, p.promptEngine, toolCallingEnabled, allowThinking)); err != nil {
		return err
	}

	p.toolRegistry = reg
	p.pipelineExec = pipeline.NewExecutor(reg)
	return nil
}

func (p *Processor) selectPipeline(task prompt.TaskType) (pipeline.Pipeline, error) {
	switch task {
	case prompt.TaskTranscribe:
		if p.correction.Enabled {
			return pipeline.TranscribeCorrect(), nil
		}
		return pipeline.Transcribe(), nil
	case prompt.TaskTranslate:
		if p.correction.Enabled {
			if p.correction.MergeWithTranslation {
				return pipeline.TranslateMerged(), nil
			}
			return pipeline.TranslateSplit(), nil
		}
		return pipeline.Translate(), nil
	default:
		return pipeline.Pipeline{}, coreerrors.NewValidationError(fmt.Sprintf("unsupported task type: %s", task), nil)
	}
}

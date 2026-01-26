// processor.go defines the audio Processor and its wiring.
package audio

import (
	"context"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/asr"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/llm"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/pipeline"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/prompt"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/tool"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/metrics"
	"github.com/sirupsen/logrus"
)

// Processor 音频处理器
type Processor struct {
	asrManager     *asr.Manager
	llmManager     *llm.Manager
	correction     config.CorrectionConfig
	promptEngine   *prompt.Engine
	audioConverter *AudioConverter
	metrics        metrics.MetricsCollector
	config         config.PromptConfig
	pipelineConfig config.PipelineConfig
	toolRegistry   *tool.Registry
	pipelineExec   *pipeline.Executor
	logger         *logrus.Logger
}

// NewProcessor 创建音频处理器
func NewProcessor(
	asrManager *asr.Manager,
	llmManager *llm.Manager,
	promptEngine *prompt.Engine,
	promptCfg config.PromptConfig,
	correctionCfg config.CorrectionConfig,
	logger *logrus.Logger,
	metricsCollector metrics.MetricsCollector,
) *Processor {
	return &Processor{
		asrManager:     asrManager,
		llmManager:     llmManager,
		promptEngine:   promptEngine,
		audioConverter: NewAudioConverter(logger),
		metrics:        metricsCollector,
		config:         promptCfg,
		correction:     correctionCfg,
		pipelineConfig: config.PipelineConfig{
			ToolCalling: config.ToolCallingConfig{
				Enabled:       true,
				AllowThinking: false,
			},
		},
		logger: logger,
	}
}

// WithPipelineConfig sets pipeline configuration.
func (p *Processor) WithPipelineConfig(cfg config.PipelineConfig) *Processor {
	p.pipelineConfig = cfg
	// Lazily rebuilt on first use to honor the latest tool_calling flags.
	p.toolRegistry = nil
	p.pipelineExec = nil
	return p
}

// ProcessDirect optionally handles requests without going through ProcessingService's single-LLM-call flow.
func (p *Processor) ProcessDirect(ctx context.Context, req ProcessRequest) (*ProcessResponse, bool, error) {
	resp, err := p.processWithPipeline(ctx, req)
	if err != nil {
		return nil, true, err
	}
	return resp, true, nil
}

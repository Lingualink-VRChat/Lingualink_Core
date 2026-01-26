package text

import (
	"context"
	"fmt"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/cache"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/correction"
	coreerrors "github.com/Lingualink-VRChat/Lingualink_Core/internal/core/errors"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/llm"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/pipeline"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/prompt"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/tool"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/logging"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/metrics"
	"github.com/sirupsen/logrus"
)

// ProcessRequest 文本处理请求
type ProcessRequest struct {
	Text            string                  `json:"text"`
	Task            prompt.TaskType         `json:"task,omitempty"`
	SourceLanguage  string                  `json:"source_language,omitempty"`
	TargetLanguages []string                `json:"target_languages"`
	UserDictionary  []config.DictionaryTerm `json:"user_dictionary,omitempty"`
	Options         map[string]interface{}  `json:"options,omitempty"`
}

// BatchProcessRequest is the request payload for batch text translation.
type BatchProcessRequest struct {
	Texts           []string               `json:"texts"`
	SourceLanguage  string                 `json:"source_language,omitempty"`
	TargetLanguages []string               `json:"target_languages"`
	Options         map[string]interface{} `json:"options,omitempty"`
}

// GetTargetLanguages 实现 ProcessableRequest 接口
func (req ProcessRequest) GetTargetLanguages() []string {
	return req.TargetLanguages
}

// ProcessResponse 文本处理响应
type ProcessResponse struct {
	RequestID      string                 `json:"request_id"`
	Status         string                 `json:"status"`
	SourceText     string                 `json:"source_text"`
	CorrectedText  string                 `json:"corrected_text,omitempty"`
	Translations   map[string]string      `json:"translations"`
	RawResponse    string                 `json:"raw_response"`
	ProcessingTime float64                `json:"processing_time"`
	Metadata       map[string]interface{} `json:"metadata"`
}

func (r *ProcessResponse) SetProcessingTime(seconds float64) {
	r.ProcessingTime = seconds
}

func (r *ProcessResponse) SetRequestID(requestID string) {
	r.RequestID = requestID
}

// Processor 文本处理器
type Processor struct {
	llmManager   *llm.Manager
	promptEngine *prompt.Engine
	metrics      metrics.MetricsCollector
	config       config.PromptConfig
	correction   config.CorrectionConfig
	pipelineCfg  config.PipelineConfig
	toolRegistry *tool.Registry
	pipelineExec *pipeline.Executor
	logger       *logrus.Logger

	translationCache cache.TranslationCache
	cacheTTL         time.Duration
}

// NewProcessor 创建文本处理器
func NewProcessor(
	llmManager *llm.Manager,
	promptEngine *prompt.Engine,
	metrics metrics.MetricsCollector,
	promptCfg config.PromptConfig,
	logger *logrus.Logger,
) *Processor {
	return NewProcessorWithCache(llmManager, promptEngine, metrics, promptCfg, logger, nil, 0)
}

// NewProcessorWithCache creates a text Processor with an optional translation cache.
func NewProcessorWithCache(
	llmManager *llm.Manager,
	promptEngine *prompt.Engine,
	metrics metrics.MetricsCollector,
	promptCfg config.PromptConfig,
	logger *logrus.Logger,
	translationCache cache.TranslationCache,
	cacheTTL time.Duration,
) *Processor {
	return &Processor{
		llmManager:   llmManager,
		promptEngine: promptEngine,
		metrics:      metrics,
		config:       promptCfg,
		correction:   config.CorrectionConfig{},
		pipelineCfg: config.PipelineConfig{
			ToolCalling: config.ToolCallingConfig{
				Enabled:       true,
				AllowThinking: false,
			},
		},
		logger:           logger,
		translationCache: translationCache,
		cacheTTL:         cacheTTL,
	}
}

// WithCorrectionConfig sets correction configuration.
func (p *Processor) WithCorrectionConfig(cfg config.CorrectionConfig) *Processor {
	p.correction = cfg
	return p
}

// WithPipelineConfig configures pipeline execution behavior.
func (p *Processor) WithPipelineConfig(cfg config.PipelineConfig) *Processor {
	p.pipelineCfg = cfg
	// Lazily rebuilt on first use to honor the latest tool_calling flags.
	p.toolRegistry = nil
	p.pipelineExec = nil
	return p
}

// Process 方法已移除 - 现在使用 ProcessingService 统一处理流程

func (p *Processor) ensurePipelineInitialized() error {
	if p.pipelineExec != nil && p.toolRegistry != nil {
		return nil
	}

	reg := tool.NewRegistry()

	toolCallingEnabled := p.pipelineCfg.ToolCalling.Enabled
	allowThinking := p.pipelineCfg.ToolCalling.AllowThinking

	if err := reg.Register(tool.NewTextCorrectTool(p.llmManager, p.promptEngine, toolCallingEnabled, allowThinking)); err != nil {
		return err
	}
	if err := reg.Register(tool.NewTextTranslateTool(p.llmManager, p.promptEngine, toolCallingEnabled, allowThinking)); err != nil {
		return err
	}
	if err := reg.Register(tool.NewTextCorrectTranslateTool(p.llmManager, p.promptEngine, toolCallingEnabled, allowThinking)); err != nil {
		return err
	}

	p.toolRegistry = reg
	p.pipelineExec = pipeline.NewExecutor(reg)
	return nil
}

// Validate 验证请求 - 实现 LogicHandler 接口
func (p *Processor) Validate(req ProcessRequest) error {
	if req.Text == "" {
		return coreerrors.NewValidationError("text is required", nil)
	}

	// 验证文本长度限制（3000字符）
	maxLength := 3000
	if len(req.Text) > maxLength {
		return coreerrors.NewValidationError(
			fmt.Sprintf("text length (%d characters) exceeds maximum allowed length (%d characters)", len(req.Text), maxLength),
			nil,
		)
	}

	if len(req.TargetLanguages) == 0 {
		// translate tasks require target languages; transcribe/correct does not.
		task := req.Task
		if task == "" {
			task = prompt.TaskTranslate
		}
		if task == prompt.TaskTranslate {
			return coreerrors.NewValidationError("target languages are required", nil)
		}
	}

	return nil
}

func (p *Processor) TryGetCachedResponse(ctx context.Context, req ProcessRequest) (*ProcessResponse, bool, error) {
	if p.translationCache == nil || p.cacheTTL <= 0 {
		return nil, false, nil
	}
	task := req.Task
	if task == "" {
		task = prompt.TaskTranslate
	}
	if task != prompt.TaskTranslate || p.correction.Enabled {
		return nil, false, nil
	}

	targetLangCodes := req.TargetLanguages
	if len(targetLangCodes) == 0 {
		targetLangCodes = p.config.Defaults.TargetLanguages
	}

	key := cache.GenerateCacheKey(req.Text, req.SourceLanguage, targetLangCodes)
	cached, ok := p.translationCache.Get(key)
	if !ok || cached == nil || len(cached.Translations) == 0 {
		return nil, false, nil
	}

	resp := acquireProcessResponse()
	resp.RequestID = generateRequestID()
	resp.Status = "success"
	resp.SourceText = req.Text
	resp.RawResponse = ""
	resp.ProcessingTime = 0
	resp.Metadata["cache_hit"] = true
	resp.Metadata["cached_at"] = cached.CachedAt.Unix()
	resp.Metadata["pipeline"] = pipeline.PipelineTextTranslate
	resp.Metadata["step_durations_ms"] = map[string]int64{"cache": 0}

	for k, v := range cached.Translations {
		resp.Translations[k] = v
		metrics.IncTranslation(req.SourceLanguage, k)
	}

	return resp, true, nil
}

func (p *Processor) StoreCachedResponse(ctx context.Context, req ProcessRequest, resp *ProcessResponse) error {
	if p.translationCache == nil || p.cacheTTL <= 0 {
		return nil
	}
	task := req.Task
	if task == "" {
		task = prompt.TaskTranslate
	}
	if task != prompt.TaskTranslate || p.correction.Enabled {
		return nil
	}
	if resp == nil || resp.Status != "success" || len(resp.Translations) == 0 {
		return nil
	}

	targetLangCodes := req.TargetLanguages
	if len(targetLangCodes) == 0 {
		targetLangCodes = p.config.Defaults.TargetLanguages
	}

	key := cache.GenerateCacheKey(req.Text, req.SourceLanguage, targetLangCodes)
	p.translationCache.Set(key, &cache.CachedTranslation{
		Translations: resp.Translations,
		CachedAt:     time.Now(),
	}, p.cacheTTL)

	return nil
}

// ProcessDirect optionally handles requests that don't fit the single-call ProcessingService flow.
func (p *Processor) ProcessDirect(ctx context.Context, req ProcessRequest) (*ProcessResponse, bool, error) {
	resp, err := p.processWithPipeline(ctx, req)
	if err != nil {
		return nil, true, err
	}
	return resp, true, nil
}

// generateRequestID 生成请求ID
func generateRequestID() string {
	return fmt.Sprintf("txt_%d", time.Now().UnixNano())
}

func (p *Processor) processWithPipeline(ctx context.Context, req ProcessRequest) (*ProcessResponse, error) {
	requestID, _ := logging.RequestIDFromContext(ctx)

	if err := p.ensurePipelineInitialized(); err != nil {
		return nil, err
	}

	task := req.Task
	if task == "" {
		task = prompt.TaskTranslate
	}

	targetLangCodes := req.TargetLanguages
	if task == prompt.TaskTranslate && len(targetLangCodes) == 0 {
		targetLangCodes = p.config.Defaults.TargetLanguages
	}

	dictionary := correction.MergeDictionaries(p.correction.GlobalDictionary, req.UserDictionary)

	var selected pipeline.Pipeline
	switch task {
	case prompt.TaskTranslate:
		if p.correction.Enabled {
			if p.correction.MergeWithTranslation {
				selected = pipeline.TextCorrectTranslate()
			} else {
				selected = pipeline.TextCorrectThenTranslate()
			}
		} else {
			selected = pipeline.TextTranslate()
		}
	case prompt.TaskTranscribe:
		if p.correction.Enabled {
			selected = pipeline.TextCorrect()
		} else {
			selected = pipeline.TextPassthrough()
		}
	default:
		return nil, coreerrors.NewValidationError(fmt.Sprintf("unsupported task type: %s", task), nil)
	}

	pctx := &tool.PipelineContext{
		RequestID: requestID,
		OriginalRequest: map[string]interface{}{
			"text":             req.Text,
			"source_language":  req.SourceLanguage,
			"target_languages": targetLangCodes,
			"task":             string(task),
			"options":          req.Options,
		},
		Dictionary: dictionary,
	}

	outCtx, err := p.pipelineExec.Execute(ctx, selected, pctx)
	if err != nil {
		return nil, err
	}

	resp := acquireProcessResponse()
	resp.RequestID = generateRequestID()
	resp.Status = "success"
	resp.SourceText = req.Text
	resp.Metadata["pipeline"] = selected.Name

	stepDurations := make(map[string]int64)
	for k, d := range outCtx.Metrics {
		stepDurations[k] = d.Milliseconds()
	}
	resp.Metadata["step_durations_ms"] = stepDurations

	switch selected.Name {
	case pipeline.PipelineTextPassthrough:
		resp.CorrectedText = req.Text
	case pipeline.PipelineTextCorrect:
		correctOut := outCtx.StepOutputs["correct_result"]
		if v, ok := correctOut.Data["corrected_text"].(string); ok {
			resp.CorrectedText = v
		}
		if v, ok := correctOut.Data["raw_response"].(string); ok {
			resp.RawResponse = v
		}
		for k, v := range correctOut.Metadata {
			resp.Metadata[k] = v
		}
	case pipeline.PipelineTextCorrectTranslate:
		ctOut := outCtx.StepOutputs["correct_translate_result"]
		if v, ok := ctOut.Data["corrected_text"].(string); ok {
			resp.CorrectedText = v
		}
		if v, ok := ctOut.Data["raw_response"].(string); ok {
			resp.RawResponse = v
		}
		if translations, ok := ctOut.Data["translations"].(map[string]string); ok {
			for k, v := range translations {
				resp.Translations[k] = v
			}
		}
		for k, v := range ctOut.Metadata {
			resp.Metadata[k] = v
		}
	case pipeline.PipelineTextTranslate:
		trOut := outCtx.StepOutputs["translate_result"]
		if v, ok := trOut.Data["raw_response"].(string); ok {
			resp.RawResponse = v
		}
		if translations, ok := trOut.Data["translations"].(map[string]string); ok {
			for k, v := range translations {
				resp.Translations[k] = v
			}
		}
		for k, v := range trOut.Metadata {
			resp.Metadata[k] = v
		}
	case pipeline.PipelineTextCorrectThenTranslate:
		correctOut := outCtx.StepOutputs["correct_result"]
		translateOut := outCtx.StepOutputs["translate_result"]

		if v, ok := correctOut.Data["corrected_text"].(string); ok {
			resp.CorrectedText = v
		}
		if v, ok := translateOut.Data["raw_response"].(string); ok {
			resp.RawResponse = v
		}
		if translations, ok := translateOut.Data["translations"].(map[string]string); ok {
			for k, v := range translations {
				resp.Translations[k] = v
			}
		}

		resp.Metadata["correction_backend"] = correctOut.Metadata["backend"]
		resp.Metadata["translation_backend"] = translateOut.Metadata["backend"]
		resp.Metadata["raw_correction_response"] = correctOut.Data["raw_response"]

		for k, v := range translateOut.Metadata {
			resp.Metadata[k] = v
		}
	default:
		return nil, coreerrors.NewInternalError(fmt.Sprintf("unknown pipeline: %s", selected.Name), nil)
	}

	if task == prompt.TaskTranslate {
		if len(resp.Translations) == 0 {
			resp.Status = "partial_success"
		}
		for code := range resp.Translations {
			metrics.IncTranslation(req.SourceLanguage, code)
		}
	}

	return resp, nil
}

// GetCapabilities 获取文本处理能力
func (p *Processor) GetCapabilities() map[string]interface{} {
	return map[string]interface{}{
		"max_text_length":     3000,
		"supported_languages": p.promptEngine.GetLanguages(),
		"features": []string{
			"text_translation",
			"multi_target_languages",
			"language_detection",
		},
	}
}

// GetSupportedLanguages 获取支持的语言列表
func (p *Processor) GetSupportedLanguages() []map[string]interface{} {
	languages := p.promptEngine.GetLanguages()
	result := make([]map[string]interface{}, 0, len(languages))

	for code, lang := range languages {
		langInfo := map[string]interface{}{
			"code":    code,
			"display": lang.Names["display"],
			"type":    lang.Type,
			"aliases": lang.Aliases,
		}
		if lang.StyleNote != "" {
			langInfo["style_note"] = lang.StyleNote
		}
		result = append(result, langInfo)
	}

	return result
}

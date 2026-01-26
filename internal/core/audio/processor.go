package audio

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/asr"
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

// ProcessRequest 音频处理请求
type ProcessRequest struct {
	Audio           []byte                  `json:"audio"`
	AudioFormat     string                  `json:"audio_format"`
	Task            prompt.TaskType         `json:"task"`
	SourceLanguage  string                  `json:"source_language,omitempty"`
	TargetLanguages []string                `json:"target_languages"` // 接收短代码
	UserDictionary  []config.DictionaryTerm `json:"user_dictionary,omitempty"`
	// 移除Template字段，使用硬编码的默认模板
	// 移除 UserPrompt，改为服务端控制
	Options map[string]interface{} `json:"options,omitempty"`

	cleanup     func()
	cleanupOnce *sync.Once
}

// GetTargetLanguages 实现 ProcessableRequest 接口
func (req ProcessRequest) GetTargetLanguages() []string {
	return req.TargetLanguages
}

// SetCleanup registers a callback that will be executed after the LLM request is finished.
// It can be used to release large temporary buffers.
func (req *ProcessRequest) SetCleanup(fn func()) {
	req.cleanup = fn
	if fn != nil {
		req.cleanupOnce = new(sync.Once)
	} else {
		req.cleanupOnce = nil
	}
}

// Cleanup executes the registered cleanup callback at most once.
// It is safe to call multiple times across copies of ProcessRequest.
func (req ProcessRequest) Cleanup() {
	if req.cleanupOnce == nil || req.cleanup == nil {
		return
	}
	req.cleanupOnce.Do(req.cleanup)
}

// ProcessResponse 音频处理响应
type ProcessResponse struct {
	RequestID      string                 `json:"request_id"`
	Status         string                 `json:"status"`
	Transcription  string                 `json:"transcription,omitempty"`
	CorrectedText  string                 `json:"corrected_text,omitempty"`
	Translations   map[string]string      `json:"translations,omitempty"` // 键为短代码
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

// Process 方法已移除 - 现在使用 ProcessingService 统一处理流程

// Validate 验证请求 - 实现 LogicHandler 接口
func (p *Processor) Validate(req ProcessRequest) error {
	if len(req.Audio) == 0 {
		return coreerrors.NewValidationError("audio data is required", nil)
	}

	if req.AudioFormat == "" {
		return coreerrors.NewValidationError("audio format is required", nil)
	}

	// 验证音频大小限制（32MB）
	maxSize := 32 * 1024 * 1024
	if len(req.Audio) > maxSize {
		return coreerrors.NewValidationError(
			fmt.Sprintf("audio size (%d bytes) exceeds maximum allowed size (%d bytes)", len(req.Audio), maxSize),
			nil,
		)
	}

	// 验证支持的格式
	supportedFormats := map[string]bool{
		"wav":  true,
		"mp3":  true,
		"m4a":  true,
		"opus": true,
		"flac": true,
	}

	if !supportedFormats[req.AudioFormat] {
		return coreerrors.NewValidationError(fmt.Sprintf("unsupported audio format: %s", req.AudioFormat), nil)
	}

	// 验证任务类型
	validTasks := map[prompt.TaskType]bool{
		prompt.TaskTranslate:  true,
		prompt.TaskTranscribe: true,
		// 保留用于后续扩展
		// prompt.TaskTranscribeAndTranslate: true,
	}

	if !validTasks[req.Task] {
		return coreerrors.NewValidationError(fmt.Sprintf("invalid task type: %s", req.Task), nil)
	}

	return nil
}

// BuildLLMRequest 构建LLM请求 - 实现 LogicHandler 接口
func (p *Processor) BuildLLMRequest(ctx context.Context, req ProcessRequest) (*llm.LLMRequest, error) {
	requestID, _ := logging.RequestIDFromContext(ctx)

	// 1. 验证音频数据
	if err := p.audioConverter.ValidateAudioData(req.Audio, req.AudioFormat); err != nil {
		entry := p.logger.WithError(err)
		if requestID != "" {
			entry = entry.WithField(logging.FieldRequestID, requestID)
		}
		entry.Warn("Audio validation failed, proceeding anyway")
	}

	// 2. 转换音频格式（如果需要）
	audioData := req.Audio
	audioFormat := req.AudioFormat

	if p.audioConverter.IsConversionNeeded(req.AudioFormat) {
		convertedData, err := p.audioConverter.ConvertToWAV(req.Audio, req.AudioFormat)
		if err != nil {
			p.logger.WithError(err).Warn("Audio conversion failed, using original format")
		} else {
			req.Cleanup()
			audioData = convertedData
			audioFormat = "wav"
			fields := logrus.Fields{
				"original_format":  req.AudioFormat,
				"converted_format": "wav",
			}
			if requestID != "" {
				fields[logging.FieldRequestID] = requestID
			}
			p.logger.WithFields(fields).Info("Audio converted successfully")
		}
	}

	// 3. 处理目标语言（使用短代码）
	targetLangCodes := req.TargetLanguages
	// 只有在translate任务且没有指定目标语言时，才使用默认目标语言
	if req.Task == prompt.TaskTranslate && len(targetLangCodes) == 0 {
		targetLangCodes = p.config.Defaults.TargetLanguages // 从配置获取默认目标语言
	}

	// 4. Stage 1: ASR 转录
	if p.asrManager == nil {
		return nil, coreerrors.NewInternalError("asr manager not configured", nil)
	}

	asrStart := time.Now()
	asrResp, err := p.asrManager.Transcribe(ctx, &asr.ASRRequest{
		Audio:       audioData,
		AudioFormat: audioFormat,
		Language:    req.SourceLanguage,
	})
	if err != nil {
		return nil, err
	}

	if req.Task == prompt.TaskTranscribe && !p.correction.Enabled {
		return nil, coreerrors.NewInternalError("transcribe without correction should be handled by direct processing", nil)
	}
	if req.Task == prompt.TaskTranslate && p.correction.Enabled && !p.correction.MergeWithTranslation {
		return nil, coreerrors.NewInternalError("separated correction+translation should be handled by direct processing", nil)
	}

	dictionary := correction.MergeDictionaries(p.correction.GlobalDictionary, req.UserDictionary)

	// 5. Stage 2/3: 构建 LLM 提示词
	var promptObj *prompt.Prompt
	switch req.Task {
	case prompt.TaskTranscribe:
		promptObj, err = p.promptEngine.BuildTextCorrectPrompt(ctx, asrResp.Text, dictionary)
	case prompt.TaskTranslate:
		if p.correction.Enabled && p.correction.MergeWithTranslation {
			promptObj, err = p.promptEngine.BuildTextCorrectTranslatePrompt(ctx, asrResp.Text, targetLangCodes, dictionary)
		} else {
			promptObj, err = p.promptEngine.BuildTextPrompt(ctx, prompt.PromptRequest{
				Task:            prompt.TaskTranslate,
				SourceLanguage:  req.SourceLanguage,
				TargetLanguages: targetLangCodes,
				Variables: map[string]interface{}{
					"source_text": asrResp.Text,
				},
			})
		}
	default:
		return nil, coreerrors.NewValidationError(fmt.Sprintf("unsupported task type: %s", req.Task), nil)
	}
	if err != nil {
		var appErr *coreerrors.AppError
		if errors.As(err, &appErr) {
			return nil, appErr
		}
		return nil, coreerrors.NewInternalError("build prompt failed", err)
	}

	llmReq := &llm.LLMRequest{
		SystemPrompt: promptObj.System,
		UserPrompt:   promptObj.User,
		Options:      req.Options,
		Context: map[string]interface{}{
			"asr_text":               asrResp.Text,
			"asr_language":           asrResp.DetectedLanguage,
			"asr_duration_ms":        time.Since(asrStart).Milliseconds(),
			"audio_original_format":  req.AudioFormat,
			"audio_processed_format": audioFormat,
			"conversion_applied":     audioFormat != req.AudioFormat,
		},
	}

	return llmReq, nil
}

// ProcessDirect optionally handles requests without going through ProcessingService's single-LLM-call flow.
func (p *Processor) ProcessDirect(ctx context.Context, req ProcessRequest) (*ProcessResponse, bool, error) {
	resp, err := p.processWithPipeline(ctx, req)
	if err != nil {
		return nil, true, err
	}
	return resp, true, nil
}

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

func (p *Processor) processWithPipeline(ctx context.Context, req ProcessRequest) (*ProcessResponse, error) {
	requestID, _ := logging.RequestIDFromContext(ctx)

	if err := p.ensurePipelineInitialized(); err != nil {
		return nil, err
	}

	// Validate audio data (best-effort).
	if err := p.audioConverter.ValidateAudioData(req.Audio, req.AudioFormat); err != nil {
		entry := p.logger.WithError(err)
		if requestID != "" {
			entry = entry.WithField(logging.FieldRequestID, requestID)
		}
		entry.Warn("Audio validation failed, proceeding anyway")
	}

	// Convert audio if needed.
	audioData := req.Audio
	audioFormat := req.AudioFormat
	conversionApplied := false

	if p.audioConverter.IsConversionNeeded(req.AudioFormat) {
		convertedData, err := p.audioConverter.ConvertToWAV(req.Audio, req.AudioFormat)
		if err != nil {
			p.logger.WithError(err).Warn("Audio conversion failed, using original format")
		} else {
			req.Cleanup()
			audioData = convertedData
			audioFormat = "wav"
			conversionApplied = true
			fields := logrus.Fields{
				"original_format":  req.AudioFormat,
				"converted_format": "wav",
			}
			if requestID != "" {
				fields[logging.FieldRequestID] = requestID
			}
			p.logger.WithFields(fields).Info("Audio converted successfully")
		}
	}

	targetLangCodes := req.TargetLanguages
	if req.Task == prompt.TaskTranslate && len(targetLangCodes) == 0 {
		targetLangCodes = p.config.Defaults.TargetLanguages
	}

	dictionary := correction.MergeDictionaries(p.correction.GlobalDictionary, req.UserDictionary)

	// Select pipeline based on task + correction config.
	var selected pipeline.Pipeline
	switch req.Task {
	case prompt.TaskTranscribe:
		if p.correction.Enabled {
			selected = pipeline.TranscribeCorrect()
		} else {
			selected = pipeline.Transcribe()
		}
	case prompt.TaskTranslate:
		if p.correction.Enabled {
			if p.correction.MergeWithTranslation {
				selected = pipeline.TranslateMerged()
			} else {
				selected = pipeline.TranslateSplit()
			}
		} else {
			selected = pipeline.Translate()
		}
	default:
		return nil, coreerrors.NewValidationError(fmt.Sprintf("unsupported task type: %s", req.Task), nil)
	}

	pctx := &tool.PipelineContext{
		RequestID: requestID,
		OriginalRequest: map[string]interface{}{
			"audio":                 audioData,
			"audio_format":          audioFormat,
			"audio_original_format": req.AudioFormat,
			"source_language":       req.SourceLanguage,
			"target_languages":      targetLangCodes,
			"task":                  string(req.Task),
			"options":               req.Options,
		},
		Dictionary: dictionary,
	}

	outCtx, err := p.pipelineExec.Execute(ctx, selected, pctx)
	if err != nil {
		return nil, err
	}

	asrOut := outCtx.StepOutputs["asr_result"].Data
	transcription, _ := asrOut["text"].(string)
	asrLanguage, _ := asrOut["language"].(string)

	resp := acquireProcessResponse()
	resp.RequestID = generateRequestID()
	resp.Status = "success"
	resp.Transcription = transcription
	resp.Metadata["pipeline"] = selected.Name
	resp.Metadata["asr_language"] = asrLanguage
	resp.Metadata["asr_duration_ms"] = outCtx.Metrics["asr_result"].Milliseconds()
	resp.Metadata["original_format"] = req.AudioFormat
	resp.Metadata["processed_format"] = audioFormat
	resp.Metadata["conversion_applied"] = conversionApplied

	stepDurations := make(map[string]int64)
	for k, d := range outCtx.Metrics {
		stepDurations[k] = d.Milliseconds()
	}
	resp.Metadata["step_durations_ms"] = stepDurations

	// Extract corrected text / translations based on pipeline.
	switch selected.Name {
	case pipeline.PipelineTranscribe:
		// ASR only.
	case pipeline.PipelineTranscribeCorrect:
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
	case pipeline.PipelineTranslateMerged:
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
	case pipeline.PipelineTranslate:
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
	case pipeline.PipelineTranslateSplit:
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

		// Keep "backend"/token fields aligned with the translation stage.
		for k, v := range translateOut.Metadata {
			resp.Metadata[k] = v
		}
	default:
		return nil, coreerrors.NewInternalError(fmt.Sprintf("unknown pipeline: %s", selected.Name), nil)
	}

	sourceLangForMetrics := req.SourceLanguage
	if sourceLangForMetrics == "" {
		sourceLangForMetrics = asrLanguage
	}

	if req.Task == prompt.TaskTranscribe && resp.Transcription != "" {
		metrics.IncTranscription(sourceLangForMetrics)
	}
	if req.Task == prompt.TaskTranslate {
		for code := range resp.Translations {
			metrics.IncTranslation(sourceLangForMetrics, code)
		}
	}

	return resp, nil
}

func (p *Processor) transcribe(ctx context.Context, req ProcessRequest) (*asr.ASRResponse, string, bool, time.Duration, error) {
	if p.asrManager == nil {
		return nil, "", false, 0, coreerrors.NewInternalError("asr manager not configured", nil)
	}

	audioData := req.Audio
	audioFormat := req.AudioFormat
	conversionApplied := false

	if p.audioConverter.IsConversionNeeded(req.AudioFormat) {
		convertedData, err := p.audioConverter.ConvertToWAV(req.Audio, req.AudioFormat)
		if err != nil {
			p.logger.WithError(err).Warn("Audio conversion failed, using original format")
		} else {
			req.Cleanup()
			audioData = convertedData
			audioFormat = "wav"
			conversionApplied = true
		}
	}

	start := time.Now()
	asrResp, err := p.asrManager.Transcribe(ctx, &asr.ASRRequest{
		Audio:       audioData,
		AudioFormat: audioFormat,
		Language:    req.SourceLanguage,
	})
	if err != nil {
		return nil, audioFormat, conversionApplied, time.Since(start), err
	}
	return asrResp, audioFormat, conversionApplied, time.Since(start), nil
}

// BuildSuccessResponse 构建成功响应 - 实现 LogicHandler 接口
func (p *Processor) BuildSuccessResponse(llmResp *llm.LLMResponse, parsedResp *prompt.ParsedResponse, req ProcessRequest) *ProcessResponse {
	requestID := generateRequestID()

	response := acquireProcessResponse()
	response.RequestID = requestID
	response.Status = "success"
	response.RawResponse = llmResp.Content
	response.ProcessingTime = 0 // 这将在 Service 中设置
	response.Metadata["model"] = llmResp.Model
	response.Metadata["prompt_tokens"] = llmResp.PromptTokens
	response.Metadata["total_tokens"] = llmResp.TotalTokens
	response.Metadata["backend"] = llmResp.Metadata["backend"]
	response.Metadata["original_format"] = req.AudioFormat
	response.Transcription = ""
	response.CorrectedText = ""

	// 添加解析器信息到元数据
	if parsedResp != nil && parsedResp.Metadata != nil {
		if parser, ok := parsedResp.Metadata["parser"]; ok {
			response.Metadata["parser"] = parser
		}
		if parseSuccess, ok := parsedResp.Metadata["parse_success"]; ok {
			response.Metadata["parse_success"] = parseSuccess
		}
	}

	// 如果解析失败，标记为部分成功
	if parsedResp == nil || parsedResp.Metadata["parse_error"] != nil {
		response.Status = "partial_success"
	}

	// 提取 ASR 转录
	if llmResp != nil && llmResp.Metadata != nil {
		if ctxAny, ok := llmResp.Metadata["context"]; ok {
			if ctxMap, ok := ctxAny.(map[string]interface{}); ok {
				if v, ok := ctxMap["asr_text"].(string); ok {
					response.Transcription = v
				}
				if v, ok := ctxMap["asr_language"].(string); ok && v != "" {
					response.Metadata["asr_language"] = v
				}
				if v, ok := ctxMap["asr_duration_ms"]; ok {
					response.Metadata["asr_duration_ms"] = v
				}
				if v, ok := ctxMap["audio_processed_format"]; ok {
					response.Metadata["processed_format"] = v
				}
				if v, ok := ctxMap["conversion_applied"]; ok {
					response.Metadata["conversion_applied"] = v
				}
			}
		}
	}

	if parsedResp != nil && parsedResp.CorrectedText != "" {
		response.CorrectedText = parsedResp.CorrectedText
	}

	sourceLangForMetrics := req.SourceLanguage
	if sourceLangForMetrics == "" {
		if v, ok := response.Metadata["asr_language"].(string); ok {
			sourceLangForMetrics = v
		}
	}
	if req.Task == prompt.TaskTranscribe && response.Transcription != "" {
		metrics.IncTranscription(sourceLangForMetrics)
	}

	// 提取翻译结果
	targetLangCodes := req.TargetLanguages
	if parsedResp != nil {
		for langCode, translationText := range parsedResp.Sections {
			// 验证这是一个我们期望的目标语言代码
			isTargetLang := false
			for _, targetCode := range targetLangCodes {
				if langCode == targetCode {
					isTargetLang = true
					break
				}
			}
			if isTargetLang {
				response.Translations[langCode] = translationText
				if req.Task == prompt.TaskTranslate {
					metrics.IncTranslation(sourceLangForMetrics, langCode)
				}
			} else {
				p.logger.Warnf("Unexpected section key '%s' found after parsing, not adding to translations.", langCode)
			}
		}
	}

	return response
}

// generateRequestID 生成请求ID
func generateRequestID() string {
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}

// GetSupportedFormats 获取支持的音频格式
func (p *Processor) GetSupportedFormats() []string {
	return p.audioConverter.GetSupportedFormats()
}

// IsFFmpegAvailable reports whether FFmpeg is available for audio conversion.
func (p *Processor) IsFFmpegAvailable() bool {
	return p.audioConverter.IsFFmpegAvailable()
}

// GetSupportedLanguages 获取支持的语言列表
func (p *Processor) GetSupportedLanguages() []map[string]interface{} {
	languages := p.promptEngine.GetLanguages()
	result := make([]map[string]interface{}, 0, len(languages))

	for _, lang := range languages {
		langInfo := map[string]interface{}{
			"code":    lang.Code,
			"type":    lang.Type,
			"aliases": lang.Aliases,
		}

		// 添加所有名称信息
		for key, value := range lang.Names {
			langInfo[key] = value
		}

		if lang.StyleNote != "" {
			langInfo["style_note"] = lang.StyleNote
		}

		result = append(result, langInfo)
	}

	return result
}

// GetCapabilities 获取处理器能力
func (p *Processor) GetCapabilities() map[string]interface{} {
	// 从prompt engine获取支持的语言代码
	languages := p.promptEngine.GetLanguages()
	languageCodes := make([]string, 0, len(languages))
	for code := range languages {
		languageCodes = append(languageCodes, code)
	}

	capabilities := map[string]interface{}{
		"supported_formats":   p.GetSupportedFormats(),
		"max_audio_size":      32 * 1024 * 1024, // 32MB
		"supported_tasks":     []string{"translate", "transcribe"},
		"supported_languages": languageCodes,
		"audio_conversion":    p.audioConverter.IsFFmpegAvailable(),
	}

	// 添加音频转换器的详细指标
	converterMetrics := p.audioConverter.GetMetrics()
	capabilities["conversion_metrics"] = converterMetrics

	return capabilities
}

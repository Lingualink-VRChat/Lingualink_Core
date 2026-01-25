package audio

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	coreerrors "github.com/Lingualink-VRChat/Lingualink_Core/internal/core/errors"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/llm"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/prompt"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/logging"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/metrics"
	"github.com/sirupsen/logrus"
)

// ProcessRequest 音频处理请求
type ProcessRequest struct {
	Audio           []byte          `json:"audio"`
	AudioFormat     string          `json:"audio_format"`
	Task            prompt.TaskType `json:"task"`
	SourceLanguage  string          `json:"source_language,omitempty"`
	TargetLanguages []string        `json:"target_languages"` // 接收短代码
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
	llmManager     *llm.Manager
	promptEngine   *prompt.Engine
	audioConverter *AudioConverter
	metrics        metrics.MetricsCollector
	config         config.PromptConfig
	logger         *logrus.Logger
}

// NewProcessor 创建音频处理器
func NewProcessor(
	llmManager *llm.Manager,
	promptEngine *prompt.Engine,
	cfg config.PromptConfig,
	logger *logrus.Logger,
	metricsCollector metrics.MetricsCollector,
) *Processor {
	return &Processor{
		llmManager:     llmManager,
		promptEngine:   promptEngine,
		audioConverter: NewAudioConverter(logger),
		metrics:        metricsCollector,
		config:         cfg,
		logger:         logger,
	}
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

	// 2. 验证音频数据
	if err := p.audioConverter.ValidateAudioData(req.Audio, req.AudioFormat); err != nil {
		entry := p.logger.WithError(err)
		if requestID != "" {
			entry = entry.WithField(logging.FieldRequestID, requestID)
		}
		entry.Warn("Audio validation failed, proceeding anyway")
	}

	// 3. 转换音频格式（如果需要）
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

	// 4. 处理目标语言（使用短代码）
	targetLangCodes := req.TargetLanguages
	// 只有在translate任务且没有指定目标语言时，才使用默认目标语言
	if req.Task == prompt.TaskTranslate && len(targetLangCodes) == 0 {
		targetLangCodes = p.config.Defaults.TargetLanguages // 从配置获取默认目标语言
	}

	// 5. 构建提示词（prompt引擎会将短代码转换为中文显示名称）
	promptReq := prompt.PromptRequest{
		Task:            req.Task,
		SourceLanguage:  req.SourceLanguage,
		TargetLanguages: targetLangCodes, // 传入短代码
	}

	promptObj, err := p.promptEngine.Build(ctx, promptReq)
	if err != nil {
		var appErr *coreerrors.AppError
		if errors.As(err, &appErr) {
			return nil, appErr
		}
		return nil, coreerrors.NewInternalError("build prompt failed", err)
	}

	// 6. 构建LLM请求
	llmReq := &llm.LLMRequest{
		SystemPrompt: promptObj.System,
		UserPrompt:   promptObj.User,
		Audio:        audioData,
		AudioFormat:  audioFormat,
	}

	return llmReq, nil
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
	response.Metadata["processed_format"] = "wav" // 假设已转换为 WAV
	response.Metadata["conversion_applied"] = p.audioConverter.IsConversionNeeded(req.AudioFormat)
	response.Transcription = ""

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

	// 提取转录内容（原文）
	if parsedResp != nil && parsedResp.Sections["原文"] != "" {
		response.Transcription = parsedResp.Sections["原文"]
	}
	if req.Task == prompt.TaskTranscribe && response.Transcription != "" {
		metrics.IncTranscription(req.SourceLanguage)
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
					metrics.IncTranslation(req.SourceLanguage, langCode)
				}
			} else if langCode != "原文" { // "原文"已经处理过了
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
			"aliases": lang.Aliases,
		}

		// 添加所有名称信息
		for key, value := range lang.Names {
			langInfo[key] = value
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

package audio

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/llm"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/prompt"
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
}

// GetTargetLanguages 实现 ProcessableRequest 接口
func (req ProcessRequest) GetTargetLanguages() []string {
	return req.TargetLanguages
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
		return fmt.Errorf("audio data is required")
	}

	if req.AudioFormat == "" {
		return fmt.Errorf("audio format is required")
	}

	// 验证音频大小限制（32MB）
	maxSize := 32 * 1024 * 1024
	if len(req.Audio) > maxSize {
		return fmt.Errorf("audio size (%d bytes) exceeds maximum allowed size (%d bytes)", len(req.Audio), maxSize)
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
		return fmt.Errorf("unsupported audio format: %s", req.AudioFormat)
	}

	// 验证任务类型
	validTasks := map[prompt.TaskType]bool{
		prompt.TaskTranslate:  true,
		prompt.TaskTranscribe: true,
		// 保留用于后续扩展
		// prompt.TaskTranscribeAndTranslate: true,
	}

	if !validTasks[req.Task] {
		return fmt.Errorf("invalid task type: %s", req.Task)
	}

	return nil
}

// BuildLLMRequest 构建LLM请求 - 实现 LogicHandler 接口
func (p *Processor) BuildLLMRequest(ctx context.Context, req ProcessRequest) (*llm.LLMRequest, *prompt.OutputRules, error) {
	// 2. 验证音频数据
	if err := p.audioConverter.ValidateAudioData(req.Audio, req.AudioFormat); err != nil {
		p.logger.WithError(err).Warn("Audio validation failed, proceeding anyway")
	}

	// 3. 转换音频格式（如果需要）
	audioData := req.Audio
	audioFormat := req.AudioFormat

	if p.audioConverter.IsConversionNeeded(req.AudioFormat) {
		convertedData, err := p.audioConverter.ConvertToWAV(req.Audio, req.AudioFormat)
		if err != nil {
			p.logger.WithError(err).Warn("Audio conversion failed, using original format")
		} else {
			audioData = convertedData
			audioFormat = "wav"
			p.logger.WithFields(logrus.Fields{
				"original_format":  req.AudioFormat,
				"converted_format": "wav",
			}).Info("Audio converted successfully")
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
		return nil, nil, fmt.Errorf("build prompt: %w", err)
	}

	// 6. 构建LLM请求
	llmReq := &llm.LLMRequest{
		SystemPrompt: promptObj.System,
		UserPrompt:   promptObj.User,
		Audio:        audioData,
		AudioFormat:  audioFormat,
	}

	return llmReq, &promptObj.OutputRules, nil
}

// BuildSuccessResponse 构建成功响应 - 实现 LogicHandler 接口
func (p *Processor) BuildSuccessResponse(llmResp *llm.LLMResponse, parsedResp *prompt.ParsedResponse, req ProcessRequest) *ProcessResponse {
	requestID := generateRequestID()

	response := &ProcessResponse{
		RequestID:      requestID,
		Status:         "success",
		RawResponse:    llmResp.Content,
		ProcessingTime: 0, // 这将在 Service 中设置
		Metadata: map[string]interface{}{
			"model":              llmResp.Model,
			"prompt_tokens":      llmResp.PromptTokens,
			"total_tokens":       llmResp.TotalTokens,
			"backend":            llmResp.Metadata["backend"],
			"original_format":    req.AudioFormat,
			"processed_format":   "wav", // 假设已转换为 WAV
			"conversion_applied": p.audioConverter.IsConversionNeeded(req.AudioFormat),
		},
		Translations: make(map[string]string),
	}

	// 如果解析失败，标记为部分成功
	if parsedResp == nil || parsedResp.Metadata["parse_error"] != nil {
		response.Status = "partial_success"
	}

	// 提取转录内容（原文）
	if parsedResp != nil && parsedResp.Sections["原文"] != "" {
		response.Transcription = parsedResp.Sections["原文"]
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
			} else if langCode != "原文" { // "原文"已经处理过了
				p.logger.Warnf("Unexpected section key '%s' found after parsing, not adding to translations.", langCode)
			}
		}
	}

	return response
}

// ApplyFallback 应用更智能的回退逻辑 - 实现 LogicHandler 接口
func (p *Processor) ApplyFallback(response *ProcessResponse, rawContent string, outputRules *prompt.OutputRules) {
	// --- Tier 0: 清理和预检 ---

	// 1. 收集所有可能的提示词"脚手架" (keywords)
	var keywords []string
	if outputRules != nil {
		for _, section := range outputRules.Sections {
			keywords = append(keywords, section.Key)
			keywords = append(keywords, section.Aliases...)
		}
	}
	// 添加通用分隔符
	separators := []string{":", "：", "\n"}

	// 2. 从原始响应中剥离所有已知的脚手架和分隔符
	cleanedContent := rawContent
	for _, keyword := range keywords {
		cleanedContent = strings.ReplaceAll(cleanedContent, keyword, "")
	}
	for _, sep := range separators {
		cleanedContent = strings.ReplaceAll(cleanedContent, sep, "")
	}
	cleanedContent = strings.TrimSpace(cleanedContent)

	// 3. 如果清理后内容为空，则说明LLM没有提供任何有用信息，直接返回
	if cleanedContent == "" {
		p.logger.WithFields(logrus.Fields{
			"raw_response": rawContent,
		}).Warn("Fallback aborted: LLM response contained only prompt artifacts.")
		// 如果解析器未能解析出任何内容，并且原始响应只包含脚手架，
		// 那么就保持 transcription 和 translations 为空，这是最准确的状态。
		// 我们可以根据情况决定是否将 status 改为 failed。
		if response.Transcription == "" && len(response.Translations) == 0 {
			response.Status = "failed"
			if response.Metadata == nil {
				response.Metadata = make(map[string]interface{})
			}
			response.Metadata["error_reason"] = "LLM returned an empty or artifact-only response."
		}
		return
	}

	// 如果执行到这里，说明 `cleanedContent` 包含一些未知但可能有效的内容。
	var fallbackReasons []string

	// --- Tier 1: 填充缺失的转录内容 ---
	if response.Transcription == "" {
		response.Transcription = cleanedContent
		fallbackReasons = append(fallbackReasons, "using sanitized raw content as transcription")
		p.logger.WithField("content", cleanedContent).Info("Applied fallback for transcription.")
	}

	// --- Tier 2: 填充缺失的翻译内容 (仅在 translate 任务中) ---
	// 从 outputRules 中获取目标语言
	var targetLangCodes []string
	if outputRules != nil {
		for _, section := range outputRules.Sections {
			if section.LanguageCode != "" {
				targetLangCodes = append(targetLangCodes, section.LanguageCode)
			}
		}
	}

	if len(targetLangCodes) > 0 && len(response.Translations) == 0 {
		// 启发式规则：如果转录内容和清理后的内容相同，很可能LLM只返回了转录。
		// 如果不同，则认为清理后的内容是第一个目标语言的翻译。
		if response.Transcription != cleanedContent {
			response.Translations[targetLangCodes[0]] = cleanedContent
			fallbackReasons = append(fallbackReasons, fmt.Sprintf("using sanitized raw content as translation for %s", targetLangCodes[0]))
			p.logger.WithFields(logrus.Fields{
				"target_language": targetLangCodes[0],
				"content":         cleanedContent,
			}).Info("Applied fallback for translation.")
		}
	}

	// 如果应用了任何回退逻辑，更新状态和元数据
	if len(fallbackReasons) > 0 {
		response.Status = "partial_success"
		if response.Metadata == nil {
			response.Metadata = make(map[string]interface{})
		}
		response.Metadata["fallback_mode"] = true
		response.Metadata["fallback_reason"] = strings.Join(fallbackReasons, "; ")
		p.logger.WithField("reasons", fallbackReasons).Info("Fallback logic successfully applied.")
	}
}

// generateRequestID 生成请求ID
func generateRequestID() string {
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}

// GetSupportedFormats 获取支持的音频格式
func (p *Processor) GetSupportedFormats() []string {
	return p.audioConverter.GetSupportedFormats()
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

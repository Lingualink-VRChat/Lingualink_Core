package audio

import (
	"context"
	"fmt"
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

// Process 处理音频
func (p *Processor) Process(ctx context.Context, req ProcessRequest) (*ProcessResponse, error) {
	startTime := time.Now()
	requestID := generateRequestID()

	// 记录指标
	defer func() {
		duration := time.Since(startTime)
		p.metrics.RecordLatency("audio.process", duration, map[string]string{
			"task":   string(req.Task),
			"format": req.AudioFormat,
		})
	}()

	p.logger.WithFields(logrus.Fields{
		"request_id": requestID,
		"task":       req.Task,
		"format":     req.AudioFormat,
		"audio_size": len(req.Audio),
	}).Info("Processing audio request")

	// 1. 验证请求
	if err := p.validateRequest(req); err != nil {
		return nil, fmt.Errorf("validate request: %w", err)
	}

	// 2. 验证音频数据
	if err := p.audioConverter.ValidateAudioData(req.Audio, req.AudioFormat); err != nil {
		p.logger.WithError(err).Warn("Audio validation failed, proceeding anyway")
	}

	// 3. 音频格式转换（如果需要）
	audioData := req.Audio
	audioFormat := req.AudioFormat
	if p.audioConverter.IsConversionNeeded(req.AudioFormat) {
		p.logger.WithField("original_format", req.AudioFormat).Info("Converting audio to WAV format")

		convertedData, err := p.audioConverter.ConvertToWAV(req.Audio, req.AudioFormat)
		if err != nil {
			p.logger.WithError(err).Error("Audio conversion failed")
			// 如果转换失败，尝试使用原始音频
			p.logger.Warn("Using original audio format due to conversion failure")
		} else {
			audioData = convertedData
			audioFormat = "wav"
			p.logger.WithFields(logrus.Fields{
				"original_size":  len(req.Audio),
				"converted_size": len(convertedData),
			}).Info("Audio conversion successful")
		}
	}

	// 4. 处理目标语言（使用短代码）
	targetLangCodes := req.TargetLanguages
	if len(targetLangCodes) == 0 {
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
		return nil, fmt.Errorf("build prompt: %w", err)
	}

	// 6. 调用LLM
	llmReq := &llm.LLMRequest{
		SystemPrompt: promptObj.System,
		UserPrompt:   promptObj.User,
		Audio:        audioData,
		AudioFormat:  audioFormat,
	}

	llmResp, err := p.llmManager.Process(ctx, llmReq)
	if err != nil {
		p.metrics.RecordCounter("audio.process.error", 1, map[string]string{
			"error_type": "llm_error",
		})
		return nil, fmt.Errorf("llm process: %w", err)
	}

	// 7. 解析响应（ParseResponse会将中文名称键转换为短代码）
	parsed, err := p.promptEngine.ParseResponse(llmResp.Content, promptObj.OutputRules)
	if err != nil {
		p.logger.WithError(err).Warn("Failed to parse LLM response, using raw response")
		// 如果解析失败，仍然返回结果，但标记状态
		parsed = &prompt.ParsedResponse{
			RawText:  llmResp.Content,
			Sections: make(map[string]string),
			Metadata: map[string]interface{}{
				"parse_error": err.Error(),
			},
		}
	}

	// 8. 构建响应
	response := &ProcessResponse{
		RequestID:      requestID,
		Status:         "success",
		RawResponse:    llmResp.Content,
		ProcessingTime: time.Since(startTime).Seconds(),
		Metadata: map[string]interface{}{
			"model":              llmResp.Model,
			"prompt_tokens":      llmResp.PromptTokens,
			"total_tokens":       llmResp.TotalTokens,
			"backend":            llmResp.Metadata["backend"],
			"original_format":    req.AudioFormat,
			"processed_format":   audioFormat,
			"conversion_applied": p.audioConverter.IsConversionNeeded(req.AudioFormat),
		},
		Translations: make(map[string]string),
	}

	// 如果解析失败，标记为部分成功
	if err != nil && response.Status == "success" {
		response.Status = "partial_success"
	}

	// 提取转录
	if transcription, ok := parsed.Sections["原文"]; ok {
		response.Transcription = transcription
		// 从sections中移除，避免在translations中重复出现
		delete(parsed.Sections, "原文")
	}

	// 提取翻译（现在keys是短代码）
	p.logger.WithFields(logrus.Fields{
		"targetLangCodes": targetLangCodes,
		"parsedSections":  parsed.Sections,
	}).Debug("Extracting translations from parsed sections")

	for langCode, translationText := range parsed.Sections {
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

	// 如果没有找到预期的段落，尝试从原始响应中提取
	if response.Transcription == "" && len(response.Translations) == 0 && err != nil {
		p.extractFromRawResponse(response, llmResp.Content)
	}

	// 记录成功指标
	p.metrics.RecordCounter("audio.process.success", 1, map[string]string{
		"task": string(req.Task),
	})

	p.logger.WithFields(logrus.Fields{
		"request_id":         requestID,
		"processing_time":    response.ProcessingTime,
		"transcription_len":  len(response.Transcription),
		"translations_count": len(response.Translations),
	}).Info("Audio processing completed")

	return response, nil
}

// validateRequest 验证请求
func (p *Processor) validateRequest(req ProcessRequest) error {
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
		prompt.TaskTranslate: true,
		// 保留用于后续扩展
		// prompt.TaskTranscribe: true,
		// prompt.TaskTranscribeAndTranslate: true,
	}

	if !validTasks[req.Task] {
		return fmt.Errorf("invalid task type: %s", req.Task)
	}

	return nil
}

// extractFromRawResponse 从原始响应中提取内容
func (p *Processor) extractFromRawResponse(response *ProcessResponse, rawContent string) {
	// 简单的回退逻辑：如果解析失败，将整个响应作为转录
	if response.Transcription == "" {
		response.Transcription = rawContent
		response.Status = "partial_success"

		if response.Metadata == nil {
			response.Metadata = make(map[string]interface{})
		}
		response.Metadata["fallback_mode"] = true
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
		"supported_tasks":     []string{"translate"},
		"supported_languages": languageCodes,
		"audio_conversion":    p.audioConverter.IsFFmpegAvailable(),
	}

	// 添加音频转换器的详细指标
	converterMetrics := p.audioConverter.GetMetrics()
	capabilities["conversion_metrics"] = converterMetrics

	return capabilities
}

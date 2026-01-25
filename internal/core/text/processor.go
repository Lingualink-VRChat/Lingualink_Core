package text

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/cache"
	coreerrors "github.com/Lingualink-VRChat/Lingualink_Core/internal/core/errors"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/llm"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/prompt"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/metrics"
	"github.com/sirupsen/logrus"
)

// ProcessRequest 文本处理请求
type ProcessRequest struct {
	Text            string                 `json:"text"`
	SourceLanguage  string                 `json:"source_language,omitempty"`
	TargetLanguages []string               `json:"target_languages"`
	Options         map[string]interface{} `json:"options,omitempty"`
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
	logger       *logrus.Logger

	translationCache cache.TranslationCache
	cacheTTL         time.Duration
}

// NewProcessor 创建文本处理器
func NewProcessor(
	llmManager *llm.Manager,
	promptEngine *prompt.Engine,
	metrics metrics.MetricsCollector,
	config config.PromptConfig,
	logger *logrus.Logger,
) *Processor {
	return NewProcessorWithCache(llmManager, promptEngine, metrics, config, logger, nil, 0)
}

// NewProcessorWithCache creates a text Processor with an optional translation cache.
func NewProcessorWithCache(
	llmManager *llm.Manager,
	promptEngine *prompt.Engine,
	metrics metrics.MetricsCollector,
	config config.PromptConfig,
	logger *logrus.Logger,
	translationCache cache.TranslationCache,
	cacheTTL time.Duration,
) *Processor {
	return &Processor{
		llmManager:       llmManager,
		promptEngine:     promptEngine,
		metrics:          metrics,
		config:           config,
		logger:           logger,
		translationCache: translationCache,
		cacheTTL:         cacheTTL,
	}
}

// Process 方法已移除 - 现在使用 ProcessingService 统一处理流程

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
		return coreerrors.NewValidationError("target languages are required", nil)
	}

	return nil
}

// BuildLLMRequest 构建LLM请求 - 实现 LogicHandler 接口
func (p *Processor) BuildLLMRequest(ctx context.Context, req ProcessRequest) (*llm.LLMRequest, error) {
	// 2. 处理目标语言
	targetLangCodes := req.TargetLanguages
	if len(targetLangCodes) == 0 {
		targetLangCodes = p.config.Defaults.TargetLanguages
	}

	// 3. 构建提示词
	promptReq := prompt.PromptRequest{
		Task:            prompt.TaskTranslate,
		SourceLanguage:  req.SourceLanguage,
		TargetLanguages: targetLangCodes,
		Variables: map[string]interface{}{
			"source_text": req.Text,
		},
	}

	promptObj, err := p.promptEngine.BuildTextPrompt(ctx, promptReq)
	if err != nil {
		var appErr *coreerrors.AppError
		if errors.As(err, &appErr) {
			return nil, appErr
		}
		return nil, coreerrors.NewInternalError("build prompt failed", err)
	}

	// 4. 构建LLM请求
	llmReq := &llm.LLMRequest{
		SystemPrompt: promptObj.System,
		UserPrompt:   promptObj.User,
		// 文本处理不需要音频数据
	}

	return llmReq, nil
}

func (p *Processor) TryGetCachedResponse(ctx context.Context, req ProcessRequest) (*ProcessResponse, bool, error) {
	if p.translationCache == nil || p.cacheTTL <= 0 {
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

// BuildSuccessResponse 构建成功响应 - 实现 LogicHandler 接口
func (p *Processor) BuildSuccessResponse(llmResp *llm.LLMResponse, parsedResp *prompt.ParsedResponse, req ProcessRequest) *ProcessResponse {
	requestID := generateRequestID()

	response := acquireProcessResponse()
	response.RequestID = requestID
	response.Status = "success"
	response.SourceText = req.Text
	response.RawResponse = llmResp.Content
	response.ProcessingTime = 0 // 这将在 Service 中设置
	response.Metadata["model"] = llmResp.Model
	response.Metadata["prompt_tokens"] = llmResp.PromptTokens
	response.Metadata["total_tokens"] = llmResp.TotalTokens
	response.Metadata["backend"] = llmResp.Metadata["backend"]

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

	// 7. 提取翻译结果
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
				metrics.IncTranslation(req.SourceLanguage, langCode)
			}
		}
	}

	return response
}

// generateRequestID 生成请求ID
func generateRequestID() string {
	return fmt.Sprintf("txt_%d", time.Now().UnixNano())
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
			"aliases": lang.Aliases,
		}
		result = append(result, langInfo)
	}

	return result
}

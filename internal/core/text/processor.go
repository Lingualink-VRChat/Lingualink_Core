package text

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/cache"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/correction"
	coreerrors "github.com/Lingualink-VRChat/Lingualink_Core/internal/core/errors"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/llm"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/prompt"
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
		llmManager:       llmManager,
		promptEngine:     promptEngine,
		metrics:          metrics,
		config:           promptCfg,
		correction:       config.CorrectionConfig{},
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

// BuildLLMRequest 构建LLM请求 - 实现 LogicHandler 接口
func (p *Processor) BuildLLMRequest(ctx context.Context, req ProcessRequest) (*llm.LLMRequest, error) {
	task := req.Task
	if task == "" {
		task = prompt.TaskTranslate
	}

	// 2. 处理目标语言
	targetLangCodes := req.TargetLanguages
	if task == prompt.TaskTranslate && len(targetLangCodes) == 0 {
		targetLangCodes = p.config.Defaults.TargetLanguages
	}

	dictionary := correction.MergeDictionaries(p.correction.GlobalDictionary, req.UserDictionary)

	var promptObj *prompt.Prompt
	var err error
	switch task {
	case prompt.TaskTranslate:
		if p.correction.Enabled && !p.correction.MergeWithTranslation {
			return nil, coreerrors.NewInternalError("separated correction+translation should be handled by direct processing", nil)
		}

		if p.correction.Enabled && p.correction.MergeWithTranslation {
			promptObj, err = p.promptEngine.BuildTextCorrectTranslatePrompt(ctx, req.Text, targetLangCodes, dictionary)
		} else {
			promptObj, err = p.promptEngine.BuildTextPrompt(ctx, prompt.PromptRequest{
				Task:            prompt.TaskTranslate,
				SourceLanguage:  req.SourceLanguage,
				TargetLanguages: targetLangCodes,
				Variables: map[string]interface{}{
					"source_text": req.Text,
				},
			})
		}
	case prompt.TaskTranscribe:
		if !p.correction.Enabled {
			return nil, coreerrors.NewInternalError("transcribe without correction should be handled by direct processing", nil)
		}
		promptObj, err = p.promptEngine.BuildTextCorrectPrompt(ctx, req.Text, dictionary)
	default:
		return nil, coreerrors.NewValidationError(fmt.Sprintf("unsupported task type: %s", task), nil)
	}
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
		Options:      req.Options,
	}

	return llmReq, nil
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

	task := req.Task
	if task == "" {
		task = prompt.TaskTranslate
	}
	if parsedResp != nil && parsedResp.CorrectedText != "" {
		response.CorrectedText = parsedResp.CorrectedText
	}

	// 7. 提取翻译结果
	if task == prompt.TaskTranslate {
		targetLangCodes := req.TargetLanguages
		if len(targetLangCodes) == 0 {
			targetLangCodes = p.config.Defaults.TargetLanguages
		}
		if parsedResp != nil {
			for _, targetCode := range targetLangCodes {
				if translationText, ok := parsedResp.Sections[targetCode]; ok && translationText != "" {
					response.Translations[targetCode] = translationText
					metrics.IncTranslation(req.SourceLanguage, targetCode)
				}
			}
		}
	}

	return response
}

// ProcessDirect optionally handles requests that don't fit the single-call ProcessingService flow.
func (p *Processor) ProcessDirect(ctx context.Context, req ProcessRequest) (*ProcessResponse, bool, error) {
	task := req.Task
	if task == "" {
		task = prompt.TaskTranslate
	}

	if task == prompt.TaskTranscribe && !p.correction.Enabled {
		resp := acquireProcessResponse()
		resp.RequestID = generateRequestID()
		resp.Status = "success"
		resp.SourceText = req.Text
		resp.CorrectedText = req.Text
		resp.RawResponse = ""
		return resp, true, nil
	}

	if task == prompt.TaskTranslate && p.correction.Enabled && !p.correction.MergeWithTranslation {
		if p.llmManager == nil {
			return nil, true, coreerrors.NewInternalError("llm manager not configured", nil)
		}
		targetLangCodes := req.TargetLanguages
		if len(targetLangCodes) == 0 {
			targetLangCodes = p.config.Defaults.TargetLanguages
		}
		dictionary := correction.MergeDictionaries(p.correction.GlobalDictionary, req.UserDictionary)

		correctPrompt, err := p.promptEngine.BuildTextCorrectPrompt(ctx, req.Text, dictionary)
		if err != nil {
			return nil, true, err
		}
		correctResp, err := p.llmManager.ProcessWithTimeout(ctx, &llm.LLMRequest{
			SystemPrompt: correctPrompt.System,
			UserPrompt:   correctPrompt.User,
			Options:      req.Options,
		})
		if err != nil {
			return nil, true, err
		}
		parsedCorrect, err := p.promptEngine.ParseResponse(correctResp.Content)
		if err != nil {
			return nil, true, err
		}
		correctedText := strings.TrimSpace(parsedCorrect.CorrectedText)
		if correctedText == "" {
			correctedText = strings.TrimSpace(req.Text)
		}

		translatePrompt, err := p.promptEngine.BuildTextPrompt(ctx, prompt.PromptRequest{
			Task:            prompt.TaskTranslate,
			SourceLanguage:  req.SourceLanguage,
			TargetLanguages: targetLangCodes,
			Variables: map[string]interface{}{
				"source_text": correctedText,
			},
		})
		if err != nil {
			return nil, true, err
		}
		translateResp, err := p.llmManager.ProcessWithTimeout(ctx, &llm.LLMRequest{
			SystemPrompt: translatePrompt.System,
			UserPrompt:   translatePrompt.User,
			Options:      req.Options,
		})
		if err != nil {
			return nil, true, err
		}
		parsedTranslate, err := p.promptEngine.ParseResponse(translateResp.Content)
		if err != nil {
			return nil, true, err
		}

		resp := acquireProcessResponse()
		resp.RequestID = generateRequestID()
		resp.Status = "success"
		resp.SourceText = req.Text
		resp.CorrectedText = correctedText
		resp.RawResponse = translateResp.Content
		resp.Metadata["correction_backend"] = correctResp.Metadata["backend"]
		resp.Metadata["translation_backend"] = translateResp.Metadata["backend"]
		resp.Metadata["raw_correction_response"] = correctResp.Content
		for _, code := range targetLangCodes {
			if v, ok := parsedTranslate.Sections[code]; ok && v != "" {
				resp.Translations[code] = v
				metrics.IncTranslation(req.SourceLanguage, code)
			}
		}
		return resp, true, nil
	}

	return nil, false, nil
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

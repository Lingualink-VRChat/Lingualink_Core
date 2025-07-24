package text

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

// ProcessRequest 文本处理请求
type ProcessRequest struct {
	Text            string                 `json:"text"`
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

// Processor 文本处理器
type Processor struct {
	llmManager   *llm.Manager
	promptEngine *prompt.Engine
	metrics      metrics.MetricsCollector
	config       config.PromptConfig
	logger       *logrus.Logger
}

// NewProcessor 创建文本处理器
func NewProcessor(
	llmManager *llm.Manager,
	promptEngine *prompt.Engine,
	metrics metrics.MetricsCollector,
	config config.PromptConfig,
	logger *logrus.Logger,
) *Processor {
	return &Processor{
		llmManager:   llmManager,
		promptEngine: promptEngine,
		metrics:      metrics,
		config:       config,
		logger:       logger,
	}
}

// Process 方法已移除 - 现在使用 ProcessingService 统一处理流程

// Validate 验证请求 - 实现 LogicHandler 接口
func (p *Processor) Validate(req ProcessRequest) error {
	if req.Text == "" {
		return fmt.Errorf("text is required")
	}

	// 验证文本长度限制（3000字符）
	maxLength := 3000
	if len(req.Text) > maxLength {
		return fmt.Errorf("text length (%d characters) exceeds maximum allowed length (%d characters)", len(req.Text), maxLength)
	}

	if len(req.TargetLanguages) == 0 {
		return fmt.Errorf("target languages are required")
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
		return nil, fmt.Errorf("build prompt: %w", err)
	}

	// 4. 构建LLM请求
	llmReq := &llm.LLMRequest{
		SystemPrompt: promptObj.System,
		UserPrompt:   promptObj.User,
		// 文本处理不需要音频数据
	}

	return llmReq, nil
}

// BuildSuccessResponse 构建成功响应 - 实现 LogicHandler 接口
func (p *Processor) BuildSuccessResponse(llmResp *llm.LLMResponse, parsedResp *prompt.ParsedResponse, req ProcessRequest) *ProcessResponse {
	requestID := generateRequestID()

	response := &ProcessResponse{
		RequestID:      requestID,
		Status:         "success",
		SourceText:     req.Text,
		RawResponse:    llmResp.Content,
		ProcessingTime: 0, // 这将在 Service 中设置
		Metadata: map[string]interface{}{
			"model":         llmResp.Model,
			"prompt_tokens": llmResp.PromptTokens,
			"total_tokens":  llmResp.TotalTokens,
			"backend":       llmResp.Metadata["backend"],
		},
		Translations: make(map[string]string),
	}

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

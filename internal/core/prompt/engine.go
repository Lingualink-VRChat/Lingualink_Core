package prompt

import (
	"context"
	"errors"
	"fmt"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	coreerrors "github.com/Lingualink-VRChat/Lingualink_Core/internal/core/errors"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/metrics"
	"github.com/sirupsen/logrus"
)

// TaskType 任务类型
type TaskType string

const (
	// TaskTranslate indicates a translation task.
	TaskTranslate TaskType = "translate"
	// TaskTranscribe indicates a transcription task.
	TaskTranscribe TaskType = "transcribe"
	// 保留用于后续扩展
	// TaskBoth       TaskType = "both"
)

// OutputFormat 输出格式
type OutputFormat string

const (
	// FormatStructured indicates a structured output format.
	FormatStructured OutputFormat = "structured"
	// FormatJSON indicates a JSON output format.
	FormatJSON OutputFormat = "json"
	// FormatMarkdown indicates a Markdown output format.
	FormatMarkdown OutputFormat = "markdown"
	// FormatPlain indicates a plain text output format.
	FormatPlain OutputFormat = "plain"
)

// PromptRequest 提示词请求
type PromptRequest struct {
	Task            TaskType               `json:"task"`
	SourceLanguage  string                 `json:"source_language"`
	TargetLanguages []string               `json:"target_languages"` // 接收短代码
	Variables       map[string]interface{} `json:"variables"`
	OutputFormat    OutputFormat           `json:"output_format"`
	// 移除 UserPrompt，改为服务端控制
}

// Prompt 生成的提示词
type Prompt struct {
	System      string      `json:"system"`
	User        string      `json:"user"`
	OutputRules OutputRules `json:"output_rules"`
}

// OutputRules 输出规则
type OutputRules struct {
	Format    OutputFormat    `json:"format"`
	Separator string          `json:"separator"`
	Sections  []OutputSection `json:"sections"`
}

// OutputSection 输出段落定义
type OutputSection struct {
	Key          string   `json:"key"`           // LLM输出的中文名称（如"英文"）
	Aliases      []string `json:"aliases"`       // 别名
	LanguageCode string   `json:"language_code"` // 对应的短代码（如"en"）
	Required     bool     `json:"required"`
	Order        int      `json:"order"`
}

// ParsedResponse 解析后的响应
type ParsedResponse struct {
	RawText  string                 `json:"raw_text"`
	Sections map[string]string      `json:"sections"` // 键为短代码或特殊键（如"原文"）
	Metadata map[string]interface{} `json:"metadata"`
}

// 移除 PromptTemplate 和 Language 定义，已移动到单独的文件

// Engine 提示词引擎
type Engine struct {
	templateManager *TemplateManager
	languageManager *LanguageManager
	config          config.PromptConfig
	logger          *logrus.Logger
}

// NewEngine 创建提示词引擎
func NewEngine(cfg config.PromptConfig, logger *logrus.Logger) (*Engine, error) {
	// 创建各个管理器
	templateManager := NewTemplateManager(logger)
	languageManager := NewLanguageManager(cfg, logger)

	engine := &Engine{
		templateManager: templateManager,
		languageManager: languageManager,
		config:          cfg,
		logger:          logger,
	}

	return engine, nil
}

// 移除 loadDefaultTemplate 方法，模板管理已移动到 TemplateManager

// Build 构建音频处理提示词
func (e *Engine) Build(ctx context.Context, req PromptRequest) (*Prompt, error) {
	// 根据任务类型选择模板
	var templateName string
	var targetLanguageNames []string
	var err error

	if req.Task == TaskTranscribe {
		// 转录任务不需要目标语言
		templateName = "audio_transcribe"
	} else if req.Task == TaskTranslate {
		// 翻译任务需要目标语言
		templateName = "audio_translate"
		if len(req.TargetLanguages) > 0 {
			targetLanguageNames, err = e.languageManager.ConvertCodesToDisplayNames(req.TargetLanguages)
			if err != nil {
				var appErr *coreerrors.AppError
				if errors.As(err, &appErr) {
					return nil, appErr
				}
				return nil, coreerrors.NewValidationError("convert target language codes failed", err)
			}
		}
	} else {
		return nil, coreerrors.NewValidationError(fmt.Sprintf("unsupported task type: %s", req.Task), nil)
	}

	// 准备模板数据
	data := map[string]interface{}{
		"Task":                req.Task,
		"SourceLanguage":      req.SourceLanguage,
		"TargetLanguageCodes": req.TargetLanguages, // 保留原始短代码
		"TargetLanguageNames": targetLanguageNames, // 用于模板的中文显示名称
		"Variables":           req.Variables,
	}

	// 使用对应的模板
	prompt, _, err := e.templateManager.BuildPrompt(ctx, templateName, data)
	if err != nil {
		return nil, coreerrors.NewInternalError("build audio prompt failed", err)
	}

	// 动态生成OutputRules，音频处理总是包含源文本
	// transcribe任务不需要翻译段落，translate任务需要
	includeTranslations := req.Task == TaskTranslate
	var targetCodes []string
	if includeTranslations {
		targetCodes = req.TargetLanguages
	}
	dynamicOutputRules := e.languageManager.BuildDynamicOutputRules(req.Task, targetCodes, true)

	prompt.OutputRules = dynamicOutputRules
	return prompt, nil
}

// BuildTextPrompt 构建文本翻译提示词
func (e *Engine) BuildTextPrompt(ctx context.Context, req PromptRequest) (*Prompt, error) {
	// 将短代码转换为中文显示名称用于构建LLM prompt
	targetLanguageNames, err := e.languageManager.ConvertCodesToDisplayNames(req.TargetLanguages)
	if err != nil {
		var appErr *coreerrors.AppError
		if errors.As(err, &appErr) {
			return nil, appErr
		}
		return nil, coreerrors.NewValidationError("convert target language codes failed", err)
	}

	// 准备模板数据
	data := map[string]interface{}{
		"Task":                req.Task,
		"SourceLanguage":      req.SourceLanguage,
		"TargetLanguageCodes": req.TargetLanguages, // 保留原始短代码
		"TargetLanguageNames": targetLanguageNames, // 用于模板的中文显示名称
		"Variables":           req.Variables,
		"SourceText":          req.Variables["source_text"], // 源文本
	}

	// 使用文本翻译模板
	prompt, _, err := e.templateManager.BuildPrompt(ctx, "text_translate", data)
	if err != nil {
		return nil, coreerrors.NewInternalError("build text prompt failed", err)
	}

	// 动态生成OutputRules，不包含源文本段落（因为文本翻译不需要转录）
	dynamicOutputRules := e.languageManager.BuildDynamicOutputRules(req.Task, req.TargetLanguages, false)

	prompt.OutputRules = dynamicOutputRules
	return prompt, nil
}

// GetLanguages 获取所有支持的语言
func (e *Engine) GetLanguages() map[string]*Language {
	return e.languageManager.GetLanguages()
}

// ParseResponse 解析LLM响应 - 仅支持 JSON 解析
func (e *Engine) ParseResponse(content string) (*ParsedResponse, error) {
	// 只进行 JSON 块解析，失败时直接返回错误
	jsonData, ok := extractJSONBlock(content)
	if !ok {
		metrics.ObserveJSONParseSuccess("json", false)
		e.logger.WithField("content", content).Error("No JSON block found in LLM response")
		return nil, coreerrors.NewParsingError("no json block found in response", nil)
	}

	parsedResp, err := parseJSONResponse(jsonData)
	if err != nil {
		metrics.ObserveJSONParseSuccess("json", false)
		e.logger.WithError(err).WithField("jsonData", string(jsonData)).Error("Failed to parse JSON response")
		return nil, coreerrors.NewParsingError("invalid json in response", err)
	}

	metrics.ObserveJSONParseSuccess("json", true)
	e.logger.WithFields(map[string]interface{}{
		"parser":  "json",
		"success": true,
	}).Debug("Successfully parsed JSON response")

	return parsedResp, nil
}

// 移除重复的方法和类型定义，已移动到单独的文件

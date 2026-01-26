package prompt

import (
	"context"
	"errors"

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
	RawText  string            `json:"raw_text"`
	Sections map[string]string `json:"sections"` // translations keyed by language code
	// Optional fields for multi-stage pipelines.
	Transcription string                 `json:"transcription,omitempty"`
	SourceText    string                 `json:"source_text,omitempty"`
	CorrectedText string                 `json:"corrected_text,omitempty"`
	Metadata      map[string]interface{} `json:"metadata"`
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

// BuildTextCorrectPrompt builds a correction-only prompt for ASR text.
func (e *Engine) BuildTextCorrectPrompt(ctx context.Context, sourceText string, dictionary []config.DictionaryTerm) (*Prompt, error) {
	data := map[string]interface{}{
		"SourceText":  sourceText,
		"Dictionary":  dictionary,
		"Variables":   map[string]interface{}{},
		"TargetCodes": []string{},
	}

	p, _, err := e.templateManager.BuildPrompt(ctx, "text_correct", data)
	if err != nil {
		return nil, coreerrors.NewInternalError("build text correct prompt failed", err)
	}
	return p, nil
}

// BuildTextCorrectTranslatePrompt builds a merged correction+translation prompt for ASR text.
func (e *Engine) BuildTextCorrectTranslatePrompt(ctx context.Context, sourceText string, targetLangCodes []string, dictionary []config.DictionaryTerm) (*Prompt, error) {
	targetLanguageNames, err := e.languageManager.ConvertCodesToDisplayNames(targetLangCodes)
	if err != nil {
		var appErr *coreerrors.AppError
		if errors.As(err, &appErr) {
			return nil, appErr
		}
		return nil, coreerrors.NewValidationError("convert target language codes failed", err)
	}

	targetLanguageStyleNotes := e.languageManager.BuildStyleNotes(targetLangCodes)

	data := map[string]interface{}{
		"TargetLanguageCodes":      targetLangCodes,
		"TargetLanguageNames":      targetLanguageNames,
		"TargetLanguageStyleNotes": targetLanguageStyleNotes,
		"SourceText":               sourceText,
		"Dictionary":               dictionary,
	}

	p, _, err := e.templateManager.BuildPrompt(ctx, "text_correct_translate", data)
	if err != nil {
		return nil, coreerrors.NewInternalError("build text correct+translate prompt failed", err)
	}

	p.OutputRules = e.languageManager.BuildDynamicOutputRules(TaskTranslate, targetLangCodes, false)
	return p, nil
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

	targetLanguageStyleNotes := e.languageManager.BuildStyleNotes(req.TargetLanguages)

	// 准备模板数据
	data := map[string]interface{}{
		"Task":                     req.Task,
		"SourceLanguage":           req.SourceLanguage,
		"TargetLanguageCodes":      req.TargetLanguages,      // 保留原始短代码
		"TargetLanguageNames":      targetLanguageNames,      // 用于模板的中文显示名称
		"TargetLanguageStyleNotes": targetLanguageStyleNotes, // 用于模板的风格说明
		"Variables":                req.Variables,
		"SourceText":               req.Variables["source_text"], // 源文本
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

package prompt

import (
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/sirupsen/logrus"
)

// PromptTemplate 提示词模板
type PromptTemplate struct {
	Name         string                 `yaml:"name"`
	Version      string                 `yaml:"version"`
	Description  string                 `yaml:"description"`
	SystemPrompt string                 `yaml:"system_prompt"`
	UserPrompt   string                 `yaml:"user_prompt"`
	OutputRules  OutputRules            `yaml:"output_rules"`
	Variables    map[string]interface{} `yaml:"variables"`
}

// TemplateManager 模板管理器
type TemplateManager struct {
	templates map[string]*PromptTemplate
	logger    *logrus.Logger
}

// NewTemplateManager 创建模板管理器
func NewTemplateManager(logger *logrus.Logger) *TemplateManager {
	manager := &TemplateManager{
		templates: make(map[string]*PromptTemplate),
		logger:    logger,
	}

	// 加载默认模板
	if err := manager.loadDefaultTemplates(); err != nil {
		logger.WithError(err).Error("Failed to load default templates")
	}

	return manager
}

// loadDefaultTemplates 加载默认模板
func (tm *TemplateManager) loadDefaultTemplates() error {
	// 文本纠错模板
	correctionTemplate := &PromptTemplate{
		Name:        "text_correct",
		Version:     "1.0",
		Description: "语音转写文本纠错",
		SystemPrompt: `你是一个语音转写文本纠正助手。

你的任务：
- 修正语音识别文本中的识别错误、同音字错误、错别字和标点问题
- 保持原意，不增删信息
- 当识别结果中出现与用户词典中词汇发音相似、拼写接近或语义相关的词时，将其替换为词典中的标准形式

{{- if .Dictionary }}

【用户词典】
{{- range .Dictionary }}
- {{ .Term }}（可能被误识别为：{{ join .Aliases ", " }}）
{{- end }}
{{- end }}

请以 JSON 格式输出：
` + "```json" + `
{
  "corrected_text": "<纠正后的文本>"
}
` + "```",
		UserPrompt: `
待纠正的语音识别文本：
{{ .SourceText }}`,
	}

	// 文本纠错 + 翻译（合并）模板
	correctAndTranslateTemplate := &PromptTemplate{
		Name:        "text_correct_translate",
		Version:     "1.0",
		Description: "语音转写文本纠错并翻译（合并调用）",
		SystemPrompt: `你是一个专业的语音转写纠错和翻译助手。

你的任务：
1. 修正语音识别文本中的识别错误（同音字、错别字、标点问题）
2. 参考用户词典替换专有名词
3. 将纠正后的文本翻译成目标语言

{{- if .Dictionary }}

【用户词典】
{{- range .Dictionary }}
- {{ .Term }}（可能被误识别为：{{ join .Aliases ", " }}）
{{- end }}
{{- end }}

【目标语言】
{{- range $index, $langName := .TargetLanguageNames }}
{{ add $index 1 }}. {{ $langName }}
{{- end }}

{{- if .TargetLanguageStyleNotes }}

【翻译风格说明】
{{- range .TargetLanguageStyleNotes }}
- {{ .DisplayName }}：{{ .Note }}
{{- end }}
{{- end }}

请以 JSON 格式输出：
` + "```json" + `
{
  "corrected_text": "<纠正后的源文本>",
  "translations": {
{{- range $index, $langCode := .TargetLanguageCodes }}
    "{{ $langCode }}": "<{{ index $.TargetLanguageNames $index }} 译文>"{{ if ne $index (sub (len $.TargetLanguageCodes) 1) }},{{ end }}
{{- end }}
  }
}
` + "```",
		UserPrompt: `
待处理的语音识别文本：
{{ .SourceText }}`,
	}

	// 文本翻译模板
	textTemplate := &PromptTemplate{
		Name:        "text_translate",
		Version:     "1.0",
		Description: "文本翻译模板",
		SystemPrompt: `你是一个专业的翻译助手。你的任务是将给定的文本翻译成指定的目标语言。

{{- range $index, $langName := .TargetLanguageNames }}
{{ add $index 1 }}. 将文本翻译成{{ $langName }}。
{{- end }}

{{- if .TargetLanguageStyleNotes }}

【翻译风格说明】
{{- range .TargetLanguageStyleNotes }}
- {{ .DisplayName }}：{{ .Note }}
{{- end }}
{{- end }}

请最终 **务必** 在回答中包含如下 JSON，对其使用 markdown ` + "```json```" + ` 包裹：
` + "```json" + `
{
  "translations": {
{{- range $index, $langCode := .TargetLanguageCodes }}
    "{{ $langCode }}": "<{{ index $.TargetLanguageNames $index }} 译文>"{{ if ne $index (sub (len $.TargetLanguageCodes) 1) }},{{ end }}
{{- end }}
  }
}
` + "```" + `
除 JSON 与必要换行外，可以补充解释；但 JSON 代码块必须完整、合法。请确保翻译准确、自然，并保持原文的语气和风格。`,
		UserPrompt: `请翻译以下文本：

{{ .SourceText }}`,
	}

	tm.templates["text_correct"] = correctionTemplate
	tm.templates["text_correct_translate"] = correctAndTranslateTemplate
	tm.templates["text_translate"] = textTemplate

	return nil
}

// GetTemplate 获取模板
func (tm *TemplateManager) GetTemplate(name string) (*PromptTemplate, bool) {
	tmpl, ok := tm.templates[name]
	return tmpl, ok
}

// BuildPrompt 构建提示词
func (tm *TemplateManager) BuildPrompt(ctx context.Context, templateName string, data map[string]interface{}) (*Prompt, OutputRules, error) {
	tmpl, ok := tm.templates[templateName]
	if !ok {
		return nil, OutputRules{}, fmt.Errorf("template not found: %s", templateName)
	}

	// 添加模板函数
	funcMap := template.FuncMap{
		"add":  func(a, b int) int { return a + b },
		"sub":  func(a, b int) int { return a - b },
		"len":  func(s []string) int { return len(s) },
		"join": func(list []string, sep string) string { return strings.Join(list, sep) },
		"default": func(defaultValue, value interface{}) interface{} {
			if value == nil || value == "" {
				return defaultValue
			}
			return value
		},
		"contains": func(list []string, s string) bool {
			for _, v := range list {
				if v == s {
					return true
				}
			}
			return false
		},
	}

	// 渲染系统提示词
	systemTmpl, err := template.New("system").Funcs(funcMap).Parse(tmpl.SystemPrompt)
	if err != nil {
		return nil, OutputRules{}, fmt.Errorf("parse system template: %w", err)
	}

	var systemBuf strings.Builder
	if err := systemTmpl.Execute(&systemBuf, data); err != nil {
		return nil, OutputRules{}, fmt.Errorf("execute system template: %w", err)
	}

	// 渲染用户提示词
	userTmpl, err := template.New("user").Funcs(funcMap).Parse(tmpl.UserPrompt)
	if err != nil {
		return nil, OutputRules{}, fmt.Errorf("parse user template: %w", err)
	}

	var userBuf strings.Builder
	if err := userTmpl.Execute(&userBuf, data); err != nil {
		return nil, OutputRules{}, fmt.Errorf("execute user template: %w", err)
	}

	prompt := &Prompt{
		System: systemBuf.String(),
		User:   userBuf.String(),
	}

	return prompt, tmpl.OutputRules, nil
}

// ListTemplates 列出所有模板
func (tm *TemplateManager) ListTemplates() []string {
	names := make([]string, 0, len(tm.templates))
	for name := range tm.templates {
		names = append(names, name)
	}
	return names
}

// AddTemplate 添加模板
func (tm *TemplateManager) AddTemplate(template *PromptTemplate) {
	tm.templates[template.Name] = template
}

// RemoveTemplate 移除模板
func (tm *TemplateManager) RemoveTemplate(name string) bool {
	if _, ok := tm.templates[name]; ok {
		delete(tm.templates, name)
		return true
	}
	return false
}

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
	// 音频转录模板
	audioTranscribeTemplate := &PromptTemplate{
		Name:        "audio_transcribe",
		Version:     "1.0",
		Description: "音频转录模板",
		SystemPrompt: `你是一个高级的语音处理助手。你的任务是：
1. 将音频内容转录成其原始语言的文本。

请最终 **务必** 在回答中包含如下 JSON，对其使用 markdown ` + "```json```" + ` 包裹：
` + "```json" + `
{
  "transcription": "<转录文本>",
  "translations": {}
}
` + "```" + `
除 JSON 与必要换行外，可以补充解释；但 JSON 代码块必须完整、合法。`,
		UserPrompt: `请转录下面的音频内容。`,
	}

	// 音频翻译模板
	audioTranslateTemplate := &PromptTemplate{
		Name:        "audio_translate",
		Version:     "1.0",
		Description: "音频转录和翻译模板",
		SystemPrompt: `你是一个高级的语音处理助手。你的任务是：
1. 首先将音频内容转录成其原始语言的文本。
{{- range $index, $langName := .TargetLanguageNames }}
{{ add $index 2 }}. 将文本翻译成{{ $langName }}。
{{- end }}

{{ if contains .TargetLanguageCodes "neko" }}
重要补充（当目标包含"猫娘语"时适用）：
- "猫娘语"的要求：
  1) 语气可爱、偏撒娇，适当带表情符号和喵
  2) 保持原文语义
- 输出的 JSON 中，"translations" 的 "neko" 键对应"猫娘语"版本
{{ end }}

请最终 **务必** 在回答中包含如下 JSON，对其使用 markdown ` + "```json```" + ` 包裹：
` + "```json" + `
{
  "transcription": "<转录文本>",
  "translations": {
{{- range $index, $langCode := .TargetLanguageCodes }}
    "{{ $langCode }}": "<{{ index $.TargetLanguageNames $index }} 译文>"{{ if ne $index (sub (len $.TargetLanguageCodes) 1) }},{{ end }}
{{- end }}
  }
}
` + "```" + `
除 JSON 与必要换行外，可以补充解释；但 JSON 代码块必须完整、合法。`,
		UserPrompt: `请处理下面的音频内容。`,
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

{{ if contains .TargetLanguageCodes "neko" }}
重要补充（当目标包含"猫娘语"时适用）：
- "猫娘语"的要求：
  1) 语气可爱、偏撒娇，适当带表情符号和喵
  2) 保持原文语义
- 输出的 JSON 中，"translations" 的 "neko" 键对应"猫娘语"版本
{{ end }}

请最终 **务必** 在回答中包含如下 JSON，对其使用 markdown ` + "```json```" + ` 包裹：
` + "```json" + `
{
  "source_text": "<原文>",
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

	tm.templates["audio_transcribe"] = audioTranscribeTemplate
	tm.templates["audio_translate"] = audioTranslateTemplate
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
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"len": func(s []string) int { return len(s) },
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

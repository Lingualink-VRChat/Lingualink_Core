package prompt

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// TaskType 任务类型
type TaskType string

const (
	TaskTranscribe             TaskType = "transcribe"
	TaskTranslate              TaskType = "translate"
	TaskTranscribeAndTranslate TaskType = "both"
)

// OutputFormat 输出格式
type OutputFormat string

const (
	FormatStructured OutputFormat = "structured"
	FormatJSON       OutputFormat = "json"
	FormatMarkdown   OutputFormat = "markdown"
	FormatPlain      OutputFormat = "plain"
)

// PromptRequest 提示词请求
type PromptRequest struct {
	Task            TaskType               `json:"task"`
	SourceLanguage  string                 `json:"source_language"`
	TargetLanguages []string               `json:"target_languages"`
	Template        string                 `json:"template"`
	Variables       map[string]interface{} `json:"variables"`
	OutputFormat    OutputFormat           `json:"output_format"`
	UserPrompt      string                 `json:"user_prompt"`
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
	Key          string   `json:"key"`
	Aliases      []string `json:"aliases"`
	LanguageCode string   `json:"language_code"`
	Required     bool     `json:"required"`
	Order        int      `json:"order"`
}

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

// Language 语言定义
type Language struct {
	Code    string            `yaml:"code"`
	Names   map[string]string `yaml:"names"`
	Aliases []string          `yaml:"aliases"`
}

// Engine 提示词引擎
type Engine struct {
	templates map[string]*PromptTemplate
	languages map[string]*Language
	config    config.PromptConfig
	logger    *logrus.Logger
}

// NewEngine 创建提示词引擎
func NewEngine(cfg config.PromptConfig, logger *logrus.Logger) (*Engine, error) {
	engine := &Engine{
		templates: make(map[string]*PromptTemplate),
		languages: make(map[string]*Language),
		config:    cfg,
		logger:    logger,
	}

	// 加载语言配置
	for _, lang := range cfg.Languages {
		engine.languages[lang.Code] = &Language{
			Code:    lang.Code,
			Names:   lang.Names,
			Aliases: lang.Aliases,
		}
	}

	// 加载默认语言
	if len(engine.languages) == 0 {
		engine.loadDefaultLanguages()
	}

	// 加载默认模板
	if err := engine.loadDefaultTemplate(); err != nil {
		return nil, fmt.Errorf("load default template: %w", err)
	}

	return engine, nil
}

// loadDefaultLanguages 加载默认语言
func (e *Engine) loadDefaultLanguages() {
	defaultLanguages := map[string]*Language{
		"zh": {
			Code: "zh",
			Names: map[string]string{
				"display": "中文",
				"english": "Chinese",
				"native":  "中文",
			},
			Aliases: []string{"chinese", "中文", "汉语", "zh-CN", "zh-TW"},
		},
		"en": {
			Code: "en",
			Names: map[string]string{
				"display": "英文",
				"english": "English",
				"native":  "English",
			},
			Aliases: []string{"english", "英文", "英语"},
		},
		"ja": {
			Code: "ja",
			Names: map[string]string{
				"display": "日文",
				"english": "Japanese",
				"native":  "日本語",
			},
			Aliases: []string{"japanese", "日文", "日语", "日本語"},
		},
	}

	for code, lang := range defaultLanguages {
		e.languages[code] = lang
	}
}

// loadDefaultTemplate 加载默认模板
func (e *Engine) loadDefaultTemplate() error {
	defaultTemplate := &PromptTemplate{
		Name:        "default",
		Version:     "1.0",
		Description: "默认音频处理模板",
		SystemPrompt: `你是一个高级的语音处理助手。你的任务是：
{{- if or (eq .Task "transcribe") (eq .Task "both") }}
1. 首先将音频内容转录成其原始语言的文本。
{{- end }}
{{- if or (eq .Task "translate") (eq .Task "both") }}
{{- range $index, $lang := .TargetLanguages }}
{{ add $index 2 }}. 将文本翻译成{{ $lang }}。
{{- end }}
{{- end }}

请按照以下格式清晰地组织你的输出：
{{- if or (eq .Task "transcribe") (eq .Task "both") }}
原文：
{{- end }}
{{- range .TargetLanguages }}
{{ . }}：
{{- end }}`,
		UserPrompt: `{{ .UserPrompt | default "请处理下面的音频。" }}`,
		OutputRules: OutputRules{
			Format:    FormatStructured,
			Separator: "：",
			Sections: []OutputSection{
				{
					Key:      "原文",
					Aliases:  []string{"Original", "原始文本", "Transcription"},
					Required: true,
					Order:    1,
				},
				{
					Key:          "英文",
					Aliases:      []string{"English", "英语"},
					LanguageCode: "en",
					Order:        2,
				},
				{
					Key:          "日文",
					Aliases:      []string{"Japanese", "日语", "日本語"},
					LanguageCode: "ja",
					Order:        3,
				},
				{
					Key:          "中文",
					Aliases:      []string{"Chinese", "中文", "汉语"},
					LanguageCode: "zh",
					Order:        4,
				},
			},
		},
	}

	e.templates["default"] = defaultTemplate
	return nil
}

// Build 构建提示词
func (e *Engine) Build(ctx context.Context, req PromptRequest) (*Prompt, error) {
	// 获取模板
	templateName := req.Template
	if templateName == "" {
		templateName = e.config.Defaults.Template
	}

	tmpl, ok := e.templates[templateName]
	if !ok {
		return nil, fmt.Errorf("template not found: %s", templateName)
	}

	// 标准化目标语言
	normalizedTargetLangs, err := e.normalizeLanguages(req.TargetLanguages)
	if err != nil {
		return nil, fmt.Errorf("normalize target languages: %w", err)
	}

	// 准备模板数据
	data := map[string]interface{}{
		"Task":            req.Task,
		"SourceLanguage":  req.SourceLanguage,
		"TargetLanguages": normalizedTargetLangs,
		"UserPrompt":      req.UserPrompt,
		"Variables":       req.Variables,
	}

	// 添加模板函数
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"default": func(defaultValue, value interface{}) interface{} {
			if value == nil || value == "" {
				return defaultValue
			}
			return value
		},
	}

	// 渲染系统提示词
	systemTmpl, err := template.New("system").Funcs(funcMap).Parse(tmpl.SystemPrompt)
	if err != nil {
		return nil, fmt.Errorf("parse system template: %w", err)
	}

	var systemBuf strings.Builder
	if err := systemTmpl.Execute(&systemBuf, data); err != nil {
		return nil, fmt.Errorf("execute system template: %w", err)
	}

	// 渲染用户提示词
	userTmpl, err := template.New("user").Funcs(funcMap).Parse(tmpl.UserPrompt)
	if err != nil {
		return nil, fmt.Errorf("parse user template: %w", err)
	}

	var userBuf strings.Builder
	if err := userTmpl.Execute(&userBuf, data); err != nil {
		return nil, fmt.Errorf("execute user template: %w", err)
	}

	return &Prompt{
		System:      systemBuf.String(),
		User:        userBuf.String(),
		OutputRules: tmpl.OutputRules,
	}, nil
}

// normalizeLanguages 标准化语言列表
func (e *Engine) normalizeLanguages(languages []string) ([]string, error) {
	var normalized []string
	for _, lang := range languages {
		code, err := e.normalizeLanguage(lang)
		if err != nil {
			return nil, fmt.Errorf("normalize language %s: %w", lang, err)
		}

		// 获取显示名称
		if langDef, ok := e.languages[code]; ok {
			if displayName, ok := langDef.Names["display"]; ok {
				normalized = append(normalized, displayName)
			} else {
				normalized = append(normalized, code)
			}
		} else {
			normalized = append(normalized, code)
		}
	}
	return normalized, nil
}

// normalizeLanguage 标准化单个语言
func (e *Engine) normalizeLanguage(input string) (string, error) {
	input = strings.TrimSpace(strings.ToLower(input))

	// 直接匹配语言代码
	if lang, ok := e.languages[input]; ok {
		return lang.Code, nil
	}

	// 别名匹配
	for code, lang := range e.languages {
		for _, alias := range lang.Aliases {
			if strings.EqualFold(input, alias) {
				return code, nil
			}
		}
	}

	return "", fmt.Errorf("unknown language: %s", input)
}

// ParsedResponse 解析后的响应
type ParsedResponse struct {
	RawText  string                 `json:"raw_text"`
	Sections map[string]string      `json:"sections"`
	Metadata map[string]interface{} `json:"metadata"`
}

// ParseResponse 解析LLM响应
func (e *Engine) ParseResponse(content string, rules OutputRules) (*ParsedResponse, error) {
	parser := &StructuredParser{
		separators: []string{":", "：", "->", "=>"},
		logger:     e.logger,
	}

	return parser.Parse(content, rules)
}

// StructuredParser 结构化文本解析器
type StructuredParser struct {
	separators []string
	logger     *logrus.Logger
}

// Parse 解析结构化文本
func (p *StructuredParser) Parse(content string, rules OutputRules) (*ParsedResponse, error) {
	result := &ParsedResponse{
		RawText:  content,
		Sections: make(map[string]string),
		Metadata: make(map[string]interface{}),
	}

	// 预处理：分行
	lines := strings.Split(strings.TrimSpace(content), "\n")

	// 构建段落匹配器
	sectionMatchers := p.buildSectionMatchers(rules.Sections)

	var currentSection string
	var currentContent []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// 尝试匹配新段落
		if section, value, matched := p.matchSection(trimmed, sectionMatchers); matched {
			// 保存前一个段落
			if currentSection != "" {
				result.Sections[currentSection] = strings.TrimSpace(
					strings.Join(currentContent, "\n"),
				)
			}

			// 开始新段落
			currentSection = section
			currentContent = []string{}
			if value != "" {
				currentContent = append(currentContent, value)
			}
		} else if currentSection != "" {
			// 继续当前段落
			currentContent = append(currentContent, trimmed)
		}
	}

	// 保存最后一个段落
	if currentSection != "" {
		result.Sections[currentSection] = strings.TrimSpace(
			strings.Join(currentContent, "\n"),
		)
	}

	// 添加元数据
	result.Metadata["parse_time"] = time.Now().Unix()
	result.Metadata["parser_version"] = "1.0"

	return result, nil
}

// buildSectionMatchers 构建段落匹配器
func (p *StructuredParser) buildSectionMatchers(sections []OutputSection) map[string][]string {
	matchers := make(map[string][]string)
	for _, section := range sections {
		patterns := []string{section.Key}
		patterns = append(patterns, section.Aliases...)
		matchers[section.Key] = patterns
	}
	return matchers
}

// matchSection 匹配段落
func (p *StructuredParser) matchSection(line string, matchers map[string][]string) (string, string, bool) {
	for _, sep := range p.separators {
		if idx := strings.Index(line, sep); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+len(sep):])

			// 尝试匹配已知段落
			for section, patterns := range matchers {
				for _, pattern := range patterns {
					if p.fuzzyMatch(key, pattern) {
						return section, value, true
					}
				}
			}

			// 未知段落也保留
			return key, value, true
		}
	}
	return "", "", false
}

// fuzzyMatch 模糊匹配
func (p *StructuredParser) fuzzyMatch(input, pattern string) bool {
	// 1. 精确匹配
	if strings.EqualFold(input, pattern) {
		return true
	}

	// 2. 去除空格和标点
	cleanInput := regexp.MustCompile(`[\s\p{P}]+`).ReplaceAllString(input, "")
	cleanPattern := regexp.MustCompile(`[\s\p{P}]+`).ReplaceAllString(pattern, "")
	if strings.EqualFold(cleanInput, cleanPattern) {
		return true
	}

	// 3. 包含匹配
	if strings.Contains(strings.ToLower(input), strings.ToLower(pattern)) ||
		strings.Contains(strings.ToLower(pattern), strings.ToLower(input)) {
		return true
	}

	return false
}

// RegisterTemplate 注册模板
func (e *Engine) RegisterTemplate(name string, templateData []byte) error {
	var tmpl PromptTemplate
	if err := yaml.Unmarshal(templateData, &tmpl); err != nil {
		return fmt.Errorf("unmarshal template: %w", err)
	}

	e.templates[name] = &tmpl
	e.logger.Infof("Registered template: %s", name)
	return nil
}

package prompt

import (
	"fmt"
	"strings"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	coreerrors "github.com/Lingualink-VRChat/Lingualink_Core/internal/core/errors"
	"github.com/sirupsen/logrus"
)

// Language 语言定义
type Language struct {
	Code    string            `yaml:"code"`
	Names   map[string]string `yaml:"names"`
	Aliases []string          `yaml:"aliases"`
}

// LanguageManager 语言管理器
type LanguageManager struct {
	languages  map[string]*Language // 按短代码索引
	codeIndex  map[string]*Language // code(lowercase) -> language
	aliasIndex map[string]*Language // alias/display/native/english(lowercase) -> language
	logger     *logrus.Logger
}

// NewLanguageManager 创建语言管理器
func NewLanguageManager(cfg config.PromptConfig, logger *logrus.Logger) *LanguageManager {
	manager := &LanguageManager{
		languages:  make(map[string]*Language),
		codeIndex:  make(map[string]*Language),
		aliasIndex: make(map[string]*Language),
		logger:     logger,
	}

	// 加载语言配置
	for _, lang := range cfg.Languages {
		langDef := &Language{
			Code:    lang.Code,
			Names:   lang.Names,
			Aliases: lang.Aliases,
		}
		manager.languages[lang.Code] = langDef

		if langDef.Code != "" {
			manager.codeIndex[strings.ToLower(langDef.Code)] = langDef
		}

		// 建立别名索引（display/native/english/aliases）
		for _, name := range langDef.Names {
			normalized := strings.ToLower(strings.TrimSpace(name))
			if normalized != "" {
				manager.aliasIndex[normalized] = langDef
			}
		}
		for _, alias := range langDef.Aliases {
			normalized := strings.ToLower(strings.TrimSpace(alias))
			if normalized != "" {
				manager.aliasIndex[normalized] = langDef
			}
		}
	}

	return manager
}

// GetLanguages 获取所有支持的语言
func (lm *LanguageManager) GetLanguages() map[string]*Language {
	return lm.languages
}

// GetLanguage 根据代码获取语言
func (lm *LanguageManager) GetLanguage(code string) (*Language, bool) {
	lang, ok := lm.languages[code]
	return lang, ok
}

// ConvertCodesToDisplayNames 将短代码转换为中文显示名称
func (lm *LanguageManager) ConvertCodesToDisplayNames(codes []string) ([]string, error) {
	var displayNames []string
	for _, code := range codes {
		normalizedCode, err := lm.NormalizeLanguage(code)
		if err != nil {
			return nil, coreerrors.NewValidationError(fmt.Sprintf("normalize language code %s: %s", code, err.Error()), err)
		}

		if lang, ok := lm.languages[normalizedCode]; ok {
			if displayName, ok := lang.Names["display"]; ok {
				displayNames = append(displayNames, displayName)
			} else {
				lm.logger.Warnf("Display name not found for language code: %s, using code itself", normalizedCode)
				displayNames = append(displayNames, normalizedCode)
			}
		} else {
			return nil, coreerrors.NewValidationError(fmt.Sprintf("language definition not found for code: %s", normalizedCode), nil)
		}
	}
	return displayNames, nil
}

// NormalizeLanguage 标准化单个语言（输入可以是代码或别名）
func (lm *LanguageManager) NormalizeLanguage(input string) (string, error) {
	input = strings.TrimSpace(strings.ToLower(input))

	// 检查空字符串
	if input == "" {
		return "", coreerrors.NewValidationError("unknown language", nil)
	}

	// 直接匹配语言代码
	if lang, ok := lm.codeIndex[input]; ok {
		return lang.Code, nil
	}

	// 别名匹配
	if lang, ok := lm.aliasIndex[input]; ok {
		return lang.Code, nil
	}

	return "", coreerrors.NewValidationError(fmt.Sprintf("unknown language: %s", input), nil)
}

// IdentifyLanguageFromText 通过文本识别语言代码
func (lm *LanguageManager) IdentifyLanguageFromText(text string) (string, error) {
	// 标准化输入
	normalizedText := strings.TrimSpace(text)

	// 直接尝试语言代码匹配
	if code, err := lm.NormalizeLanguage(normalizedText); err == nil {
		return code, nil
	}

	// 尝试更宽松的匹配
	cleanedText := strings.ToLower(strings.TrimSpace(normalizedText))

	for langCode, lang := range lm.languages {
		// 检查display名称
		for _, name := range lang.Names {
			if strings.EqualFold(cleanedText, name) {
				return langCode, nil
			}
		}

		// 检查别名
		for _, alias := range lang.Aliases {
			if strings.EqualFold(cleanedText, alias) {
				return langCode, nil
			}
		}

		// 检查语言代码
		if strings.EqualFold(cleanedText, langCode) {
			return langCode, nil
		}

		// 检查部分匹配（用于处理如"英语翻译"这样的变体）
		for _, alias := range lang.Aliases {
			if strings.Contains(cleanedText, strings.ToLower(alias)) && len(alias) > 2 {
				return langCode, nil
			}
		}

		for _, name := range lang.Names {
			if strings.Contains(cleanedText, strings.ToLower(name)) && len(name) > 2 {
				return langCode, nil
			}
		}
	}

	return "", coreerrors.NewValidationError(fmt.Sprintf("no language identified for text: %s", text), nil)
}

// BuildDynamicOutputRules 根据任务类型和目标语言动态构建OutputRules
func (lm *LanguageManager) BuildDynamicOutputRules(task TaskType, targetLanguageCodes []string, includeSource bool) OutputRules {
	sections := []OutputSection{}

	// 如果需要包含源文本段落（音频处理总是需要，文本处理可选）
	if includeSource {
		sections = append(sections, OutputSection{
			Key:      "原文",
			Aliases:  []string{"Original", "原始文本", "Transcription", "转录", "原始", "源文本"},
			Required: true,
			Order:    1,
		})
	}

	// 为每个目标语言动态创建OutputSection
	startOrder := 1
	if includeSource {
		startOrder = 2
	}

	for i, langCode := range targetLanguageCodes {
		normalizedCode := strings.ToLower(strings.TrimSpace(langCode))
		if lang, ok := lm.codeIndex[normalizedCode]; ok {
			// 构建别名列表：包括display名称和所有配置的别名
			aliases := make([]string, 0)

			// 添加所有names中的值作为别名
			for _, name := range lang.Names {
				if name != "" {
					aliases = append(aliases, name)
				}
			}

			// 添加配置文件中的所有别名
			aliases = append(aliases, lang.Aliases...)

			// 添加语言代码本身作为别名
			aliases = append(aliases, lang.Code)
			aliases = append(aliases, strings.ToUpper(lang.Code))

			// 获取主要显示名称作为Key
			key := lang.Names["display"]
			if key == "" {
				key = lang.Code // 回退到代码
			}

			section := OutputSection{
				Key:          key,
				Aliases:      aliases,
				LanguageCode: lang.Code,
				Required:     false,
				Order:        i + startOrder,
			}

			sections = append(sections, section)
		} else {
			lm.logger.Warnf("Language definition not found for code: %s", langCode)
		}
	}

	return OutputRules{
		Format:    FormatStructured,
		Separator: ":",
		Sections:  sections,
	}
}

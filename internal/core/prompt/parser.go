package prompt

import (
	"regexp"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// ParsedResponse 解析后的响应
type ParsedResponse struct {
	RawText  string                 `json:"raw_text"`
	Sections map[string]string      `json:"sections"` // 键为短代码或特殊键（如"原文"）
	Metadata map[string]interface{} `json:"metadata"`
}

// StructuredParser 结构化文本解析器
type StructuredParser struct {
	separators []string
	logger     *logrus.Logger
}

// NewStructuredParser 创建结构化解析器
func NewStructuredParser(separators []string, logger *logrus.Logger) *StructuredParser {
	if len(separators) == 0 {
		separators = []string{":", "：", "->", "=>"}
	}
	return &StructuredParser{
		separators: separators,
		logger:     logger,
	}
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

			// 尝试匹配已知段落 - 按照精确度排序匹配
			bestMatch := ""
			maxMatchScore := 0

			for section, patterns := range matchers {
				for _, pattern := range patterns {
					score := p.getMatchScore(key, pattern)
					if score > maxMatchScore {
						maxMatchScore = score
						bestMatch = section
					}
				}
			}

			if bestMatch != "" {
				return bestMatch, value, true
			}

			// 未知段落也保留
			return key, value, true
		}
	}
	return "", "", false
}

// getMatchScore 获取匹配分数，分数越高表示匹配越精确
func (p *StructuredParser) getMatchScore(input, pattern string) int {
	// 1. 精确匹配 - 最高分
	if strings.EqualFold(input, pattern) {
		return 100
	}

	// 2. 去除空格和标点后精确匹配
	cleanInput := regexp.MustCompile(`[\s\p{P}]+`).ReplaceAllString(input, "")
	cleanPattern := regexp.MustCompile(`[\s\p{P}]+`).ReplaceAllString(pattern, "")
	if strings.EqualFold(cleanInput, cleanPattern) {
		return 90
	}

	// 3. 包含匹配 - 根据长度给分，越长越精确
	inputLower := strings.ToLower(input)
	patternLower := strings.ToLower(pattern)

	if inputLower == patternLower {
		return 80
	}

	if strings.Contains(inputLower, patternLower) {
		// 输入包含模式，分数基于模式长度占输入长度的比例
		return 50 + int(float64(len(patternLower))/float64(len(inputLower))*30)
	}

	if strings.Contains(patternLower, inputLower) {
		// 模式包含输入，分数基于输入长度占模式长度的比例
		return 40 + int(float64(len(inputLower))/float64(len(patternLower))*30)
	}

	return 0 // 不匹配
}

// fuzzyMatch 模糊匹配 - 保留用于向后兼容，但现在使用getMatchScore
func (p *StructuredParser) fuzzyMatch(input, pattern string) bool {
	return p.getMatchScore(input, pattern) > 0
}

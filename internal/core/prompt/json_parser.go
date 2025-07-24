package prompt

import (
	"encoding/json"
	"regexp"
)

// extractJSONBlock 从 Markdown ```json``` 代码块中提取 JSON 内容
var reJSON = regexp.MustCompile(`(?s)` + "```json\\s*({.*?})\\s*```")

func extractJSONBlock(raw string) ([]byte, bool) {
	matches := reJSON.FindStringSubmatch(raw)
	if len(matches) == 2 {
		return []byte(matches[1]), true
	}
	return nil, false
}

// parseJSONResponse 解析 JSON 响应数据
func parseJSONResponse(jsonData []byte) (*ParsedResponse, error) {
	var parsed struct {
		Transcription string            `json:"transcription"`
		SourceText    string            `json:"source_text"`
		Translations  map[string]string `json:"translations"`
	}

	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		return nil, err
	}

	sections := make(map[string]string)

	// 处理转录或原文
	if parsed.Transcription != "" {
		sections["原文"] = parsed.Transcription
	}
	if parsed.SourceText != "" {
		sections["原文"] = parsed.SourceText // 文本翻译场景
	}

	// 处理翻译结果
	for langCode, translation := range parsed.Translations {
		if translation != "" {
			sections[langCode] = translation
		}
	}

	return &ParsedResponse{
		RawText:  string(jsonData),
		Sections: sections,
		Metadata: map[string]interface{}{
			"parser":        "json",
			"parse_success": true,
		},
	}, nil
}

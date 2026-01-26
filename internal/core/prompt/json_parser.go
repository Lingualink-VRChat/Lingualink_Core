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
		CorrectedText string            `json:"corrected_text"`
		Translations  map[string]string `json:"translations"`
	}

	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		return nil, err
	}

	sections := make(map[string]string)
	for langCode, translation := range parsed.Translations {
		if translation != "" {
			sections[langCode] = translation
		}
	}

	return &ParsedResponse{
		RawText:       string(jsonData),
		Sections:      sections,
		Transcription: parsed.Transcription,
		SourceText:    parsed.SourceText,
		CorrectedText: parsed.CorrectedText,
		Metadata: map[string]interface{}{
			"parser":        "json",
			"parse_success": true,
		},
	}, nil
}

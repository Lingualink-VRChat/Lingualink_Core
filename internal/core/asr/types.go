package asr

import (
	"regexp"
	"strings"
)

// ASRRequest describes a transcription request.
type ASRRequest struct {
	Audio       []byte
	AudioFormat string
	Language    string // optional hint
	Prompt      string // optional hint
}

// Segment represents an optional segment returned by verbose ASR responses.
type Segment struct {
	ID    int     `json:"id"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// ASRResponse describes a transcription result.
type ASRResponse struct {
	Text             string    `json:"text"` // Parsed clean text after extracting backend wrappers.
	RawText          string    `json:"raw_text,omitempty"`
	DetectedLanguage string    `json:"language"`
	Duration         float64   `json:"duration"`
	Segments         []Segment `json:"segments"`
}

// Matches ASR backend output like: "language Chinese<asr_text>你好世界".
var asrTextPattern = regexp.MustCompile(`^language\s+(\w+)\s*<asr_text>(.*)$`)

// ParseASRText extracts actual ASR text from wrapped backend output.
func ParseASRText(rawText string) (language string, text string) {
	rawText = strings.TrimSpace(rawText)
	if rawText == "" {
		return "", ""
	}

	matches := asrTextPattern.FindStringSubmatch(rawText)
	if matches == nil {
		return "", rawText
	}

	lang := strings.TrimSpace(matches[1])
	extractedText := strings.TrimSpace(matches[2])
	if strings.EqualFold(lang, "None") {
		return "", ""
	}

	return lang, extractedText
}

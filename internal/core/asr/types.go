package asr

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
	Text             string    `json:"text"`
	DetectedLanguage string    `json:"language"`
	Duration         float64   `json:"duration"`
	Segments         []Segment `json:"segments"`
}

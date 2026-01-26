package tool

import (
	"context"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/asr"
	coreerrors "github.com/Lingualink-VRChat/Lingualink_Core/internal/core/errors"
)

type ASRTool struct {
	manager *asr.Manager
}

func NewASRTool(manager *asr.Manager) *ASRTool {
	return &ASRTool{manager: manager}
}

func (t *ASRTool) Name() string {
	return "asr"
}

func (t *ASRTool) Description() string {
	return "Transcribe audio to text"
}

func (t *ASRTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"audio": map[string]interface{}{
				"type":        "string",
				"description": "Audio bytes (binary payload, provided internally)",
			},
			"format": map[string]interface{}{
				"type":        "string",
				"description": "Audio format, e.g. wav/mp3/opus",
			},
			"language": map[string]interface{}{
				"type":        "string",
				"description": "Optional source language hint",
			},
		},
		"required": []string{"audio", "format"},
	}
}

func (t *ASRTool) OutputSchema() map[string]interface{} {
	// Not used for non-LLM tools.
	return nil
}

func (t *ASRTool) Validate(input Input) error {
	if input.Data == nil {
		return coreerrors.NewValidationError("input data is required", nil)
	}

	audioAny, ok := input.Data["audio"]
	if !ok || audioAny == nil {
		return coreerrors.NewValidationError("audio is required", nil)
	}
	audioBytes, ok := audioAny.([]byte)
	if !ok || len(audioBytes) == 0 {
		return coreerrors.NewValidationError("audio must be non-empty bytes", nil)
	}

	formatAny, ok := input.Data["format"]
	if !ok {
		return coreerrors.NewValidationError("format is required", nil)
	}
	format, ok := formatAny.(string)
	if !ok || format == "" {
		return coreerrors.NewValidationError("format must be a non-empty string", nil)
	}

	return nil
}

func (t *ASRTool) Execute(ctx context.Context, input Input) (Output, error) {
	if t.manager == nil {
		return Output{}, coreerrors.NewInternalError("asr manager not configured", nil)
	}
	if err := t.Validate(input); err != nil {
		return Output{}, err
	}

	audioBytes := input.Data["audio"].([]byte)
	format := input.Data["format"].(string)
	language, _ := input.Data["language"].(string)

	resp, err := t.manager.Transcribe(ctx, &asr.ASRRequest{
		Audio:       audioBytes,
		AudioFormat: format,
		Language:    language,
	})
	if err != nil {
		return Output{}, err
	}

	segments := make([]map[string]interface{}, 0, len(resp.Segments))
	for _, seg := range resp.Segments {
		segments = append(segments, map[string]interface{}{
			"id":    seg.ID,
			"start": seg.Start,
			"end":   seg.End,
			"text":  seg.Text,
		})
	}

	out := Output{
		Data: map[string]interface{}{
			"text":     resp.Text,
			"language": resp.DetectedLanguage,
			"duration": resp.Duration,
			"segments": segments,
		},
	}

	if input.Context != nil && input.Context.OriginalRequest != nil {
		if originalFormat, ok := input.Context.OriginalRequest["audio_format"].(string); ok && originalFormat != "" && originalFormat != format {
			if out.Metadata == nil {
				out.Metadata = make(map[string]interface{})
			}
			out.Metadata["audio_original_format"] = originalFormat
			out.Metadata["audio_processed_format"] = format
		}
	}

	return out, nil
}

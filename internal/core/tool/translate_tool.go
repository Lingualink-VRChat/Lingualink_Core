package tool

import (
	"context"
	"strings"

	coreerrors "github.com/Lingualink-VRChat/Lingualink_Core/internal/core/errors"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/llm"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/prompt"
)

type TranslateTool struct {
	llmManager          *llm.Manager
	promptEngine        *prompt.Engine
	toolCallingEnabled  bool
	toolCallingThinking bool
}

func NewTranslateTool(llmManager *llm.Manager, promptEngine *prompt.Engine, toolCallingEnabled, allowThinking bool) *TranslateTool {
	return &TranslateTool{
		llmManager:          llmManager,
		promptEngine:        promptEngine,
		toolCallingEnabled:  toolCallingEnabled,
		toolCallingThinking: allowThinking,
	}
}

func (t *TranslateTool) Name() string {
	return "translate"
}

func (t *TranslateTool) Description() string {
	return "Translate text into target languages"
}

func (t *TranslateTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"text": map[string]interface{}{
				"type":        "string",
				"description": "Source text to translate",
			},
			"source_language": map[string]interface{}{
				"type":        "string",
				"description": "Optional source language hint",
			},
			"target_languages": map[string]interface{}{
				"type":        "array",
				"description": "Target language codes",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
		},
		"required": []string{"text", "target_languages"},
	}
}

func (t *TranslateTool) OutputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"translations": map[string]interface{}{
				"type":                 "object",
				"description":          "Translation results keyed by language code",
				"additionalProperties": map[string]string{"type": "string"},
			},
		},
		"required": []string{"translations"},
	}
}

func (t *TranslateTool) Validate(input Input) error {
	if input.Data == nil {
		return coreerrors.NewValidationError("input data is required", nil)
	}

	textAny, ok := input.Data["text"]
	if !ok {
		return coreerrors.NewValidationError("text is required", nil)
	}
	text, ok := textAny.(string)
	if !ok || strings.TrimSpace(text) == "" {
		return coreerrors.NewValidationError("text must be a non-empty string", nil)
	}

	langs, ok := coerceStringSlice(input.Data["target_languages"])
	if !ok || len(langs) == 0 {
		return coreerrors.NewValidationError("target_languages must be a non-empty string array", nil)
	}

	return nil
}

func (t *TranslateTool) Execute(ctx context.Context, input Input) (Output, error) {
	if t.llmManager == nil {
		return Output{}, coreerrors.NewInternalError("llm manager not configured", nil)
	}
	if t.promptEngine == nil {
		return Output{}, coreerrors.NewInternalError("prompt engine not configured", nil)
	}
	if err := t.Validate(input); err != nil {
		return Output{}, err
	}

	sourceText := strings.TrimSpace(input.Data["text"].(string))
	targetLangs, _ := coerceStringSlice(input.Data["target_languages"])

	sourceLang, _ := input.Data["source_language"].(string)
	sourceLang = strings.TrimSpace(sourceLang)

	promptObj, err := t.promptEngine.BuildTextPrompt(ctx, prompt.PromptRequest{
		Task:            prompt.TaskTranslate,
		SourceLanguage:  sourceLang,
		TargetLanguages: targetLangs,
		Variables: map[string]interface{}{
			"source_text": sourceText,
		},
	})
	if err != nil {
		return Output{}, err
	}

	systemPrompt := promptObj.System
	if t.toolCallingEnabled && !t.toolCallingThinking {
		systemPrompt += "\n\n请不要输出解释或思考，仅通过工具调用返回结果。"
	}

	llmReq := &llm.LLMRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   promptObj.User,
	}
	if input.Context != nil && input.Context.OriginalRequest != nil {
		if opts, ok := input.Context.OriginalRequest["options"].(map[string]interface{}); ok {
			llmReq.Options = opts
		}
	}

	if t.toolCallingEnabled {
		llmReq.Tools = submitResultTools(t.OutputSchema(), "Submit translations")
		llmReq.ToolChoice = &llm.ToolChoice{Mode: llm.ToolChoiceRequired}
	}

	llmResp, err := t.llmManager.ProcessWithTimeout(ctx, llmReq)
	if err != nil {
		return Output{}, err
	}

	translations := map[string]string{}

	if t.toolCallingEnabled {
		var toolResult struct {
			Translations map[string]string `json:"translations"`
		}
		if err := llm.ParseToolCallResponse(llmResp, submitResultFunctionName, &toolResult); err == nil {
			for k, v := range toolResult.Translations {
				if strings.TrimSpace(v) != "" {
					translations[k] = v
				}
			}
		}
	}

	if len(translations) == 0 {
		parsed, err := t.promptEngine.ParseResponse(llmResp.Content)
		if err == nil {
			for k, v := range parsed.Sections {
				if strings.TrimSpace(v) != "" {
					translations[k] = v
				}
			}
		}
	}

	// Filter to requested languages to match API semantics.
	filtered := make(map[string]string)
	for _, code := range targetLangs {
		if v, ok := translations[code]; ok {
			filtered[code] = v
		}
	}

	out := Output{
		Data: map[string]interface{}{
			"translations": filtered,
			"raw_response": bestEffortRawResponse(llmResp),
		},
		Metadata: map[string]interface{}{
			"backend":       llmResp.Metadata["backend"],
			"model":         llmResp.Model,
			"prompt_tokens": llmResp.PromptTokens,
			"total_tokens":  llmResp.TotalTokens,
		},
	}
	return out, nil
}

package tool

import (
	"context"
	"strings"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	coreerrors "github.com/Lingualink-VRChat/Lingualink_Core/internal/core/errors"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/llm"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/prompt"
)

type CorrectTranslateTool struct {
	llmManager          *llm.Manager
	promptEngine        *prompt.Engine
	toolCallingEnabled  bool
	toolCallingThinking bool
}

func NewCorrectTranslateTool(llmManager *llm.Manager, promptEngine *prompt.Engine, toolCallingEnabled, allowThinking bool) *CorrectTranslateTool {
	return &CorrectTranslateTool{
		llmManager:          llmManager,
		promptEngine:        promptEngine,
		toolCallingEnabled:  toolCallingEnabled,
		toolCallingThinking: allowThinking,
	}
}

func (t *CorrectTranslateTool) Name() string {
	return "correct_translate"
}

func (t *CorrectTranslateTool) Description() string {
	return "Correct text and translate into target languages (merged)"
}

func (t *CorrectTranslateTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"text": map[string]interface{}{
				"type":        "string",
				"description": "Source text to correct and translate",
			},
			"target_languages": map[string]interface{}{
				"type":        "array",
				"description": "Target language codes",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
			"dictionary": map[string]interface{}{
				"type":        "array",
				"description": "Optional dictionary terms",
			},
		},
		"required": []string{"text", "target_languages"},
	}
}

func (t *CorrectTranslateTool) OutputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"corrected_text": map[string]interface{}{
				"type":        "string",
				"description": "Corrected source text",
			},
			"translations": map[string]interface{}{
				"type":                 "object",
				"description":          "Translation results keyed by language code",
				"additionalProperties": map[string]string{"type": "string"},
			},
		},
		"required": []string{"corrected_text", "translations"},
	}
}

func (t *CorrectTranslateTool) Validate(input Input) error {
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

func (t *CorrectTranslateTool) Execute(ctx context.Context, input Input) (Output, error) {
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

	dictionary := []config.DictionaryTerm{}
	if input.Context != nil {
		dictionary = input.Context.Dictionary
	}

	promptObj, err := t.promptEngine.BuildTextCorrectTranslatePrompt(ctx, sourceText, targetLangs, dictionary)
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
		llmReq.Tools = submitResultTools(t.OutputSchema(), "Submit corrected text and translations")
		llmReq.ToolChoice = &llm.ToolChoice{Mode: llm.ToolChoiceRequired}
	}

	llmResp, err := t.llmManager.ProcessWithTimeout(ctx, llmReq)
	if err != nil {
		return Output{}, err
	}

	correctedText := ""
	translations := map[string]string{}

	if t.toolCallingEnabled {
		var toolResult struct {
			CorrectedText string            `json:"corrected_text"`
			Translations  map[string]string `json:"translations"`
		}
		if err := llm.ParseToolCallResponse(llmResp, submitResultFunctionName, &toolResult); err == nil {
			correctedText = strings.TrimSpace(toolResult.CorrectedText)
			for k, v := range toolResult.Translations {
				if strings.TrimSpace(v) != "" {
					translations[k] = v
				}
			}
		}
	}

	if correctedText == "" || len(translations) == 0 {
		parsed, err := t.promptEngine.ParseResponse(llmResp.Content)
		if err == nil {
			if correctedText == "" {
				correctedText = strings.TrimSpace(parsed.CorrectedText)
			}
			if len(translations) == 0 {
				for k, v := range parsed.Sections {
					if strings.TrimSpace(v) != "" {
						translations[k] = v
					}
				}
			}
		}
	}

	if correctedText == "" {
		correctedText = sourceText
	}

	filtered := make(map[string]string)
	for _, code := range targetLangs {
		if v, ok := translations[code]; ok {
			filtered[code] = v
		}
	}

	out := Output{
		Data: map[string]interface{}{
			"corrected_text": correctedText,
			"translations":   filtered,
			"raw_response":   bestEffortRawResponse(llmResp),
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

package tool

import (
	"context"
	"strings"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	coreerrors "github.com/Lingualink-VRChat/Lingualink_Core/internal/core/errors"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/llm"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/prompt"
)

type CorrectTool struct {
	llmManager          *llm.Manager
	promptEngine        *prompt.Engine
	toolCallingEnabled  bool
	toolCallingThinking bool
}

func NewCorrectTool(llmManager *llm.Manager, promptEngine *prompt.Engine, toolCallingEnabled, allowThinking bool) *CorrectTool {
	return &CorrectTool{
		llmManager:          llmManager,
		promptEngine:        promptEngine,
		toolCallingEnabled:  toolCallingEnabled,
		toolCallingThinking: allowThinking,
	}
}

func (t *CorrectTool) Name() string {
	return "correct"
}

func (t *CorrectTool) Description() string {
	return "Correct ASR transcription text"
}

func (t *CorrectTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"text": map[string]interface{}{
				"type":        "string",
				"description": "Source text to correct",
			},
			"dictionary": map[string]interface{}{
				"type":        "array",
				"description": "Optional dictionary terms",
			},
		},
		"required": []string{"text"},
	}
}

func (t *CorrectTool) OutputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"corrected_text": map[string]interface{}{
				"type":        "string",
				"description": "Corrected source text",
			},
		},
		"required": []string{"corrected_text"},
	}
}

func (t *CorrectTool) Validate(input Input) error {
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
	return nil
}

func (t *CorrectTool) Execute(ctx context.Context, input Input) (Output, error) {
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

	dictionary := []config.DictionaryTerm{}
	if input.Context != nil {
		dictionary = input.Context.Dictionary
	}

	promptObj, err := t.promptEngine.BuildTextCorrectPrompt(ctx, sourceText, dictionary)
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
		llmReq.Tools = submitResultTools(t.OutputSchema(), "Submit corrected text")
		llmReq.ToolChoice = &llm.ToolChoice{Mode: llm.ToolChoiceRequired}
	}

	llmResp, err := t.llmManager.ProcessWithTimeout(ctx, llmReq)
	if err != nil {
		return Output{}, err
	}

	correctedText := ""

	if t.toolCallingEnabled {
		var toolResult struct {
			CorrectedText string `json:"corrected_text"`
		}
		if err := llm.ParseToolCallResponse(llmResp, submitResultFunctionName, &toolResult); err == nil {
			correctedText = strings.TrimSpace(toolResult.CorrectedText)
		}
	}

	if correctedText == "" {
		parsed, err := t.promptEngine.ParseResponse(llmResp.Content)
		if err == nil {
			correctedText = strings.TrimSpace(parsed.CorrectedText)
		}
	}
	if correctedText == "" {
		correctedText = sourceText
	}

	out := Output{
		Data: map[string]interface{}{
			"corrected_text": correctedText,
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

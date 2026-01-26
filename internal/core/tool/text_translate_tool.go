package tool

import (
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/llm"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/prompt"
)

// TextTranslateTool is a translate tool specialized for the /process_text pipeline.
// It reuses TranslateTool implementation but registers under a dedicated tool name.
type TextTranslateTool struct {
	*TranslateTool
}

func NewTextTranslateTool(llmManager *llm.Manager, promptEngine *prompt.Engine, toolCallingEnabled, allowThinking bool) *TextTranslateTool {
	return &TextTranslateTool{
		TranslateTool: NewTranslateTool(llmManager, promptEngine, toolCallingEnabled, allowThinking),
	}
}

func (t *TextTranslateTool) Name() string {
	return "text_translate"
}

func (t *TextTranslateTool) Description() string {
	return "Translate input text into target languages"
}

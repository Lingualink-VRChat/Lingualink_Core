package tool

import (
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/llm"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/prompt"
)

// TextCorrectTranslateTool is a merged correct+translate tool specialized for the /process_text pipeline.
// It reuses CorrectTranslateTool implementation but registers under a dedicated tool name.
type TextCorrectTranslateTool struct {
	*CorrectTranslateTool
}

func NewTextCorrectTranslateTool(llmManager *llm.Manager, promptEngine *prompt.Engine, toolCallingEnabled, allowThinking bool) *TextCorrectTranslateTool {
	return &TextCorrectTranslateTool{
		CorrectTranslateTool: NewCorrectTranslateTool(llmManager, promptEngine, toolCallingEnabled, allowThinking),
	}
}

func (t *TextCorrectTranslateTool) Name() string {
	return "text_correct_translate"
}

func (t *TextCorrectTranslateTool) Description() string {
	return "Correct input text and translate into target languages"
}

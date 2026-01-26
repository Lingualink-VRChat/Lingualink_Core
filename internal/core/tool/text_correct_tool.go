package tool

import (
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/llm"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/prompt"
)

// TextCorrectTool is a correction tool specialized for the /process_text pipeline.
// It reuses CorrectTool implementation but registers under a dedicated tool name.
type TextCorrectTool struct {
	*CorrectTool
}

func NewTextCorrectTool(llmManager *llm.Manager, promptEngine *prompt.Engine, toolCallingEnabled, allowThinking bool) *TextCorrectTool {
	return &TextCorrectTool{
		CorrectTool: NewCorrectTool(llmManager, promptEngine, toolCallingEnabled, allowThinking),
	}
}

func (t *TextCorrectTool) Name() string {
	return "text_correct"
}

func (t *TextCorrectTool) Description() string {
	return "Correct input text"
}

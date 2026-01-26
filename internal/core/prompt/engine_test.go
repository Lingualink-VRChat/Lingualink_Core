package prompt

import (
	"context"
	"testing"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/testutil"
)

func TestEngine_Build_AudioTranscribe(t *testing.T) {
	t.Parallel()

	logger := testutil.NewTestLogger()
	engine, err := NewEngine(newTestPromptConfig(), logger)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	p, err := engine.BuildTextCorrectPrompt(context.Background(), "你好", []config.DictionaryTerm{
		{Term: "Lingualink", Aliases: []string{"林瓜林克"}},
	})
	if err != nil {
		t.Fatalf("BuildTextCorrectPrompt: %v", err)
	}
	if p.System == "" || p.User == "" {
		t.Fatalf("expected non-empty prompts")
	}
}

func TestEngine_Build_AudioTranslate(t *testing.T) {
	t.Parallel()

	logger := testutil.NewTestLogger()
	engine, err := NewEngine(newTestPromptConfig(), logger)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	p, err := engine.BuildTextCorrectTranslatePrompt(context.Background(), "你好", []string{"en", "ja"}, nil)
	if err != nil {
		t.Fatalf("BuildTextCorrectTranslatePrompt: %v", err)
	}
	if p.System == "" || p.User == "" {
		t.Fatalf("expected non-empty prompts")
	}
	if len(p.OutputRules.Sections) != 2 {
		t.Fatalf("sections=%d want 2", len(p.OutputRules.Sections))
	}
	if p.OutputRules.Sections[0].LanguageCode != "en" || p.OutputRules.Sections[1].LanguageCode != "ja" {
		t.Fatalf("unexpected sections: %+v", p.OutputRules.Sections)
	}
}

func TestEngine_BuildTextPrompt(t *testing.T) {
	t.Parallel()

	logger := testutil.NewTestLogger()
	engine, err := NewEngine(newTestPromptConfig(), logger)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	p, err := engine.BuildTextPrompt(context.Background(), PromptRequest{
		Task:            TaskTranslate,
		SourceLanguage:  "zh",
		TargetLanguages: []string{"en", "ja"},
		Variables: map[string]interface{}{
			"source_text": "你好",
		},
	})
	if err != nil {
		t.Fatalf("BuildTextPrompt: %v", err)
	}
	if p.System == "" || p.User == "" {
		t.Fatalf("expected non-empty prompts")
	}
	if len(p.OutputRules.Sections) != 2 {
		t.Fatalf("sections=%d want 2", len(p.OutputRules.Sections))
	}
}

func TestEngine_ParseResponse(t *testing.T) {
	t.Parallel()

	logger := testutil.NewTestLogger()
	engine, err := NewEngine(newTestPromptConfig(), logger)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	content := "OK\n```json\n{\"corrected_text\":\"你好！\",\"translations\":{\"en\":\"hello\"}}\n```\n"
	parsed, err := engine.ParseResponse(content)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}
	if parsed.CorrectedText != "你好！" {
		t.Fatalf("CorrectedText=%q want 你好！", parsed.CorrectedText)
	}
	if parsed.Sections["en"] != "hello" {
		t.Fatalf("en=%q want hello", parsed.Sections["en"])
	}
}

func TestEngine_ParseResponse_NoJSONBlock(t *testing.T) {
	t.Parallel()

	logger := testutil.NewTestLogger()
	engine, err := NewEngine(newTestPromptConfig(), logger)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	if _, err := engine.ParseResponse("no json"); err == nil {
		t.Fatalf("expected error")
	}
}

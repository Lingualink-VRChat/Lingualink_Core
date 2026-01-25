package audio

import (
	"context"
	"strings"
	"testing"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/llm"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/prompt"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/testutil"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/metrics"
)

func newTestPromptConfig() config.PromptConfig {
	return config.PromptConfig{
		Defaults: config.PromptDefaults{
			Task:            "translate",
			TargetLanguages: []string{"en", "ja"},
		},
		Languages: []config.Language{
			{
				Code: "zh",
				Names: map[string]string{
					"display": "中文",
					"english": "Chinese",
				},
				Aliases: []string{"chinese", "中文"},
			},
			{
				Code: "en",
				Names: map[string]string{
					"display": "英文",
					"english": "English",
				},
				Aliases: []string{"english", "英文"},
			},
			{
				Code: "ja",
				Names: map[string]string{
					"display": "日文",
					"english": "Japanese",
				},
				Aliases: []string{"japanese", "日文"},
			},
		},
	}
}

func newTestProcessor(t *testing.T) *Processor {
	t.Helper()

	logger := testutil.NewTestLogger()
	cfg := newTestPromptConfig()
	engine, err := prompt.NewEngine(cfg, logger)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	return NewProcessor(nil, engine, cfg, logger, metrics.NewSimpleMetricsCollector(logger))
}

func TestProcessor_Validate(t *testing.T) {
	t.Parallel()

	p := newTestProcessor(t)
	audioData := testutil.LoadTestAudio(t, "test.wav")

	if err := p.Validate(ProcessRequest{
		Audio:       audioData,
		AudioFormat: "wav",
		Task:        prompt.TaskTranscribe,
	}); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestProcessor_Validate_EmptyAudio(t *testing.T) {
	t.Parallel()

	p := newTestProcessor(t)
	if err := p.Validate(ProcessRequest{
		Audio:       nil,
		AudioFormat: "wav",
		Task:        prompt.TaskTranscribe,
	}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestProcessor_Validate_TooLarge(t *testing.T) {
	t.Parallel()

	p := newTestProcessor(t)
	tooLarge := make([]byte, 32*1024*1024+1)
	if err := p.Validate(ProcessRequest{
		Audio:       tooLarge,
		AudioFormat: "wav",
		Task:        prompt.TaskTranscribe,
	}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestProcessor_Validate_UnsupportedFormat(t *testing.T) {
	t.Parallel()

	p := newTestProcessor(t)
	audioData := []byte("x")
	if err := p.Validate(ProcessRequest{
		Audio:       audioData,
		AudioFormat: "aac",
		Task:        prompt.TaskTranscribe,
	}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestProcessor_Validate_InvalidTask(t *testing.T) {
	t.Parallel()

	p := newTestProcessor(t)
	audioData := []byte("x")
	if err := p.Validate(ProcessRequest{
		Audio:       audioData,
		AudioFormat: "wav",
		Task:        prompt.TaskType("bad"),
	}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestProcessor_BuildLLMRequest_DefaultTargets(t *testing.T) {
	t.Parallel()

	p := newTestProcessor(t)
	audioData := testutil.LoadTestAudio(t, "test.wav")

	llmReq, err := p.BuildLLMRequest(context.Background(), ProcessRequest{
		Audio:       audioData,
		AudioFormat: "wav",
		Task:        prompt.TaskTranslate,
		// empty target languages: should use defaults
	})
	if err != nil {
		t.Fatalf("BuildLLMRequest: %v", err)
	}
	if llmReq.AudioFormat != "wav" {
		t.Fatalf("audio_format=%q want wav", llmReq.AudioFormat)
	}
	if len(llmReq.Audio) != len(audioData) {
		t.Fatalf("audio_size=%d want %d", len(llmReq.Audio), len(audioData))
	}
	if !strings.Contains(llmReq.SystemPrompt, "英文") || !strings.Contains(llmReq.SystemPrompt, "日文") {
		t.Fatalf("expected system prompt to include default target languages, got: %q", llmReq.SystemPrompt)
	}
}

func TestProcessor_BuildSuccessResponse(t *testing.T) {
	t.Parallel()

	p := newTestProcessor(t)

	llmResp := &llm.LLMResponse{
		Content:      "raw",
		Model:        "m",
		PromptTokens: 1,
		TotalTokens:  2,
		Metadata:     map[string]interface{}{"backend": "b"},
	}
	parsed := &prompt.ParsedResponse{
		Sections: map[string]string{
			"原文":         "你好",
			"en":         "hello",
			"ja":         "こんにちは",
			"unexpected": "x",
		},
		Metadata: map[string]interface{}{
			"parser":        "json",
			"parse_success": true,
		},
	}

	resp := p.BuildSuccessResponse(llmResp, parsed, ProcessRequest{
		Task:            prompt.TaskTranslate,
		TargetLanguages: []string{"en"},
		AudioFormat:     "wav",
	})

	if resp.Status != "success" {
		t.Fatalf("status=%q want success", resp.Status)
	}
	if resp.Transcription != "你好" {
		t.Fatalf("transcription=%q want 你好", resp.Transcription)
	}
	if resp.Translations["en"] != "hello" {
		t.Fatalf("en=%q want hello", resp.Translations["en"])
	}
	if _, ok := resp.Translations["ja"]; ok {
		t.Fatalf("did not expect ja translation")
	}
}

func TestProcessor_BuildSuccessResponse_ParseFailed(t *testing.T) {
	t.Parallel()

	p := newTestProcessor(t)

	llmResp := &llm.LLMResponse{Content: "raw", Model: "m", Metadata: map[string]interface{}{"backend": "b"}}
	resp := p.BuildSuccessResponse(llmResp, nil, ProcessRequest{AudioFormat: "wav"})
	if resp.Status != "partial_success" {
		t.Fatalf("status=%q want partial_success", resp.Status)
	}
}

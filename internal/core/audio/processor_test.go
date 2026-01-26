package audio

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/asr"
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

func newTestASRManager(t *testing.T, text string) *asr.Manager {
	t.Helper()

	asrSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[]}`))
			return
		case "/v1/audio/transcriptions":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"language": "zh",
				"duration": 1.0,
				"text":     text,
			})
			return
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(asrSrv.Close)

	logger := testutil.NewTestLogger()
	m, err := asr.NewManager(config.ASRConfig{
		Providers: []config.ASRProvider{
			{
				Name:  "asr1",
				Type:  "whisper",
				URL:   asrSrv.URL + "/v1",
				Model: "whisper-1",
			},
		},
	}, logger)
	if err != nil {
		t.Fatalf("asr.NewManager: %v", err)
	}
	return m
}

func TestProcessor_ProcessDirect_Transcribe_NoCorrection(t *testing.T) {
	t.Parallel()

	logger := testutil.NewTestLogger()
	promptCfg := newTestPromptConfig()
	engine, err := prompt.NewEngine(promptCfg, logger)
	if err != nil {
		t.Fatalf("prompt.NewEngine: %v", err)
	}

	p := NewProcessor(
		newTestASRManager(t, "你好"),
		nil,
		engine,
		promptCfg,
		config.CorrectionConfig{Enabled: false},
		logger,
		metrics.NewSimpleMetricsCollector(logger),
	)

	audioData := testutil.LoadTestAudio(t, "test.wav")
	resp, handled, err := p.ProcessDirect(context.Background(), ProcessRequest{
		Audio:       audioData,
		AudioFormat: "wav",
		Task:        prompt.TaskTranscribe,
	})
	if err != nil {
		t.Fatalf("ProcessDirect: %v", err)
	}
	if !handled {
		t.Fatalf("handled=false want true")
	}
	if resp.Transcription != "你好" {
		t.Fatalf("Transcription=%q want 你好", resp.Transcription)
	}
}

func TestProcessor_BuildLLMRequest_Translate_DefaultTargets(t *testing.T) {
	t.Parallel()

	logger := testutil.NewTestLogger()
	promptCfg := newTestPromptConfig()
	engine, err := prompt.NewEngine(promptCfg, logger)
	if err != nil {
		t.Fatalf("prompt.NewEngine: %v", err)
	}

	p := NewProcessor(
		newTestASRManager(t, "你好"),
		nil,
		engine,
		promptCfg,
		config.CorrectionConfig{Enabled: false},
		logger,
		metrics.NewSimpleMetricsCollector(logger),
	)

	audioData := testutil.LoadTestAudio(t, "test.wav")
	llmReq, err := p.BuildLLMRequest(context.Background(), ProcessRequest{
		Audio:       audioData,
		AudioFormat: "wav",
		Task:        prompt.TaskTranslate,
		// empty target languages -> defaults
	})
	if err != nil {
		t.Fatalf("BuildLLMRequest: %v", err)
	}
	if llmReq.SystemPrompt == "" || llmReq.UserPrompt == "" {
		t.Fatalf("expected non-empty prompts")
	}
	if !strings.Contains(llmReq.UserPrompt, "你好") {
		t.Fatalf("expected user prompt to include ASR text, got: %q", llmReq.UserPrompt)
	}
	if llmReq.Context["asr_text"] != "你好" {
		t.Fatalf("context.asr_text=%v want 你好", llmReq.Context["asr_text"])
	}
}

func TestProcessor_ProcessDirect_Translate_Separated(t *testing.T) {
	t.Parallel()

	var calls int
	llmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		calls++
		body, _ := io.ReadAll(r.Body)
		defer r.Body.Close()

		isCorrection := bytes.Contains(body, []byte("纠正")) || bytes.Contains(body, []byte("纠错"))
		content := ""
		if isCorrection {
			content = "```json\n{\"corrected_text\":\"你好！\"}\n```"
		} else {
			content = "```json\n{\"translations\":{\"en\":\"hello\"}}\n```"
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": content}},
			},
			"usage": map[string]any{
				"prompt_tokens": 1,
				"total_tokens":  2,
			},
		})
	}))
	t.Cleanup(llmSrv.Close)

	logger := testutil.NewTestLogger()
	promptCfg := newTestPromptConfig()
	engine, err := prompt.NewEngine(promptCfg, logger)
	if err != nil {
		t.Fatalf("prompt.NewEngine: %v", err)
	}

	llmManager, err := llm.NewManager(config.BackendsConfig{
		LoadBalancer: config.LoadBalancerConfig{Strategy: "round_robin"},
		Providers: []config.BackendProvider{
			{Name: "test", Type: "openai", URL: llmSrv.URL, Model: "test-model"},
		},
	}, logger)
	if err != nil {
		t.Fatalf("llm.NewManager: %v", err)
	}

	p := NewProcessor(
		newTestASRManager(t, "你好"),
		llmManager,
		engine,
		promptCfg,
		config.CorrectionConfig{Enabled: true, MergeWithTranslation: false},
		logger,
		metrics.NewSimpleMetricsCollector(logger),
	)

	audioData := testutil.LoadTestAudio(t, "test.wav")
	resp, handled, err := p.ProcessDirect(context.Background(), ProcessRequest{
		Audio:           audioData,
		AudioFormat:     "wav",
		Task:            prompt.TaskTranslate,
		TargetLanguages: []string{"en"},
	})
	if err != nil {
		t.Fatalf("ProcessDirect: %v", err)
	}
	if !handled {
		t.Fatalf("handled=false want true")
	}
	if resp.CorrectedText != "你好！" {
		t.Fatalf("CorrectedText=%q want 你好！", resp.CorrectedText)
	}
	if resp.Translations["en"] != "hello" {
		t.Fatalf("en=%q want hello", resp.Translations["en"])
	}
	if calls != 2 {
		t.Fatalf("calls=%d want 2", calls)
	}
}

func TestProcessor_Validate(t *testing.T) {
	t.Parallel()

	logger := testutil.NewTestLogger()
	promptCfg := newTestPromptConfig()
	engine, err := prompt.NewEngine(promptCfg, logger)
	if err != nil {
		t.Fatalf("prompt.NewEngine: %v", err)
	}

	p := NewProcessor(
		newTestASRManager(t, "你好"),
		nil,
		engine,
		promptCfg,
		config.CorrectionConfig{Enabled: false},
		logger,
		metrics.NewSimpleMetricsCollector(logger),
	)

	audioData := testutil.LoadTestAudio(t, "test.wav")
	if err := p.Validate(ProcessRequest{
		Audio:       audioData,
		AudioFormat: "wav",
		Task:        prompt.TaskTranscribe,
	}); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestProcessor_BuildSuccessResponse_UsesContext(t *testing.T) {
	t.Parallel()

	logger := testutil.NewTestLogger()
	promptCfg := newTestPromptConfig()
	engine, err := prompt.NewEngine(promptCfg, logger)
	if err != nil {
		t.Fatalf("prompt.NewEngine: %v", err)
	}

	p := NewProcessor(
		newTestASRManager(t, "你好"),
		nil,
		engine,
		promptCfg,
		config.CorrectionConfig{Enabled: true, MergeWithTranslation: true},
		logger,
		metrics.NewSimpleMetricsCollector(logger),
	)

	llmResp := &llm.LLMResponse{
		Content:      "raw",
		Model:        "m",
		PromptTokens: 1,
		TotalTokens:  2,
		Metadata: map[string]interface{}{
			"backend": "b",
			"context": map[string]interface{}{
				"asr_text":        "你好",
				"asr_duration_ms": int64(time.Millisecond),
			},
		},
	}
	parsed := &prompt.ParsedResponse{
		CorrectedText: "你好！",
		Sections: map[string]string{
			"en": "hello",
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
	if resp.Transcription != "你好" {
		t.Fatalf("Transcription=%q want 你好", resp.Transcription)
	}
	if resp.CorrectedText != "你好！" {
		t.Fatalf("CorrectedText=%q want 你好！", resp.CorrectedText)
	}
	if resp.Translations["en"] != "hello" {
		t.Fatalf("en=%q want hello", resp.Translations["en"])
	}
}

func TestProcessor_ProcessDirect_ToolUse_TranslateMerged_ToolCalling(t *testing.T) {
	t.Parallel()

	llmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}

		body, _ := io.ReadAll(r.Body)
		defer r.Body.Close()

		if !bytes.Contains(body, []byte(`"tools"`)) || !bytes.Contains(body, []byte(`"tool_choice"`)) {
			t.Fatalf("expected tool calling fields, got: %s", string(body))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": nil,
						"tool_calls": []map[string]any{
							{
								"id":   "call_1",
								"type": "function",
								"function": map[string]any{
									"name":      "submit_result",
									"arguments": "{\"corrected_text\":\"你好！\",\"translations\":{\"en\":\"hello\"}}",
								},
							},
						},
					},
				},
			},
			"usage": map[string]any{
				"prompt_tokens": 1,
				"total_tokens":  2,
			},
		})
	}))
	t.Cleanup(llmSrv.Close)

	logger := testutil.NewTestLogger()
	promptCfg := newTestPromptConfig()
	engine, err := prompt.NewEngine(promptCfg, logger)
	if err != nil {
		t.Fatalf("prompt.NewEngine: %v", err)
	}

	llmManager, err := llm.NewManager(config.BackendsConfig{
		LoadBalancer: config.LoadBalancerConfig{Strategy: "round_robin"},
		Providers: []config.BackendProvider{
			{Name: "test", Type: "openai", URL: llmSrv.URL, Model: "test-model"},
		},
	}, logger)
	if err != nil {
		t.Fatalf("llm.NewManager: %v", err)
	}

	p := NewProcessor(
		newTestASRManager(t, "你好"),
		llmManager,
		engine,
		promptCfg,
		config.CorrectionConfig{Enabled: true, MergeWithTranslation: true},
		logger,
		metrics.NewSimpleMetricsCollector(logger),
	).WithPipelineConfig(config.PipelineConfig{
		ToolCalling: config.ToolCallingConfig{
			Enabled:       true,
			AllowThinking: false,
		},
	})

	audioData := testutil.LoadTestAudio(t, "test.wav")
	resp, handled, err := p.ProcessDirect(context.Background(), ProcessRequest{
		Audio:           audioData,
		AudioFormat:     "wav",
		Task:            prompt.TaskTranslate,
		TargetLanguages: []string{"en"},
	})
	if err != nil {
		t.Fatalf("ProcessDirect: %v", err)
	}
	if !handled {
		t.Fatalf("handled=false want true")
	}
	if resp.Transcription != "你好" {
		t.Fatalf("Transcription=%q want 你好", resp.Transcription)
	}
	if resp.CorrectedText != "你好！" {
		t.Fatalf("CorrectedText=%q want 你好！", resp.CorrectedText)
	}
	if resp.Translations["en"] != "hello" {
		t.Fatalf("en=%q want hello", resp.Translations["en"])
	}
	if resp.Metadata["pipeline"] != "translate_merged" {
		t.Fatalf("pipeline=%v want translate_merged", resp.Metadata["pipeline"])
	}
}

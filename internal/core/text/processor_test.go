package text

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/cache"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/llm"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/processing"
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
	return NewProcessor(nil, engine, metrics.NewSimpleMetricsCollector(logger), cfg, logger)
}

func TestProcessor_Validate(t *testing.T) {
	t.Parallel()

	p := newTestProcessor(t)
	if err := p.Validate(ProcessRequest{
		Text:            "hi",
		TargetLanguages: []string{"en"},
	}); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestProcessor_Validate_EmptyText(t *testing.T) {
	t.Parallel()

	p := newTestProcessor(t)
	if err := p.Validate(ProcessRequest{
		Text:            "",
		TargetLanguages: []string{"en"},
	}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestProcessor_Validate_TooLong(t *testing.T) {
	t.Parallel()

	p := newTestProcessor(t)
	longText := strings.Repeat("a", 3001)
	if err := p.Validate(ProcessRequest{
		Text:            longText,
		TargetLanguages: []string{"en"},
	}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestProcessor_Validate_NoTargets(t *testing.T) {
	t.Parallel()

	p := newTestProcessor(t)
	if err := p.Validate(ProcessRequest{Text: "hi"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestProcessor_BuildLLMRequest(t *testing.T) {
	t.Parallel()

	p := newTestProcessor(t)
	llmReq, err := p.BuildLLMRequest(context.Background(), ProcessRequest{
		Text:            "你好",
		TargetLanguages: []string{"en"},
	})
	if err != nil {
		t.Fatalf("BuildLLMRequest: %v", err)
	}
	if llmReq.Audio != nil {
		t.Fatalf("expected no audio")
	}
	if llmReq.SystemPrompt == "" || llmReq.UserPrompt == "" {
		t.Fatalf("expected non-empty prompts")
	}
	if !strings.Contains(llmReq.UserPrompt, "你好") {
		t.Fatalf("expected user prompt to include source text, got: %q", llmReq.UserPrompt)
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
			"原文": "你好",
			"en": "hello",
			"ja": "こんにちは",
		},
		Metadata: map[string]interface{}{
			"parser":        "json",
			"parse_success": true,
		},
	}

	resp := p.BuildSuccessResponse(llmResp, parsed, ProcessRequest{
		Text:            "你好",
		TargetLanguages: []string{"en"},
	})

	if resp.Translations["en"] != "hello" {
		t.Fatalf("en=%q want hello", resp.Translations["en"])
	}
	if _, ok := resp.Translations["ja"]; ok {
		t.Fatalf("did not expect ja translation")
	}
}

func TestProcessor_TranslationCache_HitAvoidsLLM(t *testing.T) {
	var calls atomic.Int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/chat/completions":
			calls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]interface{}{
				"choices": []map[string]interface{}{
					{"message": map[string]interface{}{"content": "```json\n{\"source_text\":\"x\",\"translations\":{\"en\":\"y\"}}\n```"}},
				},
				"usage": map[string]interface{}{
					"prompt_tokens": 1,
					"total_tokens":  2,
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	logger := testutil.NewTestLogger()
	cfg := newTestPromptConfig()
	engine, err := prompt.NewEngine(cfg, logger)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	llmManager, err := llm.NewManager(config.BackendsConfig{
		LoadBalancer: config.LoadBalancerConfig{Strategy: "round_robin"},
		Providers: []config.BackendProvider{
			{Name: "test", Type: "openai", URL: server.URL, Model: "test-model"},
		},
	}, logger)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	translationCache := cache.NewInMemoryCache(10)
	p := NewProcessorWithCache(llmManager, engine, metrics.NewSimpleMetricsCollector(logger), cfg, logger, translationCache, time.Minute)
	service := processing.NewService[ProcessRequest, *ProcessResponse](llmManager, engine, logger)

	req := ProcessRequest{Text: "hi", TargetLanguages: []string{"en"}}
	resp1, err := service.Process(context.Background(), req, p)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if resp1.Translations["en"] != "y" {
		t.Fatalf("expected en translation, got %q", resp1.Translations["en"])
	}
	if calls.Load() != 1 {
		t.Fatalf("calls=%d want 1", calls.Load())
	}

	resp2, err := service.Process(context.Background(), req, p)
	if err != nil {
		t.Fatalf("Process (cached): %v", err)
	}
	if resp2.Translations["en"] != "y" {
		t.Fatalf("expected en translation, got %q", resp2.Translations["en"])
	}
	if calls.Load() != 1 {
		t.Fatalf("calls=%d want 1 (cache hit)", calls.Load())
	}
	if hit, _ := resp2.Metadata["cache_hit"].(bool); !hit {
		t.Fatalf("expected cache_hit=true")
	}
}

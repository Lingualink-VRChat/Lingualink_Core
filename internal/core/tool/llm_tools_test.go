package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/llm"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/prompt"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/testutil"
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

func newTestPromptEngine(t *testing.T) *prompt.Engine {
	t.Helper()

	logger := testutil.NewTestLogger()
	engine, err := prompt.NewEngine(newTestPromptConfig(), logger)
	if err != nil {
		t.Fatalf("prompt.NewEngine: %v", err)
	}
	return engine
}

func newTestLLMManager(t *testing.T, serverURL string) *llm.Manager {
	t.Helper()

	logger := testutil.NewTestLogger()
	m, err := llm.NewManager(config.BackendsConfig{
		LoadBalancer: config.LoadBalancerConfig{Strategy: "round_robin"},
		Providers: []config.BackendProvider{
			{Name: "test", Type: "openai", URL: serverURL, Model: "test-model"},
		},
	}, logger)
	if err != nil {
		t.Fatalf("llm.NewManager: %v", err)
	}
	return m
}

func TestCorrectTool_ToolCalling_ParseToolCall(t *testing.T) {
	t.Parallel()

	llmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}

		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()

		if !bytes.Contains(body, []byte(`"tools"`)) {
			t.Fatalf("expected tools in request, got: %s", string(body))
		}
		if !bytes.Contains(body, []byte(`"tool_choice"`)) {
			t.Fatalf("expected tool_choice in request, got: %s", string(body))
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
									"name":      submitResultFunctionName,
									"arguments": "{\"corrected_text\":\"你好！\"}",
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

	engine := newTestPromptEngine(t)
	tool := NewCorrectTool(newTestLLMManager(t, llmSrv.URL), engine, true, false)

	out, err := tool.Execute(context.Background(), Input{
		Data: map[string]any{"text": "你好"},
		Context: &PipelineContext{
			OriginalRequest: map[string]any{},
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got, _ := out.Data["corrected_text"].(string); got != "你好！" {
		t.Fatalf("corrected_text=%q want 你好！", got)
	}
}

func TestCorrectTool_ToolCalling_FallbackToJSONBlock(t *testing.T) {
	t.Parallel()

	llmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}

		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()

		if !bytes.Contains(body, []byte(`"tools"`)) {
			t.Fatalf("expected tools in request, got: %s", string(body))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": "```json\n{\"corrected_text\":\"你好！\"}\n```",
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

	engine := newTestPromptEngine(t)
	tool := NewCorrectTool(newTestLLMManager(t, llmSrv.URL), engine, true, false)

	out, err := tool.Execute(context.Background(), Input{
		Data: map[string]any{"text": "你好"},
		Context: &PipelineContext{
			OriginalRequest: map[string]any{},
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got, _ := out.Data["corrected_text"].(string); got != "你好！" {
		t.Fatalf("corrected_text=%q want 你好！", got)
	}
}

func TestTranslateTool_NoToolCalling_JSONBlock(t *testing.T) {
	t.Parallel()

	llmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}

		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()

		if bytes.Contains(body, []byte(`"tools"`)) || bytes.Contains(body, []byte(`"tool_choice"`)) {
			t.Fatalf("did not expect tool calling fields, got: %s", string(body))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": "```json\n{\"translations\":{\"en\":\"hello\"}}\n```",
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

	engine := newTestPromptEngine(t)
	tool := NewTranslateTool(newTestLLMManager(t, llmSrv.URL), engine, false, false)

	out, err := tool.Execute(context.Background(), Input{
		Data: map[string]any{
			"text":             "你好",
			"source_language":  "zh",
			"target_languages": []string{"en"},
		},
		Context: &PipelineContext{
			OriginalRequest: map[string]any{},
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	translations, ok := out.Data["translations"].(map[string]string)
	if !ok {
		t.Fatalf("translations type=%T", out.Data["translations"])
	}
	if got := translations["en"]; got != "hello" {
		t.Fatalf("en=%q want hello", got)
	}
}

func TestCorrectTranslateTool_ToolCalling_ParseToolCall(t *testing.T) {
	t.Parallel()

	llmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}

		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()

		if !bytes.Contains(body, []byte(`"tools"`)) {
			t.Fatalf("expected tools in request, got: %s", string(body))
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
									"name":      submitResultFunctionName,
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

	engine := newTestPromptEngine(t)
	tool := NewCorrectTranslateTool(newTestLLMManager(t, llmSrv.URL), engine, true, false)

	out, err := tool.Execute(context.Background(), Input{
		Data: map[string]any{
			"text":             "你好",
			"target_languages": []string{"en"},
		},
		Context: &PipelineContext{
			Dictionary:      nil,
			OriginalRequest: map[string]any{},
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got, _ := out.Data["corrected_text"].(string); got != "你好！" {
		t.Fatalf("corrected_text=%q want 你好！", got)
	}
	translations, ok := out.Data["translations"].(map[string]string)
	if !ok {
		t.Fatalf("translations type=%T", out.Data["translations"])
	}
	if got := translations["en"]; got != "hello" {
		t.Fatalf("en=%q want hello", got)
	}
}

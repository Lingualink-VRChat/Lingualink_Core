package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
)

func TestBaseOpenAICompatibleBackend_Process_ToolCalling_RequestAndParse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		_ = r.Body.Close()

		var req map[string]any
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("unmarshal request: %v", err)
		}

		if _, ok := req["tools"]; !ok {
			t.Fatalf("expected tools in request, got: %s", string(body))
		}
		if _, ok := req["tool_choice"]; !ok {
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
									"name":      "submit_result",
									"arguments": "{\"corrected_text\":\"你好\"}",
								},
							},
						},
					},
				},
			},
			"usage": map[string]any{
				"prompt_tokens": 5,
				"total_tokens":  7,
			},
		})
	}))
	t.Cleanup(srv.Close)

	backend := NewBaseOpenAICompatibleBackend(
		"test",
		srv.URL,
		"",
		"test-model",
		3*time.Second,
		config.LLMParameters{},
		newTestLogger(),
	)

	resp, err := backend.Process(context.Background(), &LLMRequest{
		SystemPrompt: "sys",
		UserPrompt:   "user",
		Tools: []ToolDefinition{
			{
				Type: "function",
				Function: ToolFunctionDefinition{
					Name:        "submit_result",
					Description: "submit",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"corrected_text": map[string]any{"type": "string"},
						},
						"required": []string{"corrected_text"},
					},
				},
			},
		},
		ToolChoice: &ToolChoice{Mode: ToolChoiceRequired},
	})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	if len(resp.ToolCalls) != 1 {
		t.Fatalf("ToolCalls=%d want 1", len(resp.ToolCalls))
	}

	var parsed struct {
		CorrectedText string `json:"corrected_text"`
	}
	if err := ParseToolCallResponse(resp, "submit_result", &parsed); err != nil {
		t.Fatalf("ParseToolCallResponse: %v", err)
	}
	if parsed.CorrectedText != "你好" {
		t.Fatalf("corrected_text=%q want 你好", parsed.CorrectedText)
	}
}

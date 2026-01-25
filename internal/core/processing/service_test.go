package processing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/llm"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/prompt"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/testutil"
)

type testReq struct {
	targets []string
}

func (r testReq) GetTargetLanguages() []string {
	return r.targets
}

type testResp struct {
	processingTime float64
}

func (r *testResp) SetProcessingTime(seconds float64) {
	r.processingTime = seconds
}

type mockLogicHandler struct {
	validateErr error
	buildErr    error
	parsedSeen  *prompt.ParsedResponse
}

func (h *mockLogicHandler) Validate(req testReq) error {
	return h.validateErr
}

func (h *mockLogicHandler) BuildLLMRequest(ctx context.Context, req testReq) (*llm.LLMRequest, error) {
	if h.buildErr != nil {
		return nil, h.buildErr
	}
	return &llm.LLMRequest{UserPrompt: "hi"}, nil
}

func (h *mockLogicHandler) BuildSuccessResponse(llmResp *llm.LLMResponse, parsedResp *prompt.ParsedResponse, req testReq) *testResp {
	h.parsedSeen = parsedResp
	return &testResp{processingTime: -1}
}

func newLLMTestServer(t *testing.T, status int, content string, delay time.Duration) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/chat/completions":
			if delay > 0 {
				time.Sleep(delay)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			if status != http.StatusOK {
				_, _ = w.Write([]byte(`{"error":"fail"}`))
				return
			}
			resp := map[string]interface{}{
				"choices": []map[string]interface{}{
					{"message": map[string]interface{}{"content": content}},
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
}

func newTestService(t *testing.T, serverURL string) (*Service[testReq, *testResp], *prompt.Engine) {
	t.Helper()

	logger := testutil.NewTestLogger()
	llmManager, err := llm.NewManager(config.BackendsConfig{
		LoadBalancer: config.LoadBalancerConfig{Strategy: "round_robin"},
		Providers: []config.BackendProvider{
			{Name: "test", Type: "openai", URL: serverURL, Model: "test-model"},
		},
	}, logger)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	engine, err := prompt.NewEngine(config.PromptConfig{}, logger)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	return NewService[testReq, *testResp](llmManager, engine, logger), engine
}

func TestService_Process_Success(t *testing.T) {
	server := newLLMTestServer(t, http.StatusOK, "```json\n{\"source_text\":\"x\",\"translations\":{\"en\":\"y\"}}\n```", 2*time.Millisecond)
	t.Cleanup(server.Close)

	service, _ := newTestService(t, server.URL)
	handler := &mockLogicHandler{}

	resp, err := service.Process(context.Background(), testReq{targets: []string{"en"}}, handler)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if handler.parsedSeen == nil || handler.parsedSeen.Sections["en"] != "y" {
		t.Fatalf("expected parsed response to be passed to handler")
	}
	if resp.processingTime <= 0 {
		t.Fatalf("processingTime=%v want >0", resp.processingTime)
	}
}

func TestService_Process_ValidateFail(t *testing.T) {
	server := newLLMTestServer(t, http.StatusOK, "```json\n{\"source_text\":\"x\",\"translations\":{}}\n```", 0)
	t.Cleanup(server.Close)

	service, _ := newTestService(t, server.URL)
	handler := &mockLogicHandler{validateErr: context.Canceled}

	if _, err := service.Process(context.Background(), testReq{}, handler); err == nil {
		t.Fatalf("expected error")
	}
}

func TestService_Process_BuildLLMRequestFail(t *testing.T) {
	server := newLLMTestServer(t, http.StatusOK, "```json\n{\"source_text\":\"x\",\"translations\":{}}\n```", 0)
	t.Cleanup(server.Close)

	service, _ := newTestService(t, server.URL)
	handler := &mockLogicHandler{buildErr: context.Canceled}

	if _, err := service.Process(context.Background(), testReq{}, handler); err == nil {
		t.Fatalf("expected error")
	}
}

func TestService_Process_LLMFail(t *testing.T) {
	server := newLLMTestServer(t, http.StatusInternalServerError, "", 0)
	t.Cleanup(server.Close)

	service, _ := newTestService(t, server.URL)
	handler := &mockLogicHandler{}

	if _, err := service.Process(context.Background(), testReq{}, handler); err == nil {
		t.Fatalf("expected error")
	}
}

func TestService_Process_ParseFail(t *testing.T) {
	server := newLLMTestServer(t, http.StatusOK, "no json", 0)
	t.Cleanup(server.Close)

	service, _ := newTestService(t, server.URL)
	handler := &mockLogicHandler{}

	if _, err := service.Process(context.Background(), testReq{}, handler); err == nil {
		t.Fatalf("expected error")
	}
}

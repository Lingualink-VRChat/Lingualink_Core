package integrationtest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/api/handlers"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/api/middleware"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/api/routes"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/audio"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/cache"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/llm"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/processing"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/prompt"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/text"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/testutil"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/auth"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/metrics"
	"github.com/gin-gonic/gin"
)

// TestServer is an integration test server that runs the full Lingualink Gin router.
type TestServer struct {
	Server *httptest.Server
	Client *http.Client
	Config *config.Config
	APIKey string

	llmServer *httptest.Server
}

// NewTestServer starts a new in-memory Lingualink server wired to a mock OpenAI-compatible backend.
func NewTestServer(t *testing.T) *TestServer {
	t.Helper()

	gin.SetMode(gin.TestMode)

	logger := testutil.NewTestLogger()
	metricsCollector := metrics.NewSimpleMetricsCollector(logger)

	llmContent := "```json\n{\"transcription\":\"你好\",\"translations\":{\"en\":\"hello\"}}\n```"
	llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/models":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": []map[string]interface{}{}})
		case "/chat/completions":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"choices": []map[string]interface{}{
					{"message": map[string]interface{}{"content": llmContent}},
				},
				"usage": map[string]interface{}{
					"prompt_tokens": 1,
					"total_tokens":  2,
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(llmServer.Close)

	backendCfg := config.BackendsConfig{
		LoadBalancer: config.LoadBalancerConfig{Strategy: "round_robin"},
		Providers: []config.BackendProvider{
			{Name: "test", Type: "openai", URL: llmServer.URL, Model: "test-model"},
		},
	}
	llmManager, err := llm.NewManager(backendCfg, logger)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	promptCfg := config.PromptConfig{
		Defaults: config.PromptDefaults{
			Task:            "translate",
			TargetLanguages: []string{"en"},
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
		},
	}
	promptEngine, err := prompt.NewEngine(promptCfg, logger)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	cfg := &config.Config{
		Server:   config.ServerConfig{Mode: "test", Port: 8080, Host: "127.0.0.1"},
		Auth:     config.AuthConfig{Strategies: []config.AuthStrategy{{Type: "api_key", Enabled: true}}},
		Backends: backendCfg,
		Prompt:   promptCfg,
		Logging:  config.LoggingConfig{Level: "debug", Format: "json"},
	}

	keysPath := filepath.Join(t.TempDir(), "api_keys.json")
	t.Setenv("LINGUALINK_KEYS_FILE", keysPath)

	authenticator := auth.NewMultiAuthenticator(cfg.Auth, logger)

	audioProcessor := audio.NewProcessor(llmManager, promptEngine, promptCfg, logger, metricsCollector)
	translationCache := cache.NewInMemoryCache(1000)
	textProcessor := text.NewProcessorWithCache(llmManager, promptEngine, metricsCollector, promptCfg, logger, translationCache, 5*time.Minute)
	audioProcessingService := processing.NewService[audio.ProcessRequest, *audio.ProcessResponse](llmManager, promptEngine, logger)
	textProcessingService := processing.NewService[text.ProcessRequest, *text.ProcessResponse](llmManager, promptEngine, logger)
	statusStore := processing.NewInMemoryStatusStore(5 * time.Minute)

	handler := handlers.NewHandler(audioProcessor, textProcessor, audioProcessingService, textProcessingService, statusStore, authenticator, logger, metricsCollector, cfg, llmManager)

	router := gin.New()
	router.Use(middleware.RequestID())
	router.Use(middleware.Recovery(logger))
	routes.RegisterRoutes(router, handler, authenticator)

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	return &TestServer{
		Server:    srv,
		Client:    srv.Client(),
		Config:    cfg,
		APIKey:    "lingualink-demo-key",
		llmServer: llmServer,
	}
}

// DoRequest issues an HTTP request against the test server.
func (ts *TestServer) DoRequest(method, path string, body any) (*http.Response, error) {
	var bodyReader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(b)
	} else {
		bodyReader = bytes.NewReader(nil)
	}

	req, err := http.NewRequest(method, ts.Server.URL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if ts.APIKey != "" {
		req.Header.Set("X-API-Key", ts.APIKey)
	}
	return ts.Client.Do(req)
}

// Cleanup closes the underlying servers. It is safe to call multiple times.
func (ts *TestServer) Cleanup() {
	if ts.Server != nil {
		ts.Server.Close()
		ts.Server = nil
	}
	if ts.llmServer != nil {
		ts.llmServer.Close()
		ts.llmServer = nil
	}
}

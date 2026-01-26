package handlers_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/api/handlers"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/api/middleware"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/api/routes"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/asr"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/audio"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/llm"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/processing"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/prompt"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/text"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/testutil"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/auth"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/metrics"
	"github.com/gin-gonic/gin"
)

func newLLMServer(t *testing.T, content string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/models":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{},
			})
		case "/chat/completions":
			time.Sleep(2 * time.Millisecond)
			w.Header().Set("Content-Type", "application/json")
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

func writeTestKeysFile(t *testing.T, path string) {
	t.Helper()

	keys := `{
  "keys": {
    "user-key": {
      "id": "user-1",
      "requests_per_minute": 60,
      "enabled": true
    },
    "service-key": {
      "id": "backend-service",
      "requests_per_minute": 60,
      "enabled": true
    }
  }
}`
	if err := os.WriteFile(path, []byte(keys), 0600); err != nil {
		t.Fatalf("write keys file: %v", err)
	}
}

func newTestPromptConfig() config.PromptConfig {
	return config.PromptConfig{
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
}

func minimalWAV() []byte {
	buf := make([]byte, 100)
	copy(buf[0:], []byte("RIFF"))
	copy(buf[8:], []byte("WAVE"))
	return buf
}

func newTestRouter(t *testing.T) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)

	logger := testutil.NewTestLogger()
	metricsCollector := metrics.NewSimpleMetricsCollector(logger)

	llmContent := "```json\n{\"translations\":{\"en\":\"hello\"}}\n```"
	llmServer := newLLMServer(t, llmContent)
	t.Cleanup(llmServer.Close)

	asrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{},
			})
		case "/v1/audio/transcriptions":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"language": "zh",
				"duration": 1.0,
				"text":     "你好",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(asrServer.Close)

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

	asrManager, err := asr.NewManager(config.ASRConfig{
		Providers: []config.ASRProvider{
			{Name: "asr", Type: "whisper", URL: asrServer.URL + "/v1", Model: "whisper-1"},
		},
	}, logger)
	if err != nil {
		t.Fatalf("asr.NewManager: %v", err)
	}

	promptCfg := newTestPromptConfig()
	promptEngine, err := prompt.NewEngine(promptCfg, logger)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	correctionCfg := config.CorrectionConfig{Enabled: false, MergeWithTranslation: true}
	audioProcessor := audio.NewProcessor(asrManager, llmManager, promptEngine, promptCfg, correctionCfg, logger, metricsCollector)
	textProcessor := text.NewProcessor(llmManager, promptEngine, metricsCollector, promptCfg, logger).WithCorrectionConfig(correctionCfg)
	audioProcessingService := processing.NewService[audio.ProcessRequest, *audio.ProcessResponse](llmManager, promptEngine, logger)
	textProcessingService := processing.NewService[text.ProcessRequest, *text.ProcessResponse](llmManager, promptEngine, logger)
	statusStore := processing.NewInMemoryStatusStore(5 * time.Minute)

	keysPath := filepath.Join(t.TempDir(), "api_keys.json")
	writeTestKeysFile(t, keysPath)
	t.Setenv("LINGUALINK_KEYS_FILE", keysPath)

	authenticator := auth.NewMultiAuthenticator(config.AuthConfig{
		Strategies: []config.AuthStrategy{
			{Type: "api_key", Enabled: true},
		},
	}, logger)

	cfg := &config.Config{
		Server:     config.ServerConfig{Mode: "test", Port: 8080, Host: "127.0.0.1"},
		Auth:       config.AuthConfig{Strategies: []config.AuthStrategy{{Type: "api_key", Enabled: true}}},
		ASR:        config.ASRConfig{Providers: []config.ASRProvider{{Name: "asr", Type: "whisper", URL: asrServer.URL + "/v1", Model: "whisper-1"}}},
		Correction: correctionCfg,
		Backends:   backendCfg,
		Prompt:     promptCfg,
		Logging:    config.LoggingConfig{Level: "debug", Format: "json"},
	}
	handler := handlers.NewHandler(audioProcessor, textProcessor, audioProcessingService, textProcessingService, statusStore, authenticator, logger, metricsCollector, cfg, llmManager, asrManager)

	router := gin.New()
	router.Use(middleware.RequestID())
	router.Use(middleware.Recovery(logger))

	routes.RegisterRoutes(router, handler, authenticator)
	return router
}

func doRequest(t *testing.T, router http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

func TestHealthCheck_Basic(t *testing.T) {
	router := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	resp := doRequest(t, router, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", resp.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["status"] != "healthy" {
		t.Fatalf("status=%v want healthy", body["status"])
	}
}

func TestHealthCheck_Detailed(t *testing.T) {
	router := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health?detailed=true", nil)
	resp := doRequest(t, router, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", resp.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := body["services"]; !ok {
		t.Fatalf("expected services in detailed response")
	}
}

func TestLivenessCheck(t *testing.T) {
	router := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/live", nil)
	resp := doRequest(t, router, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", resp.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["status"] != "live" {
		t.Fatalf("status=%v want live", body["status"])
	}
}

func TestReadinessCheck(t *testing.T) {
	router := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ready", nil)
	resp := doRequest(t, router, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", resp.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["status"] != "ready" {
		t.Fatalf("status=%v want ready", body["status"])
	}
}

func TestDeepHealthCheck(t *testing.T) {
	router := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/deep", nil)
	resp := doRequest(t, router, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", resp.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["status"] == "unhealthy" {
		t.Fatalf("status=%v want not unhealthy", body["status"])
	}
}

func TestPrometheusMetricsEndpoint(t *testing.T) {
	router := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	resp := doRequest(t, router, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "lingualink_audio_processing_seconds") {
		t.Fatalf("expected prometheus metric output")
	}
}

func TestGetCapabilities(t *testing.T) {
	router := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities", nil)
	resp := doRequest(t, router, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", resp.Code)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := body["supported_formats"]; !ok {
		t.Fatalf("expected supported_formats")
	}
}

func TestListSupportedLanguages(t *testing.T) {
	router := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/languages", nil)
	resp := doRequest(t, router, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", resp.Code)
	}

	var body struct {
		Languages []map[string]interface{} `json:"languages"`
		Count     int                      `json:"count"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Count != len(body.Languages) {
		t.Fatalf("count=%d len=%d", body.Count, len(body.Languages))
	}
	if body.Count != 2 {
		t.Fatalf("count=%d want 2", body.Count)
	}
}

func TestProcessAudioJSON_Success(t *testing.T) {
	router := newTestRouter(t)
	audioB64 := base64.StdEncoding.EncodeToString(minimalWAV())

	body := []byte(`{"audio":"` + audioB64 + `","audio_format":"wav","task":"transcribe","target_languages":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/process_audio", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "user-key")

	resp := doRequest(t, router, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d want 200, body=%s", resp.Code, resp.Body.String())
	}

	var out map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["status"] != "success" {
		t.Fatalf("status=%v want success", out["status"])
	}
	if out["processing_time"].(float64) <= 0 {
		t.Fatalf("processing_time=%v want >0", out["processing_time"])
	}

	requestID, ok := out["request_id"].(string)
	if !ok || requestID == "" {
		t.Fatalf("missing request_id: %v", out["request_id"])
	}
	if resp.Header().Get("X-Request-ID") != requestID {
		t.Fatalf("X-Request-ID=%q request_id=%q", resp.Header().Get("X-Request-ID"), requestID)
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/api/v1/status/"+requestID, nil)
	statusReq.Header.Set("X-API-Key", "user-key")
	statusResp := doRequest(t, router, statusReq)
	if statusResp.Code != http.StatusOK {
		t.Fatalf("status=%d want 200, body=%s", statusResp.Code, statusResp.Body.String())
	}

	var st map[string]interface{}
	if err := json.Unmarshal(statusResp.Body.Bytes(), &st); err != nil {
		t.Fatalf("unmarshal status: %v", err)
	}
	if st["status"] != "completed" {
		t.Fatalf("status=%v want completed", st["status"])
	}
}

func TestProcessAudioJSON_NoAuth(t *testing.T) {
	router := newTestRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/process_audio", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")

	resp := doRequest(t, router, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", resp.Code)
	}
}

func TestProcessAudioJSON_InvalidJSON(t *testing.T) {
	router := newTestRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/process_audio", bytes.NewReader([]byte(`{bad json`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "user-key")

	resp := doRequest(t, router, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", resp.Code)
	}
}

func TestProcessAudioJSON_InvalidBase64(t *testing.T) {
	router := newTestRouter(t)
	body := []byte(`{"audio":"not_base64","audio_format":"wav","task":"transcribe","target_languages":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/process_audio", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "user-key")

	resp := doRequest(t, router, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", resp.Code)
	}
}

func TestProcessText_Success(t *testing.T) {
	router := newTestRouter(t)
	body := []byte(`{"text":"你好","target_languages":["en"]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/process_text", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "user-key")

	resp := doRequest(t, router, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d want 200, body=%s", resp.Code, resp.Body.String())
	}

	var out map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	requestID, ok := out["request_id"].(string)
	if !ok || requestID == "" {
		t.Fatalf("missing request_id: %v", out["request_id"])
	}
	metadata, ok := out["metadata"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing metadata: %v", out["metadata"])
	}
	if metadata["pipeline"] != "text_translate" {
		t.Fatalf("metadata.pipeline=%v want text_translate", metadata["pipeline"])
	}
	if _, ok := metadata["step_durations_ms"].(map[string]interface{}); !ok {
		t.Fatalf("metadata.step_durations_ms=%v want object", metadata["step_durations_ms"])
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/api/v1/status/"+requestID, nil)
	statusReq.Header.Set("X-API-Key", "user-key")
	statusResp := doRequest(t, router, statusReq)
	if statusResp.Code != http.StatusOK {
		t.Fatalf("status=%d want 200, body=%s", statusResp.Code, statusResp.Body.String())
	}
}

func TestProcessTextBatch_Success(t *testing.T) {
	router := newTestRouter(t)
	body := []byte(`{"texts":["你好","再见"],"target_languages":["en"]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/process_text_batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "user-key")

	resp := doRequest(t, router, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d want 200, body=%s", resp.Code, resp.Body.String())
	}

	var out map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["request_id"] == "" {
		t.Fatalf("missing request_id")
	}
	results, ok := out["results"].([]interface{})
	if !ok || len(results) != 2 {
		t.Fatalf("results=%v want 2 items", out["results"])
	}
	for i, item := range results {
		m, ok := item.(map[string]interface{})
		if !ok {
			t.Fatalf("results[%d]=%T want object", i, item)
		}
		metadata, ok := m["metadata"].(map[string]interface{})
		if !ok {
			t.Fatalf("results[%d].metadata=%v want object", i, m["metadata"])
		}
		if metadata["pipeline"] != "text_translate" {
			t.Fatalf("results[%d].metadata.pipeline=%v want text_translate", i, metadata["pipeline"])
		}
	}
	if out["count"].(float64) != 2 {
		t.Fatalf("count=%v want 2", out["count"])
	}
}

func TestGetMetrics_RequiresServiceIdentity(t *testing.T) {
	router := newTestRouter(t)

	// user key -> forbidden
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/metrics", nil)
	req.Header.Set("X-API-Key", "user-key")
	resp := doRequest(t, router, req)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403", resp.Code)
	}

	// service key -> ok
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/admin/metrics", nil)
	req2.Header.Set("X-API-Key", "service-key")
	resp2 := doRequest(t, router, req2)
	if resp2.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", resp2.Code)
	}
}

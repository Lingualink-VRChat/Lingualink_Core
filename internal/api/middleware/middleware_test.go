package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/testutil"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/auth"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/metrics"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func newTestAuthenticator(t *testing.T, logger *logrus.Logger) *auth.MultiAuthenticator {
	return newTestAuthenticatorWithRPM(t, logger, 60)
}

func newTestAuthenticatorWithRPM(t *testing.T, logger *logrus.Logger, rpm int) *auth.MultiAuthenticator {
	t.Helper()

	keysPath := filepath.Join(t.TempDir(), "api_keys.json")
	keys := `{
  "keys": {
    "user-key": {
      "id": "user-1",
      "requests_per_minute": %d,
      "enabled": true
    }
  }
}`
	if err := os.WriteFile(keysPath, []byte(fmt.Sprintf(keys, rpm)), 0600); err != nil {
		t.Fatalf("write keys: %v", err)
	}
	t.Setenv("LINGUALINK_KEYS_FILE", keysPath)

	return auth.NewMultiAuthenticator(config.AuthConfig{
		Strategies: []config.AuthStrategy{
			{Type: "api_key", Enabled: true},
		},
	}, logger)
}

func resetRateLimitStore() {
	ResetRateLimitStore()
}

func TestRequestID_Generated(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(RequestID())
	r.GET("/x", func(c *gin.Context) {
		rid, _ := c.Get("request_id")
		c.String(200, rid.(string))
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", rr.Code)
	}
	if rr.Header().Get("X-Request-ID") == "" {
		t.Fatalf("expected X-Request-ID header")
	}
}

func TestRequestID_Passthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(RequestID())
	r.GET("/x", func(c *gin.Context) { c.Status(200) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-Request-ID", "req-123")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Header().Get("X-Request-ID") != "req-123" {
		t.Fatalf("request id not passthrough, got %q", rr.Header().Get("X-Request-ID"))
	}
}

func TestAuth_ValidKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := testutil.NewTestLogger()
	authenticator := newTestAuthenticator(t, logger)

	r := gin.New()
	r.Use(Auth(authenticator))
	r.GET("/x", func(c *gin.Context) { c.Status(200) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-API-Key", "user-key")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d want 200, body=%s", rr.Code, rr.Body.String())
	}
}

func TestAuth_ValidBearerKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := testutil.NewTestLogger()
	authenticator := newTestAuthenticator(t, logger)

	r := gin.New()
	r.Use(Auth(authenticator))
	r.GET("/x", func(c *gin.Context) { c.Status(200) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer user-key")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d want 200, body=%s", rr.Code, rr.Body.String())
	}
}

func TestAuth_InvalidKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := testutil.NewTestLogger()
	authenticator := newTestAuthenticator(t, logger)

	r := gin.New()
	r.Use(Auth(authenticator))
	r.GET("/x", func(c *gin.Context) { c.Status(200) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-API-Key", "bad-key")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rr.Code)
	}
}

func TestAuth_NoKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := testutil.NewTestLogger()
	authenticator := newTestAuthenticator(t, logger)

	r := gin.New()
	r.Use(Auth(authenticator))
	r.GET("/x", func(c *gin.Context) { c.Status(200) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rr.Code)
	}
}

func TestAuth_RateLimitExceeded(t *testing.T) {
	gin.SetMode(gin.TestMode)

	resetRateLimitStore()

	logger := testutil.NewTestLogger()
	authenticator := newTestAuthenticatorWithRPM(t, logger, 1)

	r := gin.New()
	r.Use(Auth(authenticator))
	r.GET("/x", func(c *gin.Context) { c.Status(200) })

	req1 := httptest.NewRequest(http.MethodGet, "/x", nil)
	req1.Header.Set("X-API-Key", "user-key")
	rr1 := httptest.NewRecorder()
	r.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Fatalf("first status=%d want 200", rr1.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/x", nil)
	req2.Header.Set("X-API-Key", "user-key")
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusTooManyRequests {
		t.Fatalf("second status=%d want 429 body=%s", rr2.Code, rr2.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(rr2.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body["error"] != errCodeRateLimitExceeded {
		t.Fatalf("error=%q want %q", body["error"], errCodeRateLimitExceeded)
	}
}

func TestAllowRequestByRateLimit_FreeQuotaReturnsSpecificError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	resetRateLimitStore()

	identity := &auth.Identity{
		ID:   "user-free",
		Type: auth.IdentityTypeUser,
		Metadata: map[string]interface{}{
			"free_quota": true,
		},
		RateLimits: &auth.RateLimitConfig{
			RequestsPerMinute: 1,
			BurstSize:         1,
			WindowSize:        time.Hour,
		},
	}

	c1, _ := gin.CreateTestContext(httptest.NewRecorder())
	if !allowRequestByRateLimit(identity, time.Now(), c1) {
		t.Fatal("expected first request to pass")
	}

	recorder := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(recorder)
	if allowRequestByRateLimit(identity, time.Now(), c2) {
		t.Fatal("expected second request to be limited")
	}
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d want 429", recorder.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body["error"] != errCodeFreeTrialQuotaExhausted {
		t.Fatalf("error=%q want %q", body["error"], errCodeFreeTrialQuotaExhausted)
	}
}

type captureHook struct {
	mu      sync.Mutex
	entries []*logrus.Entry
}

func (h *captureHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *captureHook) Fire(entry *logrus.Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.entries = append(h.entries, entry)
	return nil
}

func TestLogging_IncludesRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := testutil.NewTestLogger()
	hook := &captureHook{}
	logger.AddHook(hook)

	r := gin.New()
	r.Use(RequestID())
	r.Use(Logging(logger))
	r.GET("/x", func(c *gin.Context) { c.Status(200) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-Request-ID", "req-1")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	hook.mu.Lock()
	defer hook.mu.Unlock()
	if len(hook.entries) == 0 {
		t.Fatalf("expected at least one log entry")
	}
	last := hook.entries[len(hook.entries)-1]
	if last.Data["request_id"] != "req-1" {
		t.Fatalf("request_id=%v want req-1", last.Data["request_id"])
	}
}

type mockCollector struct {
	mu           sync.Mutex
	latencyCalls int
	counterCalls int
	lastTags     map[string]string
}

func (c *mockCollector) RecordLatency(name string, duration time.Duration, tags map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.latencyCalls++
	c.lastTags = tags
}

func (c *mockCollector) RecordCounter(name string, value int64, tags map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counterCalls++
	c.lastTags = tags
}

func (c *mockCollector) RecordGauge(name string, value float64, tags map[string]string) {}

func (c *mockCollector) GetMetrics() map[string]interface{} { return nil }

func TestMetrics_Records(t *testing.T) {
	gin.SetMode(gin.TestMode)

	collector := &mockCollector{}

	r := gin.New()
	r.Use(Metrics(collector))
	r.GET("/x", func(c *gin.Context) { c.Status(204) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	collector.mu.Lock()
	defer collector.mu.Unlock()
	if collector.latencyCalls != 1 || collector.counterCalls != 1 {
		t.Fatalf("latencyCalls=%d counterCalls=%d", collector.latencyCalls, collector.counterCalls)
	}
	if collector.lastTags["path"] != "/x" {
		t.Fatalf("path tag=%q want /x", collector.lastTags["path"])
	}
	if collector.lastTags["method"] != http.MethodGet {
		t.Fatalf("method tag=%q want GET", collector.lastTags["method"])
	}
	if collector.lastTags["status"] == "" {
		t.Fatalf("expected status tag")
	}
}

func TestRecovery_CatchesPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := testutil.NewTestLogger()
	r := gin.New()
	r.Use(Recovery(logger))
	r.GET("/panic", func(c *gin.Context) { panic("boom") })

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d want 500", rr.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["error"] == "" {
		t.Fatalf("expected error message")
	}
}

var _ metrics.MetricsCollector = (*mockCollector)(nil)

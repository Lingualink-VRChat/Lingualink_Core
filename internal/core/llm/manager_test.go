package llm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
)

type mockBackend struct {
	name       string
	shouldFail bool
	response   *LLMResponse
	delay      time.Duration
	healthErr  error
}

func (b *mockBackend) Process(ctx context.Context, req *LLMRequest) (*LLMResponse, error) {
	if b.delay > 0 {
		time.Sleep(b.delay)
	}
	if b.shouldFail {
		return nil, errors.New("backend failed")
	}
	if b.response == nil {
		return &LLMResponse{Content: "ok", Model: "mock"}, nil
	}
	return b.response, nil
}

func (b *mockBackend) HealthCheck(ctx context.Context) error {
	return b.healthErr
}

func (b *mockBackend) GetCapabilities() Capabilities {
	return Capabilities{}
}

func (b *mockBackend) GetName() string {
	return b.name
}

func containsString(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

func TestNewManager_Success(t *testing.T) {
	t.Parallel()

	logger := newTestLogger()
	m, err := NewManager(config.BackendsConfig{
		LoadBalancer: config.LoadBalancerConfig{Strategy: "round_robin"},
		Providers: []config.BackendProvider{
			{Name: "test", Type: "openai", URL: "http://example.com", Model: "test-model"},
		},
	}, logger)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if !containsString(m.ListBackends(), "test") {
		t.Fatalf("expected backend 'test' in list")
	}
	if _, ok := m.GetBackend("test"); !ok {
		t.Fatalf("expected GetBackend(test) ok")
	}
}

func TestNewManager_NoBackends(t *testing.T) {
	t.Parallel()

	logger := newTestLogger()
	if _, err := NewManager(config.BackendsConfig{}, logger); err == nil {
		t.Fatalf("expected error")
	}
}

func TestNewManager_UnknownBackendType(t *testing.T) {
	t.Parallel()

	logger := newTestLogger()
	if _, err := NewManager(config.BackendsConfig{
		Providers: []config.BackendProvider{{Name: "x", Type: "unknown"}},
	}, logger); err == nil {
		t.Fatalf("expected error")
	}
}

func TestManager_Process_Success(t *testing.T) {
	t.Parallel()

	logger := newTestLogger()
	backend := &mockBackend{
		name:     "mock",
		response: &LLMResponse{Content: "ok", Model: "mock"},
		delay:    2 * time.Millisecond,
	}

	lb := NewLoadBalancer("round_robin", logger)
	lb.AddBackend(backend)

	m := &Manager{
		backends:     map[string]LLMBackend{backend.name: backend},
		loadBalancer: lb,
		logger:       logger,
	}

	resp, err := m.Process(context.Background(), &LLMRequest{UserPrompt: "hi"})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if resp.Metadata["backend"] != backend.name {
		t.Fatalf("backend=%v want %s", resp.Metadata["backend"], backend.name)
	}
	if resp.Duration <= 0 {
		t.Fatalf("duration=%v want >0", resp.Duration)
	}
}

func TestManager_Process_BackendFailure(t *testing.T) {
	t.Parallel()

	logger := newTestLogger()
	backend := &mockBackend{name: "mock", shouldFail: true}

	lb := NewLoadBalancer("round_robin", logger)
	lb.AddBackend(backend)

	m := &Manager{
		backends:     map[string]LLMBackend{backend.name: backend},
		loadBalancer: lb,
		logger:       logger,
	}

	if _, err := m.Process(context.Background(), &LLMRequest{UserPrompt: "hi"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestManager_GetBackend_NotFound(t *testing.T) {
	t.Parallel()

	m := &Manager{backends: make(map[string]LLMBackend)}
	if _, ok := m.GetBackend("nope"); ok {
		t.Fatalf("expected not found")
	}
}

func TestManager_HealthCheck(t *testing.T) {
	t.Parallel()

	logger := newTestLogger()
	backendOK := &mockBackend{name: "ok", healthErr: nil}
	backendBad := &mockBackend{name: "bad", healthErr: errors.New("down")}

	m := &Manager{
		backends: map[string]LLMBackend{
			backendOK.name:  backendOK,
			backendBad.name: backendBad,
		},
		logger: logger,
	}

	results := m.HealthCheck(context.Background())
	if results["ok"] != nil {
		t.Fatalf("expected ok backend to be healthy")
	}
	if results["bad"] == nil {
		t.Fatalf("expected bad backend to have error")
	}
}

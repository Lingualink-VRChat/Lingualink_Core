package llm

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	"github.com/sirupsen/logrus"
)

// LLMRequest LLM请求
type LLMRequest struct {
	SystemPrompt string                 `json:"system_prompt"`
	UserPrompt   string                 `json:"user_prompt"`
	Audio        []byte                 `json:"audio"`
	AudioFormat  string                 `json:"audio_format"`
	Model        string                 `json:"model,omitempty"`
	Options      map[string]interface{} `json:"options,omitempty"`
}

// LLMResponse LLM响应
type LLMResponse struct {
	Content      string                 `json:"content"`
	Model        string                 `json:"model"`
	PromptTokens int                    `json:"prompt_tokens"`
	TotalTokens  int                    `json:"total_tokens"`
	Duration     time.Duration          `json:"duration"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// Capabilities 后端能力
type Capabilities struct {
	SupportsAudio      bool     `json:"supports_audio"`
	SupportedFormats   []string `json:"supported_formats"`
	MaxAudioSize       int64    `json:"max_audio_size"`
	SupportsStreaming  bool     `json:"supports_streaming"`
	SupportedLanguages []string `json:"supported_languages"`
}

// LLMBackend LLM后端接口
type LLMBackend interface {
	Process(ctx context.Context, req *LLMRequest) (*LLMResponse, error)
	HealthCheck(ctx context.Context) error
	GetCapabilities() Capabilities
	GetName() string
}

// Manager LLM管理器
type Manager struct {
	backends     map[string]LLMBackend
	loadBalancer LoadBalancer
	logger       *logrus.Logger
	mu           sync.RWMutex
}

// NewManager 创建LLM管理器
func NewManager(cfg config.BackendsConfig, logger *logrus.Logger) *Manager {
	manager := &Manager{
		backends: make(map[string]LLMBackend),
		logger:   logger,
	}

	// 初始化负载均衡器
	manager.loadBalancer = NewLoadBalancer(cfg.LoadBalancer.Strategy, logger)

	// 注册后端
	for _, provider := range cfg.Providers {
		backend, err := manager.createBackend(provider)
		if err != nil {
			logger.Errorf("Failed to create backend %s: %v", provider.Name, err)
			continue
		}

		manager.RegisterBackend(backend)
		logger.Infof("Registered LLM backend: %s", provider.Name)
	}

	return manager
}

// createBackend 创建后端实例
func (m *Manager) createBackend(cfg config.BackendProvider) (LLMBackend, error) {
	switch cfg.Type {
	case "openai":
		return NewOpenAIBackend(cfg, m.logger), nil
	case "vllm":
		return NewVLLMBackend(cfg, m.logger), nil
	default:
		return nil, fmt.Errorf("unsupported backend type: %s", cfg.Type)
	}
}

// RegisterBackend 注册后端
func (m *Manager) RegisterBackend(backend LLMBackend) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.backends[backend.GetName()] = backend
	m.loadBalancer.AddBackend(backend)
}

// Process 处理请求
func (m *Manager) Process(ctx context.Context, req *LLMRequest) (*LLMResponse, error) {
	// 选择后端
	backend, err := m.loadBalancer.SelectBackend(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to select backend: %w", err)
	}

	// 处理请求
	startTime := time.Now()
	resp, err := backend.Process(ctx, req)
	if err != nil {
		m.loadBalancer.ReportError(backend.GetName(), err)
		return nil, fmt.Errorf("backend process failed: %w", err)
	}

	// 记录成功
	duration := time.Since(startTime)
	m.loadBalancer.ReportSuccess(backend.GetName(), duration)

	// 设置响应元数据
	if resp.Metadata == nil {
		resp.Metadata = make(map[string]interface{})
	}
	resp.Metadata["backend"] = backend.GetName()
	resp.Duration = duration

	return resp, nil
}

// GetBackend 获取指定后端
func (m *Manager) GetBackend(name string) (LLMBackend, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	backend, ok := m.backends[name]
	return backend, ok
}

// ListBackends 列出所有后端
func (m *Manager) ListBackends() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.backends))
	for name := range m.backends {
		names = append(names, name)
	}
	return names
}

// HealthCheck 健康检查
func (m *Manager) HealthCheck(ctx context.Context) map[string]error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make(map[string]error)
	for name, backend := range m.backends {
		results[name] = backend.HealthCheck(ctx)
	}
	return results
}

// LoadBalancer 负载均衡器接口
type LoadBalancer interface {
	SelectBackend(ctx context.Context, req *LLMRequest) (LLMBackend, error)
	AddBackend(backend LLMBackend)
	ReportSuccess(backendName string, duration time.Duration)
	ReportError(backendName string, err error)
}

// RoundRobinLoadBalancer 轮询负载均衡器
type RoundRobinLoadBalancer struct {
	backends []LLMBackend
	current  int
	mu       sync.Mutex
	logger   *logrus.Logger
}

// NewLoadBalancer 创建负载均衡器
func NewLoadBalancer(strategy string, logger *logrus.Logger) LoadBalancer {
	switch strategy {
	case "round_robin":
		return &RoundRobinLoadBalancer{
			backends: make([]LLMBackend, 0),
			logger:   logger,
		}
	default:
		logger.Warnf("Unknown load balancer strategy: %s, using round_robin", strategy)
		return &RoundRobinLoadBalancer{
			backends: make([]LLMBackend, 0),
			logger:   logger,
		}
	}
}

// SelectBackend 选择后端
func (lb *RoundRobinLoadBalancer) SelectBackend(ctx context.Context, req *LLMRequest) (LLMBackend, error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if len(lb.backends) == 0 {
		return nil, fmt.Errorf("no available backends")
	}

	backend := lb.backends[lb.current]
	lb.current = (lb.current + 1) % len(lb.backends)
	return backend, nil
}

// AddBackend 添加后端
func (lb *RoundRobinLoadBalancer) AddBackend(backend LLMBackend) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.backends = append(lb.backends, backend)
}

// ReportSuccess 报告成功
func (lb *RoundRobinLoadBalancer) ReportSuccess(backendName string, duration time.Duration) {
	lb.logger.Debugf("Backend %s success, duration: %v", backendName, duration)
}

// ReportError 报告错误
func (lb *RoundRobinLoadBalancer) ReportError(backendName string, err error) {
	lb.logger.Errorf("Backend %s error: %v", backendName, err)
}

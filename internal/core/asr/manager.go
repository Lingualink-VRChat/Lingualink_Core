package asr

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	coreerrors "github.com/Lingualink-VRChat/Lingualink_Core/internal/core/errors"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/logging"
	"github.com/sirupsen/logrus"
)

// LoadBalancer selects ASR backends.
type LoadBalancer interface {
	SelectBackend(ctx context.Context, req *ASRRequest) (Backend, error)
	AddBackend(backend Backend)
	ReportSuccess(backendName string, duration time.Duration)
	ReportError(backendName string, err error)
}

type roundRobinLoadBalancer struct {
	backends []Backend
	current  int
	mu       sync.Mutex
	logger   *logrus.Logger
}

func newLoadBalancer(strategy string, logger *logrus.Logger) LoadBalancer {
	switch strategy {
	case "", "round_robin":
		return &roundRobinLoadBalancer{
			backends: make([]Backend, 0),
			logger:   logger,
		}
	default:
		return &roundRobinLoadBalancer{
			backends: make([]Backend, 0),
			logger:   logger,
		}
	}
}

func (lb *roundRobinLoadBalancer) AddBackend(backend Backend) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.backends = append(lb.backends, backend)
}

func (lb *roundRobinLoadBalancer) SelectBackend(ctx context.Context, req *ASRRequest) (Backend, error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if len(lb.backends) == 0 {
		return nil, fmt.Errorf("no asr backends available")
	}

	backend := lb.backends[lb.current%len(lb.backends)]
	lb.current++
	return backend, nil
}

func (lb *roundRobinLoadBalancer) ReportSuccess(backendName string, duration time.Duration) {
	if lb.logger == nil {
		return
	}
	lb.logger.WithFields(logrus.Fields{
		logging.FieldBackend:  backendName,
		logging.FieldDuration: duration.Milliseconds(),
	}).Debug("ASR backend request succeeded")
}

func (lb *roundRobinLoadBalancer) ReportError(backendName string, err error) {
	if lb.logger == nil {
		return
	}
	lb.logger.WithFields(logrus.Fields{
		logging.FieldBackend: backendName,
	}).WithError(err).Warn("ASR backend request failed")
}

// Manager manages multiple ASR backends and routes requests via load balancing.
type Manager struct {
	backends     map[string]Backend
	loadBalancer LoadBalancer
	logger       *logrus.Logger
	mu           sync.RWMutex
}

func NewManager(cfg config.ASRConfig, logger *logrus.Logger) (*Manager, error) {
	manager := &Manager{
		backends: make(map[string]Backend),
		logger:   logger,
	}

	manager.loadBalancer = newLoadBalancer("round_robin", logger)

	for _, provider := range cfg.Providers {
		var backend Backend
		switch provider.Type {
		case "whisper", "custom":
			backend = NewWhisperBackend(provider, logger)
		case "sensevoice":
			backend = NewWhisperBackend(provider, logger)
		default:
			return nil, coreerrors.NewValidationError(fmt.Sprintf("unsupported asr provider type: %s", provider.Type), nil)
		}

		manager.backends[provider.Name] = backend
		manager.loadBalancer.AddBackend(backend)

		if logger != nil {
			logger.WithFields(logrus.Fields{
				logging.FieldBackend: provider.Name,
				"type":               provider.Type,
				"url":                provider.URL,
			}).Info("Registered ASR backend")
		}
	}

	if len(manager.backends) == 0 {
		return nil, coreerrors.NewValidationError("no asr backends configured", nil)
	}

	return manager, nil
}

func (m *Manager) Transcribe(ctx context.Context, req *ASRRequest) (*ASRResponse, error) {
	backend, err := m.loadBalancer.SelectBackend(ctx, req)
	if err != nil {
		return nil, coreerrors.NewInternalError("failed to select asr backend", err)
	}

	start := time.Now()
	resp, err := backend.Transcribe(ctx, req)
	if err != nil {
		m.loadBalancer.ReportError(backend.GetName(), err)
		return nil, coreerrors.NewInternalError("asr backend transcribe failed", err)
	}

	m.loadBalancer.ReportSuccess(backend.GetName(), time.Since(start))
	return resp, nil
}

func (m *Manager) GetBackend(name string) (Backend, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.backends[name]
	return b, ok
}

func (m *Manager) ListBackends() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.backends))
	for name := range m.backends {
		names = append(names, name)
	}
	return names
}

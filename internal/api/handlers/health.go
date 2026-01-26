// health.go contains health, readiness, and deep-health endpoints.
package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	"github.com/gin-gonic/gin"
)

// HealthCheck 健康检查API
func (h *Handler) HealthCheck(c *gin.Context) {
	// 简单的健康检查
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": getCurrentTimestamp(),
		"version":   h.version,
		"uptime":    time.Since(h.startTime).String(),
	}

	// 检查依赖服务
	if detailed := c.Query("detailed"); detailed == "true" {
		// 这里可以添加更详细的健康检查
		health["services"] = map[string]string{
			"audio_processor": "healthy",
			"llm_manager":     "healthy",
			"prompt_engine":   "healthy",
		}
	}

	c.JSON(http.StatusOK, health)
}

// HealthStatus represents the response body for health endpoints.
type HealthStatus struct {
	Status     string                     `json:"status"`
	Timestamp  int64                      `json:"timestamp"`
	Version    string                     `json:"version"`
	Uptime     string                     `json:"uptime"`
	Components map[string]ComponentHealth `json:"components,omitempty"`
}

// ComponentHealth describes the health of a single component.
type ComponentHealth struct {
	Status  string `json:"status"`
	Latency int64  `json:"latency_ms,omitempty"`
	Message string `json:"message,omitempty"`
}

// LivenessCheck performs a lightweight liveness probe.
func (h *Handler) LivenessCheck(c *gin.Context) {
	c.JSON(http.StatusOK, HealthStatus{
		Status:    "live",
		Timestamp: getCurrentTimestamp(),
		Version:   h.version,
		Uptime:    time.Since(h.startTime).String(),
	})
}

// ReadinessCheck reports whether the service is ready to accept traffic.
func (h *Handler) ReadinessCheck(c *gin.Context) {
	components := make(map[string]ComponentHealth)

	cfgStatus := ComponentHealth{Status: "healthy"}
	if h.config == nil {
		cfgStatus = ComponentHealth{Status: "unhealthy", Message: "config not loaded"}
	} else if err := h.config.Validate(); err != nil {
		cfgStatus = ComponentHealth{Status: "unhealthy", Message: err.Error()}
	}
	components["config"] = cfgStatus

	ready := cfgStatus.Status == "healthy"

	asrHealthy := false
	if h.asrManager == nil {
		components["asr_backends"] = ComponentHealth{Status: "unhealthy", Message: "asr manager not configured"}
		ready = false
	} else {
		names := h.asrManager.ListBackends()
		if len(names) == 0 {
			components["asr_backends"] = ComponentHealth{Status: "unhealthy", Message: "no asr backends configured"}
			ready = false
		} else {
			for _, name := range names {
				backend, ok := h.asrManager.GetBackend(name)
				if !ok || backend == nil {
					continue
				}
				checkCtx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
				start := time.Now()
				err := backend.HealthCheck(checkCtx)
				cancel()
				if err == nil {
					asrHealthy = true
					components["asr_backends"] = ComponentHealth{Status: "healthy", Latency: time.Since(start).Milliseconds()}
					break
				}
			}
			if !asrHealthy {
				components["asr_backends"] = ComponentHealth{Status: "unhealthy", Message: "no healthy asr backend available"}
				ready = false
			}
		}
	}

	backendsHealthy := false

	if h.llmManager == nil {
		components["llm_backends"] = ComponentHealth{Status: "unhealthy", Message: "llm manager not configured"}
		ready = false
	} else {
		names := h.llmManager.ListBackends()
		if len(names) == 0 {
			components["llm_backends"] = ComponentHealth{Status: "unhealthy", Message: "no backends configured"}
			ready = false
		} else {
			for _, name := range names {
				backend, ok := h.llmManager.GetBackend(name)
				if !ok || backend == nil {
					continue
				}
				checkCtx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
				start := time.Now()
				err := backend.HealthCheck(checkCtx)
				cancel()
				if err == nil {
					backendsHealthy = true
					components["llm_backends"] = ComponentHealth{Status: "healthy", Latency: time.Since(start).Milliseconds()}
					break
				}
			}
			if !backendsHealthy {
				components["llm_backends"] = ComponentHealth{Status: "unhealthy", Message: "no healthy backend available"}
				ready = false
			}
		}
	}

	status := "ready"
	code := http.StatusOK
	if !ready {
		status = "not_ready"
		code = http.StatusServiceUnavailable
	}

	c.JSON(code, HealthStatus{
		Status:     status,
		Timestamp:  getCurrentTimestamp(),
		Version:    h.version,
		Uptime:     time.Since(h.startTime).String(),
		Components: components,
	})
}

// DeepHealthCheck performs detailed health checks for critical components.
func (h *Handler) DeepHealthCheck(c *gin.Context) {
	components := make(map[string]ComponentHealth)
	overall := "healthy"

	cfgComponent := ComponentHealth{Status: "healthy"}
	if h.config == nil {
		cfgComponent = ComponentHealth{Status: "unhealthy", Message: "config not loaded"}
	} else if err := h.config.Validate(); err != nil {
		cfgComponent = ComponentHealth{Status: "unhealthy", Message: err.Error()}
	}

	configFileComponent := checkConfigFileReadable()
	if cfgComponent.Status != "unhealthy" && configFileComponent.Status != "healthy" {
		cfgComponent = configFileComponent
	}
	components["config"] = cfgComponent

	if cfgComponent.Status == "unhealthy" {
		overall = "unhealthy"
	} else if cfgComponent.Status == "degraded" && overall == "healthy" {
		overall = "degraded"
	}

	ffmpegComponent := ComponentHealth{Status: "unhealthy", Message: "audio processor not configured"}
	if h.audioProcessor != nil {
		if h.audioProcessor.IsFFmpegAvailable() {
			ffmpegComponent = ComponentHealth{Status: "healthy"}
		} else {
			ffmpegComponent = ComponentHealth{Status: "degraded", Message: "ffmpeg not available"}
		}
	}
	components["ffmpeg"] = ffmpegComponent
	if ffmpegComponent.Status == "unhealthy" {
		overall = "unhealthy"
	} else if ffmpegComponent.Status == "degraded" && overall == "healthy" {
		overall = "degraded"
	}

	if h.asrManager == nil {
		components["asr_manager"] = ComponentHealth{Status: "unhealthy", Message: "asr manager not configured"}
		overall = "unhealthy"
	} else {
		names := h.asrManager.ListBackends()
		if len(names) == 0 {
			components["asr_manager"] = ComponentHealth{Status: "unhealthy", Message: "no asr backends configured"}
			overall = "unhealthy"
		} else {
			anyHealthy := false
			for _, name := range names {
				backend, ok := h.asrManager.GetBackend(name)
				if !ok || backend == nil {
					continue
				}
				checkCtx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
				start := time.Now()
				err := backend.HealthCheck(checkCtx)
				cancel()

				component := ComponentHealth{Latency: time.Since(start).Milliseconds()}
				if err == nil {
					component.Status = "healthy"
					anyHealthy = true
				} else {
					component.Status = "unhealthy"
					component.Message = err.Error()
				}
				components["asr_backend:"+name] = component
			}
			if !anyHealthy {
				overall = "unhealthy"
			}
		}
	}

	if h.llmManager == nil {
		components["llm_manager"] = ComponentHealth{Status: "unhealthy", Message: "llm manager not configured"}
		overall = "unhealthy"
	} else {
		names := h.llmManager.ListBackends()
		if len(names) == 0 {
			components["llm_manager"] = ComponentHealth{Status: "unhealthy", Message: "no backends configured"}
			overall = "unhealthy"
		} else {
			anyHealthy := false
			for _, name := range names {
				backend, ok := h.llmManager.GetBackend(name)
				if !ok || backend == nil {
					continue
				}
				checkCtx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
				start := time.Now()
				err := backend.HealthCheck(checkCtx)
				cancel()

				component := ComponentHealth{Latency: time.Since(start).Milliseconds()}
				if err == nil {
					component.Status = "healthy"
					anyHealthy = true
				} else {
					component.Status = "unhealthy"
					component.Message = err.Error()
				}
				components["llm_backend:"+name] = component
			}
			if !anyHealthy {
				overall = "unhealthy"
			}
		}
	}

	code := http.StatusOK
	if overall == "unhealthy" {
		code = http.StatusServiceUnavailable
	}

	c.JSON(code, HealthStatus{
		Status:     overall,
		Timestamp:  getCurrentTimestamp(),
		Version:    h.version,
		Uptime:     time.Since(h.startTime).String(),
		Components: components,
	})
}

func checkConfigFileReadable() ComponentHealth {
	cfgPath := os.Getenv("LINGUALINK_CONFIG_FILE")
	if cfgPath == "" {
		cfgPath = filepath.Join(config.GetConfigDir(), "config.yaml")
	}

	if _, err := os.Stat(cfgPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ComponentHealth{Status: "healthy", Message: "config file not found, using defaults"}
		}
		return ComponentHealth{Status: "degraded", Message: fmt.Sprintf("config file stat failed: %v", err)}
	}

	f, err := os.Open(cfgPath)
	if err != nil {
		return ComponentHealth{Status: "degraded", Message: fmt.Sprintf("config file open failed: %v", err)}
	}
	_ = f.Close()

	return ComponentHealth{Status: "healthy"}
}

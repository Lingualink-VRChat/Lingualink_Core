package handlers

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/audio"
	coreerrors "github.com/Lingualink-VRChat/Lingualink_Core/internal/core/errors"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/llm"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/processing"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/prompt"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/text"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/auth"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/logging"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/metrics"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// --- 通用请求处理函数 ---
func handleProcessingRequest[Req processing.ProcessableRequest, Resp any](
	c *gin.Context,
	h *Handler,
	service *processing.Service[Req, Resp],
	logicHandler processing.LogicHandler[Req, Resp],
	requestDecoder func(*gin.Context) (Req, error),
) {
	requestIDStr, ok := logging.RequestIDFromContext(c.Request.Context())
	if !ok {
		requestID, _ := c.Get("request_id")
		requestIDStr, _ = requestID.(string)
	}

	if h.statusStore != nil && requestIDStr != "" {
		_ = h.statusStore.Set(requestIDStr, &processing.ProcessingStatus{
			Status:   "processing",
			Progress: 0,
			Message:  "Processing started",
		})
	}

	// 1. 获取认证信息
	identity, exists := c.Get("identity")
	if !exists {
		if h.statusStore != nil && requestIDStr != "" {
			_ = h.statusStore.Set(requestIDStr, &processing.ProcessingStatus{
				Status:   "failed",
				Progress: 0,
				Message:  "authentication required",
			})
		}
		respondError(c, http.StatusUnauthorized, coreerrors.NewAuthError("authentication required", nil))
		return
	}
	userIdentity := identity.(*auth.Identity)

	// 2. 解码和验证请求体
	req, err := requestDecoder(c)
	if err != nil {
		h.logger.WithError(err).Error("Failed to decode request")
		if h.statusStore != nil && requestIDStr != "" {
			_ = h.statusStore.Set(requestIDStr, &processing.ProcessingStatus{
				Status:   "failed",
				Progress: 0,
				Message:  err.Error(),
			})
		}
		respondError(c, http.StatusBadRequest, coreerrors.NewValidationError(err.Error(), err))
		return
	}

	// 3. 调用核心处理服务
	resp, err := service.Process(c.Request.Context(), req, logicHandler)
	if err != nil {
		h.logger.WithError(err).Error("Processing failed")
		if h.statusStore != nil && requestIDStr != "" {
			_ = h.statusStore.Set(requestIDStr, &processing.ProcessingStatus{
				Status:   "failed",
				Progress: 100,
				Message:  err.Error(),
			})
		}
		respondError(c, 0, err)
		return
	}

	if requestIDStr != "" {
		if setter, ok := any(resp).(interface{ SetRequestID(string) }); ok {
			setter.SetRequestID(requestIDStr)
		}
	}

	// 4. 记录指标
	h.metrics.RecordCounter("api.process.success", 1, map[string]string{"user_id": userIdentity.ID})
	switch typed := any(resp).(type) {
	case *audio.ProcessResponse:
		metrics.ObserveAudioProcessingDuration(time.Duration(typed.ProcessingTime * float64(time.Second)))
	}

	if h.statusStore != nil && requestIDStr != "" {
		_ = h.statusStore.Set(requestIDStr, &processing.ProcessingStatus{
			Status:   "completed",
			Progress: 100,
			Message:  "Processing completed",
		})
	}

	// 5. 返回成功响应
	c.JSON(http.StatusOK, resp)
	if releaser, ok := any(resp).(interface{ Release() }); ok {
		releaser.Release()
	}
}

// ErrorResponse defines a standard error payload returned by the API.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details any    `json:"details,omitempty"`
}

func respondError(c *gin.Context, status int, err error) {
	if status <= 0 {
		status = http.StatusInternalServerError
	}

	var appErr *coreerrors.AppError
	if errors.As(err, &appErr) {
		if status <= 0 || status == http.StatusInternalServerError {
			status = statusFromErrorCode(appErr.Code)
		}

		resp := ErrorResponse{
			Error: appErr.Message,
			Code:  string(appErr.Code),
		}
		if appErr.Details != nil {
			resp.Details = appErr.Details
		}

		c.JSON(status, resp)
		return
	}

	c.JSON(status, ErrorResponse{Error: err.Error()})
}

func statusFromErrorCode(code coreerrors.ErrorCode) int {
	switch code {
	case coreerrors.ErrCodeValidation:
		return http.StatusBadRequest
	case coreerrors.ErrCodeAuth:
		return http.StatusUnauthorized
	case coreerrors.ErrCodeLLM:
		return http.StatusBadGateway
	case coreerrors.ErrCodeParsing:
		return http.StatusBadGateway
	case coreerrors.ErrCodeInternal:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// Handler API处理器
type Handler struct {
	config                 *config.Config
	llmManager             *llm.Manager
	startTime              time.Time
	version                string
	audioProcessor         *audio.Processor
	textProcessor          *text.Processor
	audioProcessingService *processing.Service[audio.ProcessRequest, *audio.ProcessResponse]
	textProcessingService  *processing.Service[text.ProcessRequest, *text.ProcessResponse]
	statusStore            processing.StatusStore
	authenticator          *auth.MultiAuthenticator
	logger                 *logrus.Logger
	metrics                metrics.MetricsCollector
}

// NewHandler 创建API处理器
func NewHandler(
	audioProcessor *audio.Processor,
	textProcessor *text.Processor,
	audioProcessingService *processing.Service[audio.ProcessRequest, *audio.ProcessResponse],
	textProcessingService *processing.Service[text.ProcessRequest, *text.ProcessResponse],
	statusStore processing.StatusStore,
	authenticator *auth.MultiAuthenticator,
	logger *logrus.Logger,
	metrics metrics.MetricsCollector,
	cfg *config.Config,
	llmManager *llm.Manager,
) *Handler {
	return &Handler{
		config:                 cfg,
		llmManager:             llmManager,
		startTime:              time.Now(),
		version:                "1.0.0",
		audioProcessor:         audioProcessor,
		textProcessor:          textProcessor,
		audioProcessingService: audioProcessingService,
		textProcessingService:  textProcessingService,
		statusStore:            statusStore,
		authenticator:          authenticator,
		logger:                 logger,
		metrics:                metrics,
	}
}

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

// GetCapabilities 获取能力API
func (h *Handler) GetCapabilities(c *gin.Context) {
	capabilities := h.audioProcessor.GetCapabilities()
	c.JSON(http.StatusOK, capabilities)
}

// GetMetrics 获取指标API
func (h *Handler) GetMetrics(c *gin.Context) {
	// 需要管理员权限
	identity, exists := c.Get("identity")
	if !exists {
		respondError(c, http.StatusUnauthorized, coreerrors.NewAuthError("authentication required", nil))
		return
	}

	userIdentity := identity.(*auth.Identity)
	if userIdentity.Type != auth.IdentityTypeService {
		respondError(c, http.StatusForbidden, coreerrors.NewAuthError("insufficient permissions", nil))
		return
	}

	metrics := h.metrics.GetMetrics()
	c.JSON(http.StatusOK, metrics)
}

// 辅助函数

// getCurrentTimestamp 获取当前时间戳
func getCurrentTimestamp() int64 {
	return time.Now().Unix()
}

// ProcessAudioJSON 处理JSON格式的音频请求
func (h *Handler) ProcessAudioJSON(c *gin.Context) {
	h.logger.Info("Processing JSON audio request")

	decoder := func(c *gin.Context) (audio.ProcessRequest, error) {
		var req struct {
			Audio           string                 `json:"audio"` // base64编码的音频数据
			AudioFormat     string                 `json:"audio_format"`
			Task            prompt.TaskType        `json:"task"`
			SourceLanguage  string                 `json:"source_language,omitempty"`
			TargetLanguages []string               `json:"target_languages"` // 期望短代码
			Options         map[string]interface{} `json:"options,omitempty"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			return audio.ProcessRequest{}, fmt.Errorf("invalid JSON: %w", err)
		}
		decodedLen := base64.StdEncoding.DecodedLen(len(req.Audio))
		if decodedLen > 32*1024*1024 {
			return audio.ProcessRequest{}, fmt.Errorf("audio size exceeds maximum allowed size")
		}
		buf := audio.AcquireAudioBuffer(decodedLen)
		base64Decoder := base64.NewDecoder(base64.StdEncoding, strings.NewReader(req.Audio))
		n := 0
		for n < len(buf) {
			readN, readErr := base64Decoder.Read(buf[n:])
			n += readN
			if readErr == nil {
				continue
			}
			if errors.Is(readErr, io.EOF) {
				break
			}
			audio.ReleaseAudioBuffer(buf)
			return audio.ProcessRequest{}, fmt.Errorf("invalid base64 audio data: %w", readErr)
		}
		audioReq := audio.ProcessRequest{
			Audio:           buf[:n],
			AudioFormat:     req.AudioFormat,
			Task:            req.Task,
			SourceLanguage:  req.SourceLanguage,
			TargetLanguages: req.TargetLanguages,
			Options:         req.Options,
		}
		audioReq.SetCleanup(func() { audio.ReleaseAudioBuffer(buf) })
		return audioReq, nil
	}

	handleProcessingRequest(c, h, h.audioProcessingService, h.audioProcessor, decoder)
}

// GetProcessingStatus 获取处理状态（为将来的异步处理预留）
func (h *Handler) GetProcessingStatus(c *gin.Context) {
	requestID := c.Param("request_id")
	if requestID == "" {
		respondError(c, http.StatusBadRequest, coreerrors.NewValidationError("request_id is required", nil))
		return
	}

	if h.statusStore == nil {
		respondError(c, http.StatusInternalServerError, coreerrors.NewInternalError("status store not configured", nil))
		return
	}

	status, err := h.statusStore.Get(requestID)
	if err != nil {
		if errors.Is(err, processing.ErrStatusNotFound) {
			respondError(c, http.StatusNotFound, coreerrors.NewValidationError("processing status not found", err))
			return
		}
		respondError(c, http.StatusInternalServerError, coreerrors.NewInternalError("failed to get processing status", err))
		return
	}

	c.JSON(http.StatusOK, status)
}

// ListSupportedLanguages 列出支持的语言
func (h *Handler) ListSupportedLanguages(c *gin.Context) {
	languages := h.audioProcessor.GetSupportedLanguages()

	c.JSON(http.StatusOK, gin.H{
		"languages": languages,
		"count":     len(languages),
	})
}

// ProcessText 处理文本翻译请求
func (h *Handler) ProcessText(c *gin.Context) {
	h.logger.Info("Processing text translation request")

	decoder := func(c *gin.Context) (text.ProcessRequest, error) {
		var req text.ProcessRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			return text.ProcessRequest{}, fmt.Errorf("invalid JSON: %w", err)
		}
		return req, nil
	}

	handleProcessingRequest(c, h, h.textProcessingService, h.textProcessor, decoder)
}

// ProcessTextBatch 处理批量文本翻译请求
func (h *Handler) ProcessTextBatch(c *gin.Context) {
	h.logger.Info("Processing batch text translation request")

	requestIDStr, ok := logging.RequestIDFromContext(c.Request.Context())
	if !ok {
		requestID, _ := c.Get("request_id")
		requestIDStr, _ = requestID.(string)
	}

	if h.statusStore != nil && requestIDStr != "" {
		_ = h.statusStore.Set(requestIDStr, &processing.ProcessingStatus{
			Status:   "processing",
			Progress: 0,
			Message:  "Batch processing started",
		})
	}

	identity, exists := c.Get("identity")
	if !exists {
		if h.statusStore != nil && requestIDStr != "" {
			_ = h.statusStore.Set(requestIDStr, &processing.ProcessingStatus{
				Status:   "failed",
				Progress: 0,
				Message:  "authentication required",
			})
		}
		respondError(c, http.StatusUnauthorized, coreerrors.NewAuthError("authentication required", nil))
		return
	}
	userIdentity := identity.(*auth.Identity)

	var batchReq text.BatchProcessRequest
	if err := c.ShouldBindJSON(&batchReq); err != nil {
		if h.statusStore != nil && requestIDStr != "" {
			_ = h.statusStore.Set(requestIDStr, &processing.ProcessingStatus{
				Status:   "failed",
				Progress: 0,
				Message:  err.Error(),
			})
		}
		respondError(c, http.StatusBadRequest, coreerrors.NewValidationError(fmt.Sprintf("invalid JSON: %v", err), err))
		return
	}

	if len(batchReq.Texts) == 0 {
		respondError(c, http.StatusBadRequest, coreerrors.NewValidationError("texts is required", nil))
		return
	}
	if len(batchReq.Texts) > 20 {
		respondError(c, http.StatusBadRequest, coreerrors.NewValidationError("texts exceeds maximum batch size (20)", nil))
		return
	}

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	results := make([]*text.ProcessResponse, len(batchReq.Texts))
	var wg sync.WaitGroup

	errCh := make(chan error, 1)
	sem := make(chan struct{}, 4)

	for i, sourceText := range batchReq.Texts {
		i := i
		sourceText := sourceText
		wg.Add(1)
		go func() {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			resp, err := h.textProcessingService.Process(ctx, text.ProcessRequest{
				Text:            sourceText,
				SourceLanguage:  batchReq.SourceLanguage,
				TargetLanguages: batchReq.TargetLanguages,
				Options:         batchReq.Options,
			}, h.textProcessor)
			if err != nil {
				select {
				case errCh <- fmt.Errorf("item %d: %w", i, err):
				default:
				}
				cancel()
				return
			}

			results[i] = resp
		}()
	}

	wg.Wait()

	select {
	case err := <-errCh:
		for _, r := range results {
			if r != nil {
				r.Release()
			}
		}
		if h.statusStore != nil && requestIDStr != "" {
			_ = h.statusStore.Set(requestIDStr, &processing.ProcessingStatus{
				Status:   "failed",
				Progress: 100,
				Message:  err.Error(),
			})
		}
		respondError(c, 0, err)
		return
	default:
	}

	h.metrics.RecordCounter("api.process_text_batch.success", 1, map[string]string{"user_id": userIdentity.ID})
	if h.statusStore != nil && requestIDStr != "" {
		_ = h.statusStore.Set(requestIDStr, &processing.ProcessingStatus{
			Status:   "completed",
			Progress: 100,
			Message:  "Batch processing completed",
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"request_id": requestIDStr,
		"results":    results,
		"count":      len(results),
	})
	for _, r := range results {
		if r != nil {
			r.Release()
		}
	}
}

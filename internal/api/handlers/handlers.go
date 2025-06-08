package handlers

import (
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/audio"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/processing"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/prompt"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/text"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/auth"
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
	// 1. 获取认证信息
	identity, exists := c.Get("identity")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	userIdentity := identity.(*auth.Identity)

	// 2. 解码和验证请求体
	req, err := requestDecoder(c)
	if err != nil {
		h.logger.WithError(err).Error("Failed to decode request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 3. 调用核心处理服务
	resp, err := service.Process(c.Request.Context(), req, logicHandler)
	if err != nil {
		h.logger.WithError(err).Error("Processing failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 4. 记录指标
	h.metrics.RecordCounter("api.process.success", 1, map[string]string{"user_id": userIdentity.ID})

	// 5. 返回成功响应
	c.JSON(http.StatusOK, resp)
}

// Handler API处理器
type Handler struct {
	audioProcessor         *audio.Processor
	textProcessor          *text.Processor
	audioProcessingService *processing.Service[audio.ProcessRequest, *audio.ProcessResponse]
	textProcessingService  *processing.Service[text.ProcessRequest, *text.ProcessResponse]
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
	authenticator *auth.MultiAuthenticator,
	logger *logrus.Logger,
	metrics metrics.MetricsCollector,
) *Handler {
	return &Handler{
		audioProcessor:         audioProcessor,
		textProcessor:          textProcessor,
		audioProcessingService: audioProcessingService,
		textProcessingService:  textProcessingService,
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
		"version":   "1.0.0",
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	userIdentity := identity.(*auth.Identity)
	if userIdentity.Type != auth.IdentityTypeService {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	metrics := h.metrics.GetMetrics()
	c.JSON(http.StatusOK, metrics)
}

// 辅助函数

// getCurrentTimestamp 获取当前时间戳
func getCurrentTimestamp() int64 {
	return 1704067200 // 示例时间戳，实际使用 time.Now().Unix()
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
		audioData, err := base64.StdEncoding.DecodeString(req.Audio)
		if err != nil {
			return audio.ProcessRequest{}, fmt.Errorf("invalid base64 audio data: %w", err)
		}
		return audio.ProcessRequest{
			Audio:           audioData,
			AudioFormat:     req.AudioFormat,
			Task:            req.Task,
			SourceLanguage:  req.SourceLanguage,
			TargetLanguages: req.TargetLanguages,
			Options:         req.Options,
		}, nil
	}

	handleProcessingRequest(c, h, h.audioProcessingService, h.audioProcessor, decoder)
}

// GetProcessingStatus 获取处理状态（为将来的异步处理预留）
func (h *Handler) GetProcessingStatus(c *gin.Context) {
	requestID := c.Param("request_id")
	if requestID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request_id is required"})
		return
	}

	// TODO: 实现异步处理状态查询
	// 现在返回简单的响应
	status := map[string]interface{}{
		"request_id": requestID,
		"status":     "completed", // pending, processing, completed, failed
		"progress":   100,
		"message":    "Processing completed",
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

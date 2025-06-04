package handlers

import (
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/audio"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/prompt"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/text"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/auth"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/metrics"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// Handler API处理器
type Handler struct {
	audioProcessor *audio.Processor
	textProcessor  *text.Processor
	authenticator  *auth.MultiAuthenticator
	logger         *logrus.Logger
	metrics        metrics.MetricsCollector
}

// NewHandler 创建API处理器
func NewHandler(
	audioProcessor *audio.Processor,
	textProcessor *text.Processor,
	authenticator *auth.MultiAuthenticator,
	logger *logrus.Logger,
	metrics metrics.MetricsCollector,
) *Handler {
	return &Handler{
		audioProcessor: audioProcessor,
		textProcessor:  textProcessor,
		authenticator:  authenticator,
		logger:         logger,
		metrics:        metrics,
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
	// 获取认证信息
	identity, exists := c.Get("identity")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	userIdentity := identity.(*auth.Identity)
	h.logger.WithField("user_id", userIdentity.ID).Info("Processing JSON audio request")

	var req struct {
		Audio           string          `json:"audio"` // base64编码的音频数据
		AudioFormat     string          `json:"audio_format"`
		Task            prompt.TaskType `json:"task"`
		SourceLanguage  string          `json:"source_language,omitempty"`
		TargetLanguages []string        `json:"target_languages"` // 期望短代码
		// 移除Template字段，使用硬编码的默认模板
		// 移除 UserPrompt 字段，改为服务端控制
		Options map[string]interface{} `json:"options,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	// 解码base64音频数据
	audioData, err := base64.StdEncoding.DecodeString(req.Audio)
	if err != nil {
		h.logger.WithError(err).Error("Failed to decode base64 audio data")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid base64 audio data"})
		return
	}

	// 构建处理请求
	processReq := audio.ProcessRequest{
		Audio:           audioData,
		AudioFormat:     req.AudioFormat,
		Task:            req.Task,
		SourceLanguage:  req.SourceLanguage,
		TargetLanguages: req.TargetLanguages,
		// 移除Template和UserPrompt字段
		Options: req.Options,
	}

	// 处理音频
	resp, err := h.audioProcessor.Process(c.Request.Context(), processReq)
	if err != nil {
		h.logger.WithError(err).Error("Audio processing failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 记录成功指标
	h.metrics.RecordCounter("api.process_audio.success", 1, map[string]string{
		"user_id": userIdentity.ID,
		"task":    string(req.Task),
	})

	c.JSON(http.StatusOK, resp)
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
	// 获取认证信息
	identity, exists := c.Get("identity")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	userIdentity := identity.(*auth.Identity)
	h.logger.WithField("user_id", userIdentity.ID).Info("Processing text translation request")

	var req struct {
		Text            string                 `json:"text"`
		SourceLanguage  string                 `json:"source_language,omitempty"`
		TargetLanguages []string               `json:"target_languages"`
		Options         map[string]interface{} `json:"options,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	// 构建处理请求
	processReq := text.ProcessRequest{
		Text:            req.Text,
		SourceLanguage:  req.SourceLanguage,
		TargetLanguages: req.TargetLanguages,
		Options:         req.Options,
	}

	// 处理文本
	resp, err := h.textProcessor.Process(c.Request.Context(), processReq)
	if err != nil {
		h.logger.WithError(err).Error("Text processing failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 记录成功指标
	h.metrics.RecordCounter("api.process_text.success", 1, map[string]string{
		"user_id":      userIdentity.ID,
		"target_count": fmt.Sprintf("%d", len(req.TargetLanguages)),
	})

	c.JSON(http.StatusOK, resp)
}

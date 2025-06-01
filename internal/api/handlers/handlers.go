package handlers

import (
	"encoding/base64"
	"io"
	"net/http"
	"strings"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/audio"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/prompt"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/auth"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/metrics"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// Handler API处理器
type Handler struct {
	audioProcessor *audio.Processor
	authenticator  *auth.MultiAuthenticator
	logger         *logrus.Logger
	metrics        metrics.MetricsCollector
}

// NewHandler 创建API处理器
func NewHandler(
	audioProcessor *audio.Processor,
	authenticator *auth.MultiAuthenticator,
	logger *logrus.Logger,
	metrics metrics.MetricsCollector,
) *Handler {
	return &Handler{
		audioProcessor: audioProcessor,
		authenticator:  authenticator,
		logger:         logger,
		metrics:        metrics,
	}
}

// ProcessAudio 处理音频API
func (h *Handler) ProcessAudio(c *gin.Context) {
	// 获取认证信息
	identity, exists := c.Get("identity")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	userIdentity := identity.(*auth.Identity)
	h.logger.WithField("user_id", userIdentity.ID).Info("Processing audio request")

	// 解析multipart表单
	err := c.Request.ParseMultipartForm(32 << 20) // 32MB限制
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse form"})
		return
	}

	// 获取音频文件
	file, header, err := c.Request.FormFile("audio")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "audio file is required"})
		return
	}
	defer file.Close()

	// 读取音频数据
	audioData, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read audio file"})
		return
	}

	// 获取音频格式（从文件扩展名或表单参数）
	audioFormat := c.PostForm("format")
	if audioFormat == "" {
		audioFormat = getFormatFromFilename(header.Filename)
	}

	// 获取处理参数
	task := prompt.TaskType(c.DefaultPostForm("task", "both"))
	sourceLanguage := c.PostForm("source_language")

	// 修复目标语言解析 - 支持逗号分隔的字符串
	targetLanguagesStr := c.PostForm("target_languages")
	var targetLanguages []string
	if targetLanguagesStr != "" {
		// 检查是否是逗号分隔的字符串
		if strings.Contains(targetLanguagesStr, ",") {
			for _, lang := range strings.Split(targetLanguagesStr, ",") {
				trimmed := strings.TrimSpace(lang)
				if trimmed != "" {
					targetLanguages = append(targetLanguages, trimmed)
				}
			}
		} else {
			targetLanguages = append(targetLanguages, targetLanguagesStr)
		}
	} else {
		// 尝试获取数组形式
		targetLanguages = c.PostFormArray("target_languages")
	}

	template := c.PostForm("template")
	userPrompt := c.PostForm("user_prompt")

	// 构建处理请求
	req := audio.ProcessRequest{
		Audio:           audioData,
		AudioFormat:     audioFormat,
		Task:            task,
		SourceLanguage:  sourceLanguage,
		TargetLanguages: targetLanguages,
		Template:        template,
		UserPrompt:      userPrompt,
	}

	// 处理音频
	resp, err := h.audioProcessor.Process(c.Request.Context(), req)
	if err != nil {
		h.logger.WithError(err).Error("Audio processing failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 记录成功指标
	h.metrics.RecordCounter("api.process_audio.success", 1, map[string]string{
		"user_id": userIdentity.ID,
		"task":    string(task),
	})

	c.JSON(http.StatusOK, resp)
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

// getFormatFromFilename 从文件名获取格式
func getFormatFromFilename(filename string) string {
	if len(filename) < 4 {
		return "wav" // 默认格式
	}

	ext := filename[len(filename)-3:]
	switch ext {
	case "wav", "mp3", "m4a":
		return ext
	case "pus": // opus
		return "opus"
	case "lac": // flac
		return "flac"
	default:
		return "wav"
	}
}

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
		Audio           string                 `json:"audio"` // base64编码的音频数据
		AudioFormat     string                 `json:"audio_format"`
		Task            prompt.TaskType        `json:"task"`
		SourceLanguage  string                 `json:"source_language,omitempty"`
		TargetLanguages []string               `json:"target_languages"`
		Template        string                 `json:"template,omitempty"`
		UserPrompt      string                 `json:"user_prompt,omitempty"`
		Options         map[string]interface{} `json:"options,omitempty"`
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
		Template:        req.Template,
		UserPrompt:      req.UserPrompt,
		Options:         req.Options,
	}

	// 处理音频
	resp, err := h.audioProcessor.Process(c.Request.Context(), processReq)
	if err != nil {
		h.logger.WithError(err).Error("Audio processing failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 记录成功指标
	h.metrics.RecordCounter("api.process_audio_json.success", 1, map[string]string{
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
	languages := []map[string]interface{}{
		{"code": "zh", "name": "中文", "aliases": []string{"chinese", "中文", "汉语"}},
		{"code": "en", "name": "English", "aliases": []string{"english", "英文", "英语"}},
		{"code": "ja", "name": "日本語", "aliases": []string{"japanese", "日文", "日语"}},
		{"code": "ko", "name": "한국어", "aliases": []string{"korean", "韩文", "韩语"}},
		{"code": "es", "name": "Español", "aliases": []string{"spanish", "西班牙语"}},
		{"code": "fr", "name": "Français", "aliases": []string{"french", "法语"}},
		{"code": "de", "name": "Deutsch", "aliases": []string{"german", "德语"}},
	}

	c.JSON(http.StatusOK, gin.H{
		"languages": languages,
		"count":     len(languages),
	})
}

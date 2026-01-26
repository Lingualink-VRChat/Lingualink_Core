// helpers.go contains shared helper functions for handler implementations.
package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/audio"
	coreerrors "github.com/Lingualink-VRChat/Lingualink_Core/internal/core/errors"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/processing"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/auth"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/logging"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/metrics"
	"github.com/gin-gonic/gin"
)

// --- 通用请求处理函数 ---
func handleProcessingRequest[Req processing.ProcessableRequest, Resp any](
	c *gin.Context,
	h *Handler,
	service *processing.Service[Req, Resp],
	logicHandler processing.Handler[Req, Resp],
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

// getCurrentTimestamp 获取当前时间戳
func getCurrentTimestamp() int64 {
	return time.Now().Unix()
}

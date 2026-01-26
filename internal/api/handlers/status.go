// status.go contains status endpoints for async processing.
package handlers

import (
	"errors"
	"net/http"

	coreerrors "github.com/Lingualink-VRChat/Lingualink_Core/internal/core/errors"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/processing"
	"github.com/gin-gonic/gin"
)

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

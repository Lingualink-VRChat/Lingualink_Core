// admin.go contains admin and capability endpoints.
package handlers

import (
	"net/http"

	coreerrors "github.com/Lingualink-VRChat/Lingualink_Core/internal/core/errors"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/auth"
	"github.com/gin-gonic/gin"
)

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

// ListSupportedLanguages 列出支持的语言
func (h *Handler) ListSupportedLanguages(c *gin.Context) {
	languages := h.audioProcessor.GetSupportedLanguages()

	c.JSON(http.StatusOK, gin.H{
		"languages": languages,
		"count":     len(languages),
	})
}

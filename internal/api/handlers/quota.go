package handlers

import (
	"net/http"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/api/middleware"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/auth"
	"github.com/gin-gonic/gin"
)

type quotaStatusResponse struct {
	FreeQuota          bool      `json:"free_quota"`
	SubscriptionActive bool      `json:"subscription_active"`
	Limit              int       `json:"limit"`
	Used               int       `json:"used"`
	Remaining          int       `json:"remaining"`
	WindowSizeSeconds  int64     `json:"window_size_seconds"`
	ResetAt            time.Time `json:"reset_at"`
}

func (h *Handler) GetQuotaStatus(c *gin.Context) {
	rawIdentity, exists := c.Get("identity")
	if !exists || rawIdentity == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	identity, ok := rawIdentity.(*auth.Identity)
	if !ok || identity == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	snapshot := middleware.GetRateLimitSnapshot(identity, time.Now())
	c.JSON(http.StatusOK, quotaStatusResponse{
		FreeQuota:          snapshot.FreeQuota,
		SubscriptionActive: snapshot.SubscriptionActive,
		Limit:              snapshot.Limit,
		Used:               snapshot.Used,
		Remaining:          snapshot.Remaining,
		WindowSizeSeconds:  int64(snapshot.WindowSize.Seconds()),
		ResetAt:            snapshot.ResetAt,
	})
}

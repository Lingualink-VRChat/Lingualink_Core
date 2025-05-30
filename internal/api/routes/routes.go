package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/lingualink/core/internal/api/handlers"
	"github.com/lingualink/core/internal/api/middleware"
	"github.com/lingualink/core/pkg/auth"
)

// RegisterRoutes 注册所有路由
func RegisterRoutes(router *gin.Engine, handler *handlers.Handler, authenticator *auth.MultiAuthenticator) {
	// 公开路由（无需认证）
	public := router.Group("/api/v1")
	{
		public.GET("/health", handler.HealthCheck)
		public.GET("/capabilities", handler.GetCapabilities)
		public.GET("/languages", handler.ListSupportedLanguages)
	}

	// 需要认证的路由
	protected := router.Group("/api/v1")
	protected.Use(middleware.Auth(authenticator))
	{
		// 音频处理
		protected.POST("/process", handler.ProcessAudio)
		protected.POST("/process/json", handler.ProcessAudioJSON)

		// 异步处理状态查询
		protected.GET("/status/:request_id", handler.GetProcessingStatus)
	}

	// 管理员路由（需要服务级别认证）
	admin := router.Group("/api/v1/admin")
	admin.Use(middleware.Auth(authenticator))
	{
		admin.GET("/metrics", handler.GetMetrics)
	}

	// WebSocket路由（预留）
	// ws := router.Group("/ws")
	// {
	// 	ws.GET("/stream", handler.HandleWebSocket)
	// }
}

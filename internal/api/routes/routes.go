package routes

import (
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/api/handlers"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/api/middleware"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/auth"
	"github.com/gin-gonic/gin"
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
		// 音频处理 - 重命名为 process_audio
		protected.POST("/process_audio", handler.ProcessAudioJSON)

		// 文本处理 - 新增 process_text 端点
		protected.POST("/process_text", handler.ProcessText)

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

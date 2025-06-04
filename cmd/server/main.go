package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/api/handlers"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/api/middleware"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/api/routes"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/audio"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/llm"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/prompt"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/text"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/auth"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/metrics"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func main() {
	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		logrus.Fatalf("Failed to load config: %v", err)
	}

	// 设置日志
	logger := setupLogger(cfg.Logging)

	logger.Info("Starting Lingualink Core server...")

	// 初始化组件
	metricsCollector := metrics.NewSimpleMetricsCollector(logger)
	authenticator := auth.NewMultiAuthenticator(cfg.Auth, logger)

	llmManager, err := llm.NewManager(cfg.Backends, logger)
	if err != nil {
		logrus.Fatalf("Failed to create LLM manager: %v", err)
	}

	promptEngine, err := prompt.NewEngine(cfg.Prompt, logger)
	if err != nil {
		logrus.Fatalf("Failed to create prompt engine: %v", err)
	}

	audioProcessor := audio.NewProcessor(llmManager, promptEngine, cfg.Prompt, logger, metricsCollector)
	textProcessor := text.NewProcessor(llmManager, promptEngine, metricsCollector, cfg.Prompt, logger)

	// 注册认证策略
	for _, strategy := range cfg.Auth.Strategies {
		if strategy.Enabled {
			logger.Infof("Registered auth strategy: %s", strategy.Type)
		}
	}

	// 注册LLM后端
	for _, backend := range llmManager.ListBackends() {
		logger.Infof("Registered LLM backend: %s", backend)
	}

	// 设置Gin模式
	if cfg.Server.Mode == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 设置路由
	router := setupRouter(authenticator, audioProcessor, textProcessor, metricsCollector, logger)

	// 创建HTTP服务器
	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler: router,
	}

	// 启动服务器
	go func() {
		logger.Infof("Starting server on port %d", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Failed to start server: %v", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Errorf("Server forced to shutdown: %v", err)
	}

	logger.Info("Server exited")
}

// setupRouter 设置路由
func setupRouter(authenticator *auth.MultiAuthenticator, audioProcessor *audio.Processor, textProcessor *text.Processor, metricsCollector metrics.MetricsCollector, logger *logrus.Logger) *gin.Engine {
	// 创建Gin引擎
	router := gin.New()

	// 添加中间件
	router.Use(middleware.CORS())
	router.Use(middleware.RequestID())
	router.Use(middleware.Logging(logger))
	router.Use(middleware.Metrics(metricsCollector))
	router.Use(middleware.Recovery(logger))

	// 创建处理器
	handler := handlers.NewHandler(audioProcessor, textProcessor, authenticator, logger, metricsCollector)

	// 注册路由
	routes.RegisterRoutes(router, handler, authenticator)

	return router
}

// setupLogger 设置日志器
func setupLogger(cfg config.LoggingConfig) *logrus.Logger {
	logger := logrus.New()

	// 设置日志级别
	level, err := logrus.ParseLevel(cfg.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	// 设置日志格式
	if cfg.Format == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	}

	return logger
}

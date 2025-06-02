package middleware

import (
	"context"
	"strings"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/auth"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/metrics"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// CORS 跨域中间件
func CORS() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-API-Key")
		c.Header("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})
}

// RequestID 请求ID中间件
func RequestID() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		c.Header("X-Request-ID", requestID)
		c.Set("request_id", requestID)
		c.Next()
	})
}

// Logging 日志中间件
func Logging(logger *logrus.Logger) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// 处理请求
		c.Next()

		// 记录日志
		latency := time.Since(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()
		bodySize := c.Writer.Size()

		if raw != "" {
			path = path + "?" + raw
		}

		entry := logger.WithFields(logrus.Fields{
			"status":    statusCode,
			"latency":   latency,
			"client_ip": clientIP,
			"method":    method,
			"path":      path,
			"body_size": bodySize,
		})

		if requestID, exists := c.Get("request_id"); exists {
			entry = entry.WithField("request_id", requestID)
		}

		if len(c.Errors) > 0 {
			entry.Error(c.Errors.String())
		} else {
			entry.Info("Request completed")
		}
	})
}

// Metrics 指标中间件
func Metrics(collector metrics.MetricsCollector) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		start := time.Now()

		// 处理请求
		c.Next()

		// 记录指标
		duration := time.Since(start)
		path := c.FullPath()
		method := c.Request.Method
		status := c.Writer.Status()

		tags := map[string]string{
			"method": method,
			"path":   path,
			"status": statusCodeToGroup(status),
		}

		collector.RecordLatency("http_request_duration", duration, tags)
		collector.RecordCounter("http_requests_total", 1, tags)
	})
}

// Auth 认证中间件
func Auth(authenticator *auth.MultiAuthenticator) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		// 提取认证信息
		credentials := extractCredentials(c)

		// 添加调试日志
		if credentials.APIKey != "" {
			// 只记录API key的前几位，避免泄露完整密钥
			maskedKey := credentials.APIKey
			if len(maskedKey) > 8 {
				maskedKey = maskedKey[:8] + "***"
			}
			logrus.WithFields(logrus.Fields{
				"api_key_prefix": maskedKey,
				"type":          credentials.Type,
				"path":          c.Request.URL.Path,
			}).Debug("Attempting authentication")
		} else {
			logrus.WithFields(logrus.Fields{
				"path": c.Request.URL.Path,
				"type": credentials.Type,
			}).Debug("No API key provided")
		}

		// 执行认证
		identity, err := authenticator.Authenticate(context.Background(), credentials)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"path":  c.Request.URL.Path,
				"error": err.Error(),
				"type":  credentials.Type,
			}).Warn("Authentication failed")
			c.JSON(401, gin.H{"error": "authentication failed"})
			c.Abort()
			return
		}

		// 设置身份信息
		c.Set("identity", identity)
		logrus.WithFields(logrus.Fields{
			"user_id": identity.ID,
			"path":    c.Request.URL.Path,
		}).Debug("Authentication successful")
		c.Next()
	})
}

// OptionalAuth 可选认证中间件（允许匿名访问）
func OptionalAuth(authenticator *auth.MultiAuthenticator) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		// 提取认证信息
		credentials := extractCredentials(c)

		// 如果没有认证信息，使用匿名身份
		if credentials.APIKey == "" && credentials.Token == "" {
			credentials.Type = "anonymous"
		}

		// 执行认证
		identity, err := authenticator.Authenticate(context.Background(), credentials)
		if err != nil {
			// 认证失败时使用匿名身份
			credentials.Type = "anonymous"
			identity, _ = authenticator.Authenticate(context.Background(), credentials)
		}

		// 设置身份信息
		c.Set("identity", identity)
		c.Next()
	})
}

// RateLimit 限流中间件（简单实现）
func RateLimit() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		// TODO: 实现真正的限流逻辑
		// 这里只是一个占位符
		c.Next()
	})
}

// Recovery 恢复中间件（覆盖gin的默认recovery）
func Recovery(logger *logrus.Logger) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				logger.WithFields(logrus.Fields{
					"error":  err,
					"path":   c.Request.URL.Path,
					"method": c.Request.Method,
				}).Error("Panic recovered")

				c.JSON(500, gin.H{"error": "internal server error"})
				c.Abort()
			}
		}()
		c.Next()
	})
}

// 辅助函数

// extractCredentials 提取认证凭据
func extractCredentials(c *gin.Context) auth.Credentials {
	credentials := auth.Credentials{}

	// 从Header提取API Key
	if apiKey := c.GetHeader("X-API-Key"); apiKey != "" {
		credentials.APIKey = apiKey
		credentials.Type = "api_key"
	}

	// 从Authorization Header提取Token
	if authHeader := c.GetHeader("Authorization"); authHeader != "" {
		if strings.HasPrefix(authHeader, "Bearer ") {
			credentials.Token = authHeader
			credentials.Type = "jwt"
		} else if strings.HasPrefix(authHeader, "ApiKey ") {
			credentials.APIKey = strings.TrimPrefix(authHeader, "ApiKey ")
			credentials.Type = "api_key"
		}
	}

	return credentials
}

// generateRequestID 生成请求ID
func generateRequestID() string {
	return "req_" + time.Now().Format("20060102150405") + "_" + randomString(6)
}

// randomString 生成随机字符串
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}

// statusCodeToGroup 将状态码转换为分组
func statusCodeToGroup(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	case code >= 500:
		return "5xx"
	default:
		return "unknown"
	}
}

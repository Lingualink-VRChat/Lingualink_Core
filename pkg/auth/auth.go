package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	"github.com/golang-jwt/jwt/v4"
	"github.com/sirupsen/logrus"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidToken       = errors.New("invalid token")
	ErrExpiredToken       = errors.New("expired token")
	ErrUnauthorized       = errors.New("unauthorized")
)

// Credentials 认证凭据
type Credentials struct {
	Type   string                 `json:"type"`
	Token  string                 `json:"token"`
	APIKey string                 `json:"api_key"`
	Data   map[string]interface{} `json:"data"`
}

// Identity 身份信息
type Identity struct {
	ID          string                 `json:"id"`
	ExternalID  string                 `json:"external_id"`
	Type        IdentityType           `json:"type"`
	Permissions []Permission           `json:"permissions"`
	Metadata    map[string]interface{} `json:"metadata"`
	RateLimits  *RateLimitConfig       `json:"rate_limits"`
}

// IdentityType 身份类型
type IdentityType string

const (
	// IdentityTypeUser represents an end-user identity.
	IdentityTypeUser IdentityType = "user"
	// IdentityTypeService represents a service identity.
	IdentityTypeService IdentityType = "service"
	// IdentityTypeAnonymous represents an anonymous identity.
	IdentityTypeAnonymous IdentityType = "anonymous"
)

// Permission 权限
type Permission string

const (
	// PermissionAudioProcess allows access to audio processing endpoints.
	PermissionAudioProcess Permission = "audio.process"
	// PermissionAudioTranscribe allows access to audio transcription tasks.
	PermissionAudioTranscribe Permission = "audio.transcribe"
	// PermissionAudioTranslate allows access to audio translation tasks.
	PermissionAudioTranslate Permission = "audio.translate"
	// PermissionHealthCheck allows access to health check endpoints.
	PermissionHealthCheck Permission = "health.check"
)

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	RequestsPerMinute int           `json:"requests_per_minute"`
	BurstSize         int           `json:"burst_size"`
	WindowSize        time.Duration `json:"window_size"`
}

// Authenticator 认证器接口
type Authenticator interface {
	Authenticate(ctx context.Context, credentials Credentials) (*Identity, error)
	GetType() string
}

// MultiAuthenticator 多重认证器
type MultiAuthenticator struct {
	authenticators map[string]Authenticator
	logger         *logrus.Logger
}

// NewMultiAuthenticator 创建多重认证器
func NewMultiAuthenticator(cfg config.AuthConfig, logger *logrus.Logger) *MultiAuthenticator {
	ma := &MultiAuthenticator{
		authenticators: make(map[string]Authenticator),
		logger:         logger,
	}

	// 注册认证器
	for _, strategy := range cfg.Strategies {
		if !strategy.Enabled {
			continue
		}

		var auth Authenticator
		switch strategy.Type {
		case "api_key":
			auth = NewAPIKeyAuthenticator(strategy.Config, logger)
		case "jwt":
			auth = NewJWTAuthenticator(strategy.Config, logger)
		case "webhook":
			auth = NewWebhookAuthenticator(strategy.Endpoint, strategy.Config, logger)
		case "anonymous":
			auth = NewAnonymousAuthenticator(strategy.Config, logger)
		default:
			logger.Warnf("Unknown auth strategy: %s", strategy.Type)
			continue
		}

		if auth != nil {
			ma.authenticators[strategy.Type] = auth
			logger.Infof("Registered auth strategy: %s", strategy.Type)
		}
	}

	return ma
}

// Authenticate 认证
func (ma *MultiAuthenticator) Authenticate(ctx context.Context, credentials Credentials) (*Identity, error) {
	if credentials.Type == "" {
		// 自动检测认证类型
		credentials.Type = ma.detectAuthType(credentials)
	}

	authenticator, ok := ma.authenticators[credentials.Type]
	if !ok {
		return nil, fmt.Errorf("unsupported auth type: %s", credentials.Type)
	}

	return authenticator.Authenticate(ctx, credentials)
}

// detectAuthType 自动检测认证类型
func (ma *MultiAuthenticator) detectAuthType(credentials Credentials) string {
	if credentials.APIKey != "" {
		return "api_key"
	}
	if credentials.Token != "" {
		if strings.HasPrefix(credentials.Token, "Bearer ") || len(strings.Split(credentials.Token, ".")) == 3 {
			return "jwt"
		}
	}
	return "anonymous"
}

// APIKeyAuthenticator API密钥认证器
type APIKeyAuthenticator struct {
	keyStore *APIKeyStore
	logger   *logrus.Logger
}

// NewAPIKeyAuthenticator 创建API密钥认证器
func NewAPIKeyAuthenticator(config map[string]interface{}, logger *logrus.Logger) *APIKeyAuthenticator {
	// 创建密钥存储
	keyStore := NewAPIKeyStore(logger)

	// 从JSON文件加载密钥
	keyFilePath := GetKeyFilePath()
	if err := keyStore.LoadFromFile(keyFilePath); err != nil {
		logger.Errorf("Failed to load API keys from %s: %v", keyFilePath, err)
		// 不要因为密钥文件加载失败就停止服务，而是创建默认密钥
	}

	return &APIKeyAuthenticator{
		keyStore: keyStore,
		logger:   logger,
	}
}

// max 辅助函数
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Authenticate 认证
func (auth *APIKeyAuthenticator) Authenticate(ctx context.Context, credentials Credentials) (*Identity, error) {
	if credentials.APIKey == "" {
		auth.logger.Debug("API key is empty")
		return nil, ErrInvalidCredentials
	}

	// 记录尝试使用的API key（掩码处理）
	maskedKey := credentials.APIKey
	if len(maskedKey) > 8 {
		maskedKey = maskedKey[:8] + "***"
	}
	auth.logger.WithFields(map[string]interface{}{
		"provided_key":     maskedKey,
		"valid_keys_count": len(auth.keyStore.Keys),
	}).Debug("Checking API key")

	// 从密钥存储中获取配置
	keyConfig, found := auth.keyStore.GetKey(credentials.APIKey)
	if !found {
		// 记录所有有效的API key（掩码处理）用于调试
		validKeys := auth.keyStore.ListKeys()
		auth.logger.WithFields(map[string]interface{}{
			"provided_key": maskedKey,
			"valid_keys":   validKeys,
		}).Warn("API key not found in valid keys")
		return nil, ErrInvalidCredentials
	}

	// 设置身份类型
	identityType := IdentityTypeUser
	if strings.Contains(keyConfig.ID, "enterprise") || strings.Contains(keyConfig.ID, "backend") {
		identityType = IdentityTypeService
	}

	// 创建限流配置
	var rateLimits *RateLimitConfig
	if keyConfig.RequestsPerMinute > 0 {
		rateLimits = &RateLimitConfig{
			RequestsPerMinute: keyConfig.RequestsPerMinute,
			BurstSize:         max(keyConfig.RequestsPerMinute/5, 10),
			WindowSize:        time.Minute,
		}
	} else if keyConfig.RequestsPerMinute == -1 {
		// -1表示无限制
		rateLimits = &RateLimitConfig{
			RequestsPerMinute: -1,
			BurstSize:         -1,
			WindowSize:        time.Minute,
		}
	}

	return &Identity{
		ID:   keyConfig.ID,
		Type: identityType,
		Permissions: []Permission{
			PermissionAudioProcess,
			PermissionAudioTranscribe,
			PermissionAudioTranslate,
			PermissionHealthCheck,
		},
		Metadata:   keyConfig.Metadata,
		RateLimits: rateLimits,
	}, nil
}

// GetType 获取认证器类型
func (auth *APIKeyAuthenticator) GetType() string {
	return "api_key"
}

// JWTAuthenticator JWT认证器
type JWTAuthenticator struct {
	secret []byte
	logger *logrus.Logger
}

// NewJWTAuthenticator 创建JWT认证器
func NewJWTAuthenticator(config map[string]interface{}, logger *logrus.Logger) *JWTAuthenticator {
	secret := "default-secret-key"
	if s, ok := config["secret"].(string); ok {
		secret = s
	}
	return &JWTAuthenticator{
		secret: []byte(secret),
		logger: logger,
	}
}

// Authenticate 认证
func (auth *JWTAuthenticator) Authenticate(ctx context.Context, credentials Credentials) (*Identity, error) {
	tokenString := credentials.Token
	if strings.HasPrefix(tokenString, "Bearer ") {
		tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return auth.secret, nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return &Identity{
			ID:          getString(claims, "sub", ""),
			ExternalID:  getString(claims, "user_id", ""),
			Type:        IdentityType(getString(claims, "type", "user")),
			Permissions: parsePermissions(claims["permissions"]),
			Metadata:    claims,
		}, nil
	}

	return nil, ErrInvalidToken
}

// GetType 获取认证器类型
func (auth *JWTAuthenticator) GetType() string {
	return "jwt"
}

// WebhookAuthenticator Webhook认证器
type WebhookAuthenticator struct {
	endpoint string
	config   map[string]interface{}
	logger   *logrus.Logger
}

// NewWebhookAuthenticator 创建Webhook认证器
func NewWebhookAuthenticator(endpoint string, config map[string]interface{}, logger *logrus.Logger) *WebhookAuthenticator {
	return &WebhookAuthenticator{
		endpoint: endpoint,
		config:   config,
		logger:   logger,
	}
}

// Authenticate 认证
func (auth *WebhookAuthenticator) Authenticate(ctx context.Context, credentials Credentials) (*Identity, error) {
	if auth.endpoint == "" {
		return nil, fmt.Errorf("webhook endpoint is required")
	}

	timeout := 5 * time.Second
	if auth.config != nil {
		if v, ok := auth.config["timeout_seconds"]; ok {
			switch n := v.(type) {
			case int:
				timeout = time.Duration(n) * time.Second
			case int64:
				timeout = time.Duration(n) * time.Second
			case float64:
				timeout = time.Duration(n * float64(time.Second))
			}
		}
		if v, ok := auth.config["timeout_ms"]; ok {
			switch n := v.(type) {
			case int:
				timeout = time.Duration(n) * time.Millisecond
			case int64:
				timeout = time.Duration(n) * time.Millisecond
			case float64:
				timeout = time.Duration(n * float64(time.Millisecond))
			}
		}
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	reqBody, err := json.Marshal(credentials)
	if err != nil {
		return nil, fmt.Errorf("marshal webhook credentials: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, auth.endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Optional custom headers from config.headers
	if auth.config != nil {
		if headers, ok := auth.config["headers"].(map[string]interface{}); ok {
			for k, v := range headers {
				if s, ok := v.(string); ok && s != "" {
					req.Header.Set(k, s)
				}
			}
		}
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send webhook request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read webhook response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		auth.logger.WithFields(logrus.Fields{
			"status": resp.StatusCode,
		}).Warn("Webhook authentication denied")
		return nil, ErrUnauthorized
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		auth.logger.WithFields(logrus.Fields{
			"status": resp.StatusCode,
			"body":   string(respBody),
		}).Warn("Webhook authentication failed")
		return nil, fmt.Errorf("webhook auth failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	type webhookRateLimits struct {
		RequestsPerMinute int         `json:"requests_per_minute"`
		BurstSize         int         `json:"burst_size"`
		WindowSize        interface{} `json:"window_size"`
	}
	type webhookIdentity struct {
		ID          string                 `json:"id"`
		ExternalID  string                 `json:"external_id"`
		Type        IdentityType           `json:"type"`
		Permissions []Permission           `json:"permissions"`
		Metadata    map[string]interface{} `json:"metadata"`
		RateLimits  *webhookRateLimits     `json:"rate_limits"`
	}

	var parsed webhookIdentity
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal webhook identity: %w", err)
	}

	identity := &Identity{
		ID:          parsed.ID,
		ExternalID:  parsed.ExternalID,
		Type:        parsed.Type,
		Permissions: parsed.Permissions,
		Metadata:    parsed.Metadata,
	}
	if identity.ID == "" {
		return nil, fmt.Errorf("webhook response missing identity id")
	}
	if identity.Type == "" {
		identity.Type = IdentityTypeUser
	}

	if parsed.RateLimits != nil {
		window := time.Minute
		switch v := parsed.RateLimits.WindowSize.(type) {
		case string:
			if d, err := time.ParseDuration(v); err == nil {
				window = d
			}
		case float64:
			// interpret as seconds
			window = time.Duration(v * float64(time.Second))
		case int:
			window = time.Duration(v) * time.Second
		case int64:
			window = time.Duration(v) * time.Second
		}

		identity.RateLimits = &RateLimitConfig{
			RequestsPerMinute: parsed.RateLimits.RequestsPerMinute,
			BurstSize:         parsed.RateLimits.BurstSize,
			WindowSize:        window,
		}
	}

	return identity, nil
}

// GetType 获取认证器类型
func (auth *WebhookAuthenticator) GetType() string {
	return "webhook"
}

// AnonymousAuthenticator 匿名认证器
type AnonymousAuthenticator struct {
	config map[string]interface{}
	logger *logrus.Logger
}

// NewAnonymousAuthenticator 创建匿名认证器
func NewAnonymousAuthenticator(config map[string]interface{}, logger *logrus.Logger) *AnonymousAuthenticator {
	return &AnonymousAuthenticator{
		config: config,
		logger: logger,
	}
}

// Authenticate 认证
func (auth *AnonymousAuthenticator) Authenticate(ctx context.Context, credentials Credentials) (*Identity, error) {
	return &Identity{
		ID:   "anonymous",
		Type: IdentityTypeAnonymous,
		Permissions: []Permission{
			PermissionHealthCheck,
		},
		RateLimits: &RateLimitConfig{
			RequestsPerMinute: 10,
			BurstSize:         5,
			WindowSize:        time.Minute,
		},
	}, nil
}

// GetType 获取认证器类型
func (auth *AnonymousAuthenticator) GetType() string {
	return "anonymous"
}

// 辅助函数
func getString(claims jwt.MapClaims, key, defaultValue string) string {
	if v, ok := claims[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultValue
}

func parsePermissions(perms interface{}) []Permission {
	var permissions []Permission
	if permSlice, ok := perms.([]interface{}); ok {
		for _, p := range permSlice {
			if perm, ok := p.(string); ok {
				permissions = append(permissions, Permission(perm))
			}
		}
	}
	return permissions
}

// api_key.go contains API key authentication.
package auth

import (
	"context"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

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

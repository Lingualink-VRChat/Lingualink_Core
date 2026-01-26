// multi.go contains the multi-authenticator composition and auth type detection.
package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	"github.com/sirupsen/logrus"
)

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
	if credentials.Type == "api_key" && credentials.APIKey == "" && credentials.Token != "" {
		token := strings.TrimSpace(credentials.Token)
		if strings.HasPrefix(token, "ApiKey ") {
			token = strings.TrimSpace(strings.TrimPrefix(token, "ApiKey "))
		}
		if strings.HasPrefix(token, "Bearer ") {
			token = strings.TrimSpace(strings.TrimPrefix(token, "Bearer "))
		}
		if token != "" {
			credentials.APIKey = token
		}
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
		token := strings.TrimSpace(credentials.Token)
		if strings.HasPrefix(token, "Bearer ") {
			token = strings.TrimSpace(strings.TrimPrefix(token, "Bearer "))
		}
		if token == "" {
			return "anonymous"
		}

		looksJWT := looksLikeJWT(token)
		if looksJWT {
			if _, ok := ma.authenticators["jwt"]; ok {
				return "jwt"
			}
			// JWT looks likely, but if JWT strategy isn't enabled, fall back to api_key when possible.
			if _, ok := ma.authenticators["api_key"]; ok {
				return "api_key"
			}
			return "jwt"
		}

		// Bearer tokens that don't look like JWTs are treated as API keys by default.
		if _, ok := ma.authenticators["api_key"]; ok {
			return "api_key"
		}
		if _, ok := ma.authenticators["jwt"]; ok {
			return "jwt"
		}
	}
	return "anonymous"
}

func looksLikeJWT(token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return false
		}
	}
	return true
}

// anonymous.go contains anonymous authentication.
package auth

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
)

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

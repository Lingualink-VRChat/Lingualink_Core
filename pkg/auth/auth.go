// auth.go defines shared auth types and interfaces.
package auth

import (
	"context"
	"errors"
	"time"
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

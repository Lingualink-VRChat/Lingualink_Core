// webhook.go contains webhook authentication.
package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

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

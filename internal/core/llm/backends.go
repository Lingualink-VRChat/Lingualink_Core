package llm

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	"github.com/sirupsen/logrus"
)

// OpenAIBackend OpenAI后端实现
type OpenAIBackend struct {
	*BaseOpenAICompatibleBackend
}

// NewOpenAIBackend 创建OpenAI后端
func NewOpenAIBackend(cfg config.BackendProvider, logger *logrus.Logger) *OpenAIBackend {
	return &OpenAIBackend{
		BaseOpenAICompatibleBackend: NewBaseOpenAICompatibleBackend(
			cfg.Name,
			cfg.URL,
			cfg.APIKey,
			cfg.Model,
			30*time.Second,
			cfg.Parameters,
			logger,
		),
	}
}

// HealthCheck 健康检查
func (b *OpenAIBackend) HealthCheck(ctx context.Context) error {
	// 简单的健康检查请求
	req, err := http.NewRequestWithContext(ctx, "GET", b.baseURL+"/models", nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+b.apiKey)

	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: status %d", resp.StatusCode)
	}

	return nil
}

// GetCapabilities 获取能力
func (b *OpenAIBackend) GetCapabilities() Capabilities {
	return Capabilities{
		SupportsAudio:      true,
		SupportedFormats:   []string{"wav", "mp3", "m4a", "opus"},
		MaxAudioSize:       25 * 1024 * 1024, // 25MB
		SupportsStreaming:  true,
		SupportedLanguages: []string{"en", "zh", "ja", "ko", "es", "fr", "de", "it", "pt", "ru"},
	}
}

// VLLMBackend VLLM后端实现
type VLLMBackend struct {
	*BaseOpenAICompatibleBackend
}

// NewVLLMBackend 创建VLLM后端
func NewVLLMBackend(cfg config.BackendProvider, logger *logrus.Logger) *VLLMBackend {
	return &VLLMBackend{
		BaseOpenAICompatibleBackend: NewBaseOpenAICompatibleBackend(
			cfg.Name,
			cfg.URL,
			cfg.APIKey,
			cfg.Model,
			60*time.Second,
			cfg.Parameters,
			logger,
		),
	}
}

// HealthCheck 健康检查
func (b *VLLMBackend) HealthCheck(ctx context.Context) error {
	// 健康检查请求
	req, err := http.NewRequestWithContext(ctx, "GET", b.baseURL+"/health", nil)
	if err != nil {
		return err
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: status %d", resp.StatusCode)
	}

	return nil
}

// GetCapabilities 获取能力
func (b *VLLMBackend) GetCapabilities() Capabilities {
	return Capabilities{
		SupportsAudio:      true, // 支持音频处理
		SupportedFormats:   []string{"wav", "mp3", "opus", "m4a"},
		MaxAudioSize:       25 * 1024 * 1024, // 25MB
		SupportsStreaming:  true,
		SupportedLanguages: []string{"en", "zh", "ja", "ko", "es", "fr", "de"},
	}
}

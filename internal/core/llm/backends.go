package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	"github.com/sirupsen/logrus"
)

// OpenAIBackend OpenAI后端实现
type OpenAIBackend struct {
	name    string
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
	logger  *logrus.Logger
}

// NewOpenAIBackend 创建OpenAI后端
func NewOpenAIBackend(cfg config.BackendProvider, logger *logrus.Logger) *OpenAIBackend {
	return &OpenAIBackend{
		name:    cfg.Name,
		baseURL: cfg.URL,
		apiKey:  cfg.APIKey,
		model:   cfg.Model,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// Process 处理请求
func (b *OpenAIBackend) Process(ctx context.Context, req *LLMRequest) (*LLMResponse, error) {
	// 构建OpenAI API请求
	apiReq := map[string]interface{}{
		"model": b.model,
		"messages": []map[string]interface{}{
			{"role": "system", "content": req.SystemPrompt},
			{"role": "user", "content": req.UserPrompt},
		},
	}

	// 如果有音频，构建特殊格式
	if len(req.Audio) > 0 {
		// OpenAI的音频API格式（这里简化处理）
		apiReq["audio"] = map[string]interface{}{
			"format": req.AudioFormat,
			"data":   req.Audio,
		}
	}

	reqBody, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// 发送HTTP请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", b.baseURL+"/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+b.apiKey)

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s", string(respBody))
	}

	// 解析响应
	var apiResp map[string]interface{}
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	// 提取内容
	content := ""
	if choices, ok := apiResp["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if message, ok := choice["message"].(map[string]interface{}); ok {
				if c, ok := message["content"].(string); ok {
					content = c
				}
			}
		}
	}

	// 提取使用信息
	promptTokens := 0
	totalTokens := 0
	if usage, ok := apiResp["usage"].(map[string]interface{}); ok {
		if pt, ok := usage["prompt_tokens"].(float64); ok {
			promptTokens = int(pt)
		}
		if tt, ok := usage["total_tokens"].(float64); ok {
			totalTokens = int(tt)
		}
	}

	return &LLMResponse{
		Content:      content,
		Model:        b.model,
		PromptTokens: promptTokens,
		TotalTokens:  totalTokens,
		Metadata: map[string]interface{}{
			"backend":      b.name,
			"raw_response": apiResp,
		},
	}, nil
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

// GetName 获取名称
func (b *OpenAIBackend) GetName() string {
	return b.name
}

// VLLMBackend VLLM后端实现
type VLLMBackend struct {
	name    string
	baseURL string
	model   string
	client  *http.Client
	logger  *logrus.Logger
}

// NewVLLMBackend 创建VLLM后端
func NewVLLMBackend(cfg config.BackendProvider, logger *logrus.Logger) *VLLMBackend {
	return &VLLMBackend{
		name:    cfg.Name,
		baseURL: cfg.URL,
		model:   cfg.Model,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		logger: logger,
	}
}

// Process 处理请求
func (b *VLLMBackend) Process(ctx context.Context, req *LLMRequest) (*LLMResponse, error) {
	// 构建VLLM API请求（假设支持OpenAI兼容的API）
	apiReq := map[string]interface{}{
		"model": b.model,
		"messages": []map[string]interface{}{
			{"role": "system", "content": req.SystemPrompt},
			{"role": "user", "content": req.UserPrompt},
		},
		"temperature": 0.7,
		"max_tokens":  2048,
	}

	// 如果有音频，构建特殊处理
	if len(req.Audio) > 0 {
		// VLLM可能需要特殊的音频处理
		b.logger.Warnf("Audio processing for VLLM backend not fully implemented")
		// 这里可以添加音频预处理逻辑
	}

	reqBody, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// 发送HTTP请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", b.baseURL+"/v1/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s", string(respBody))
	}

	// 解析响应
	var apiResp map[string]interface{}
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	// 提取内容
	content := ""
	if choices, ok := apiResp["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if message, ok := choice["message"].(map[string]interface{}); ok {
				if c, ok := message["content"].(string); ok {
					content = c
				}
			}
		}
	}

	// 提取使用信息
	promptTokens := 0
	totalTokens := 0
	if usage, ok := apiResp["usage"].(map[string]interface{}); ok {
		if pt, ok := usage["prompt_tokens"].(float64); ok {
			promptTokens = int(pt)
		}
		if tt, ok := usage["total_tokens"].(float64); ok {
			totalTokens = int(tt)
		}
	}

	return &LLMResponse{
		Content:      content,
		Model:        b.model,
		PromptTokens: promptTokens,
		TotalTokens:  totalTokens,
		Metadata: map[string]interface{}{
			"backend":      b.name,
			"raw_response": apiResp,
		},
	}, nil
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
		SupportsAudio:      false, // VLLM通常不直接支持音频
		SupportedFormats:   []string{},
		MaxAudioSize:       0,
		SupportsStreaming:  true,
		SupportedLanguages: []string{"en", "zh", "ja", "ko"},
	}
}

// GetName 获取名称
func (b *VLLMBackend) GetName() string {
	return b.name
}

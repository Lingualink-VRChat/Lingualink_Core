package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// BaseOpenAICompatibleBackend 基础OpenAI兼容后端
type BaseOpenAICompatibleBackend struct {
	name    string
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
	logger  *logrus.Logger
}

// NewBaseOpenAICompatibleBackend 创建基础后端
func NewBaseOpenAICompatibleBackend(name, baseURL, apiKey, model string, timeout time.Duration, logger *logrus.Logger) *BaseOpenAICompatibleBackend {
	return &BaseOpenAICompatibleBackend{
		name:    name,
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		client: &http.Client{
			Timeout: timeout,
		},
		logger: logger,
	}
}

// Process 处理请求 - 通用的OpenAI兼容实现
func (b *BaseOpenAICompatibleBackend) Process(ctx context.Context, req *LLMRequest) (*LLMResponse, error) {
	// 构建消息
	messages := b.buildMessages(req)

	// 构建API请求
	apiReq := map[string]interface{}{
		"model":    b.model,
		"messages": messages,
	}

	// 添加默认参数
	b.addDefaultParameters(apiReq)

	// 序列化请求
	reqBody, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// 创建HTTP请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", b.baseURL+"/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// 设置请求头
	b.setRequestHeaders(httpReq)

	// 发送请求
	resp, err := b.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		b.logger.WithFields(logrus.Fields{
			"status_code": resp.StatusCode,
			"response":    string(respBody),
			"backend":     b.name,
		}).Error("API error")
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// 解析响应
	var apiResp map[string]interface{}
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	// 提取内容和使用信息
	content := b.extractContent(apiResp)
	promptTokens, totalTokens := b.extractUsage(apiResp)

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

// buildMessages 构建消息数组
func (b *BaseOpenAICompatibleBackend) buildMessages(req *LLMRequest) []map[string]interface{} {
	var messages []map[string]interface{}

	// 添加系统消息
	if req.SystemPrompt != "" {
		messages = append(messages, map[string]interface{}{
			"role":    "system",
			"content": req.SystemPrompt,
		})
	}

	// 添加用户消息
	if len(req.Audio) > 0 {
		// 构建带音频的用户消息
		messages = append(messages, b.buildAudioMessage(req))
		
		b.logger.WithFields(logrus.Fields{
			"audio_format": req.AudioFormat,
			"audio_size":   len(req.Audio),
			"backend":      b.name,
		}).Info("Sending audio request")
	} else {
		// 纯文本消息
		messages = append(messages, map[string]interface{}{
			"role":    "user",
			"content": req.UserPrompt,
		})
	}

	return messages
}

// buildAudioMessage 构建音频消息 - 可被子类重写
func (b *BaseOpenAICompatibleBackend) buildAudioMessage(req *LLMRequest) map[string]interface{} {
	// 默认使用OpenAI格式的多模态消息
	audioBase64 := base64.StdEncoding.EncodeToString(req.Audio)

	userContent := []interface{}{
		map[string]interface{}{
			"type": "text",
			"text": req.UserPrompt,
		},
		map[string]interface{}{
			"type": "input_audio",
			"input_audio": map[string]interface{}{
				"data":   audioBase64,
				"format": req.AudioFormat,
			},
		},
	}

	return map[string]interface{}{
		"role":    "user",
		"content": userContent,
	}
}

// addDefaultParameters 添加默认参数 - 可被子类重写
func (b *BaseOpenAICompatibleBackend) addDefaultParameters(apiReq map[string]interface{}) {
	// 默认参数
	apiReq["temperature"] = 0.0
	apiReq["max_tokens"] = 100
}

// setRequestHeaders 设置请求头 - 可被子类重写
func (b *BaseOpenAICompatibleBackend) setRequestHeaders(httpReq *http.Request) {
	httpReq.Header.Set("Content-Type", "application/json")
	if b.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+b.apiKey)
	}
}

// extractContent 提取响应内容
func (b *BaseOpenAICompatibleBackend) extractContent(apiResp map[string]interface{}) string {
	if choices, ok := apiResp["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if message, ok := choice["message"].(map[string]interface{}); ok {
				if content, ok := message["content"].(string); ok {
					return content
				}
			}
		}
	}
	return ""
}

// extractUsage 提取使用信息
func (b *BaseOpenAICompatibleBackend) extractUsage(apiResp map[string]interface{}) (int, int) {
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

	return promptTokens, totalTokens
}

// GetName 获取名称
func (b *BaseOpenAICompatibleBackend) GetName() string {
	return b.name
}

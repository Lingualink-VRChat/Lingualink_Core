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

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	"github.com/sirupsen/logrus"
)

// BaseOpenAICompatibleBackend 基础OpenAI兼容后端
type BaseOpenAICompatibleBackend struct {
	name       string
	baseURL    string
	apiKey     string
	model      string
	client     *http.Client
	logger     *logrus.Logger
	parameters config.LLMParameters
}

// NewBaseOpenAICompatibleBackend 创建基础后端
func NewBaseOpenAICompatibleBackend(name, baseURL, apiKey, model string, timeout time.Duration, parameters config.LLMParameters, logger *logrus.Logger) *BaseOpenAICompatibleBackend {
	return &BaseOpenAICompatibleBackend{
		name:       name,
		baseURL:    baseURL,
		apiKey:     apiKey,
		model:      model,
		parameters: parameters,
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

	// 添加请求中的自定义参数（会覆盖默认参数）
	b.addRequestParameters(apiReq, req)

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
	// 从配置中读取参数，如果配置中没有则使用默认值
	if b.parameters.Temperature != nil {
		apiReq["temperature"] = *b.parameters.Temperature
	} else {
		apiReq["temperature"] = 0.7 // 默认值
	}

	if b.parameters.MaxTokens != nil {
		apiReq["max_tokens"] = *b.parameters.MaxTokens
	} else {
		apiReq["max_tokens"] = 1000 // 默认值
	}

	if b.parameters.TopP != nil {
		apiReq["top_p"] = *b.parameters.TopP
	}

	if b.parameters.TopK != nil {
		apiReq["top_k"] = *b.parameters.TopK
	}

	if b.parameters.RepetitionPenalty != nil {
		apiReq["repetition_penalty"] = *b.parameters.RepetitionPenalty
	}

	if b.parameters.FrequencyPenalty != nil {
		apiReq["frequency_penalty"] = *b.parameters.FrequencyPenalty
	}

	if b.parameters.PresencePenalty != nil {
		apiReq["presence_penalty"] = *b.parameters.PresencePenalty
	}

	if len(b.parameters.Stop) > 0 {
		apiReq["stop"] = b.parameters.Stop
	}

	if b.parameters.Seed != nil {
		apiReq["seed"] = *b.parameters.Seed
	}

	if b.parameters.Stream != nil {
		apiReq["stream"] = *b.parameters.Stream
	} else {
		apiReq["stream"] = false // 默认不使用流式输出
	}
}

// addRequestParameters 添加请求中的自定义参数
func (b *BaseOpenAICompatibleBackend) addRequestParameters(apiReq map[string]interface{}, req *LLMRequest) {
	if req.Options == nil {
		return
	}

	// 支持的参数列表
	supportedParams := []string{
		"temperature", "max_tokens", "top_p", "top_k",
		"repetition_penalty", "frequency_penalty", "presence_penalty",
		"stop", "seed", "stream",
	}

	// 从请求选项中添加参数（会覆盖配置和默认值）
	for _, param := range supportedParams {
		if value, exists := req.Options[param]; exists {
			apiReq[param] = value
		}
	}
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

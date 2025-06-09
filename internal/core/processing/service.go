package processing

import (
	"context"
	"fmt"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/llm"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/prompt"
	"github.com/sirupsen/logrus"
)

// ProcessableRequest 定义了所有请求类型必须满足的最小契约
type ProcessableRequest interface {
	GetTargetLanguages() []string
}

// LogicHandler 定义了特定处理器（如音频或文本）需要提供的逻辑
type LogicHandler[T ProcessableRequest, R any] interface {
	Validate(req T) error
	BuildLLMRequest(ctx context.Context, req T) (*llm.LLMRequest, *prompt.OutputRules, error)
	BuildSuccessResponse(llmResp *llm.LLMResponse, parsedResp *prompt.ParsedResponse, req T) R
	ApplyFallback(response R, rawContent string, outputRules *prompt.OutputRules)
}

// Service 通用处理服务
type Service[T ProcessableRequest, R any] struct {
	llmManager   *llm.Manager
	promptEngine *prompt.Engine
	logger       *logrus.Logger
}

// NewService 创建新的处理服务
func NewService[T ProcessableRequest, R any](llmManager *llm.Manager, promptEngine *prompt.Engine, logger *logrus.Logger) *Service[T, R] {
	return &Service[T, R]{
		llmManager:   llmManager,
		promptEngine: promptEngine,
		logger:       logger,
	}
}

// Process 执行通用处理流程
func (s *Service[T, R]) Process(ctx context.Context, req T, handler LogicHandler[T, R]) (R, error) {
	startTime := time.Now()
	var emptyResponse R

	// 1. 验证请求
	if err := handler.Validate(req); err != nil {
		return emptyResponse, fmt.Errorf("validation failed: %w", err)
	}

	// 2. 构建LLM请求
	llmReq, outputRules, err := handler.BuildLLMRequest(ctx, req)
	if err != nil {
		return emptyResponse, fmt.Errorf("failed to build LLM request: %w", err)
	}

	// 3. 调用LLM
	llmResp, err := s.llmManager.Process(ctx, llmReq)
	if err != nil {
		return emptyResponse, fmt.Errorf("llm process failed: %w", err)
	}

	// 4. 解析响应
	parsed, err := s.promptEngine.ParseResponse(llmResp.Content, *outputRules)
	if err != nil {
		s.logger.WithError(err).Warn("Failed to parse LLM response, will apply fallback")
	}

	// 5. 构建成功响应
	response := handler.BuildSuccessResponse(llmResp, parsed, req)

	// 6. 应用回退逻辑
	handler.ApplyFallback(response, llmResp.Content, outputRules)

	s.logger.WithFields(logrus.Fields{
		"processing_time": time.Since(startTime).Seconds(),
		"target_count":    len(req.GetTargetLanguages()),
	}).Debug("Processing completed")

	return response, nil
}

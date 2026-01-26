package processing

import (
	"context"
	"errors"
	"time"

	coreerrors "github.com/Lingualink-VRChat/Lingualink_Core/internal/core/errors"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/llm"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/prompt"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/logging"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/metrics"
	"github.com/sirupsen/logrus"
)

// ProcessableRequest 定义了所有请求类型必须满足的最小契约
type ProcessableRequest interface {
	GetTargetLanguages() []string
}

// LogicHandler 定义了特定处理器（如音频或文本）需要提供的逻辑
type LogicHandler[T ProcessableRequest, R any] interface {
	Validate(req T) error
	BuildLLMRequest(ctx context.Context, req T) (*llm.LLMRequest, error)
	BuildSuccessResponse(llmResp *llm.LLMResponse, parsedResp *prompt.ParsedResponse, req T) R
}

// processingTimeSetter 可选接口：如果响应类型支持，将由 Service 写入处理耗时（秒）。
type processingTimeSetter interface {
	SetProcessingTime(seconds float64)
}

type requestCleaner interface {
	Cleanup()
}

type responseCacheHandler[T ProcessableRequest, R any] interface {
	TryGetCachedResponse(ctx context.Context, req T) (R, bool, error)
	StoreCachedResponse(ctx context.Context, req T, resp R) error
}

type directProcessor[T ProcessableRequest, R any] interface {
	ProcessDirect(ctx context.Context, req T) (R, bool, error)
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
	if cleaner, ok := any(req).(requestCleaner); ok {
		defer cleaner.Cleanup()
	}

	// 1. 验证请求
	if err := handler.Validate(req); err != nil {
		return emptyResponse, ensureAppError(err, coreerrors.ErrCodeValidation, "")
	}

	// 1.5 缓存命中（可选）
	if cacher, ok := any(handler).(responseCacheHandler[T, R]); ok {
		cached, hit, err := cacher.TryGetCachedResponse(ctx, req)
		if err != nil {
			return emptyResponse, ensureAppError(err, coreerrors.ErrCodeInternal, "cache lookup failed")
		}
		if hit {
			processingTimeSec := time.Since(startTime).Seconds()
			if setter, ok := any(cached).(processingTimeSetter); ok {
				setter.SetProcessingTime(processingTimeSec)
			}

			fields := logrus.Fields{
				logging.FieldDuration: time.Since(startTime).Milliseconds(),
				"cache_hit":           true,
			}
			if requestID, ok := logging.RequestIDFromContext(ctx); ok {
				fields[logging.FieldRequestID] = requestID
			}
			s.logger.WithFields(fields).Debug("Processing completed (cache hit)")

			return cached, nil
		}
	}

	// 1.8 由处理器直接处理（可选：用于多阶段/无LLM流程）
	if dp, ok := any(handler).(directProcessor[T, R]); ok {
		resp, handled, err := dp.ProcessDirect(ctx, req)
		if err != nil {
			return emptyResponse, ensureAppError(err, coreerrors.ErrCodeInternal, "direct processing failed")
		}
		if handled {
			processingTimeSec := time.Since(startTime).Seconds()
			if setter, ok := any(resp).(processingTimeSetter); ok {
				setter.SetProcessingTime(processingTimeSec)
			}

			if cacher, ok := any(handler).(responseCacheHandler[T, R]); ok {
				if err := cacher.StoreCachedResponse(ctx, req, resp); err != nil {
					s.logger.WithError(err).Debug("Failed to store cached response")
				}
			}

			return resp, nil
		}
	}

	// 2. 构建LLM请求
	llmReq, err := handler.BuildLLMRequest(ctx, req)
	if err != nil {
		return emptyResponse, ensureAppError(err, coreerrors.ErrCodeInternal, "failed to build LLM request")
	}

	// 3. 调用LLM
	llmResp, err := s.llmManager.ProcessWithTimeout(ctx, llmReq)
	if err != nil {
		return emptyResponse, ensureAppError(err, coreerrors.ErrCodeLLM, "llm process failed")
	}

	backend := ""
	if llmResp.Metadata != nil {
		if backendName, ok := llmResp.Metadata["backend"].(string); ok {
			backend = backendName
		}
	}
	metrics.ObserveLLMRequestDuration(backend, llmResp.Model, llmResp.Duration)

	// 4. 解析响应
	parsed, err := s.promptEngine.ParseResponse(llmResp.Content)
	if err != nil {
		s.logger.WithError(err).Error("Failed to parse LLM response")
		return emptyResponse, ensureAppError(err, coreerrors.ErrCodeParsing, "failed to parse LLM response")
	}

	// 5. 构建成功响应
	response := handler.BuildSuccessResponse(llmResp, parsed, req)

	processingTimeSec := time.Since(startTime).Seconds()
	if setter, ok := any(response).(processingTimeSetter); ok {
		setter.SetProcessingTime(processingTimeSec)
	}

	if cacher, ok := any(handler).(responseCacheHandler[T, R]); ok {
		if err := cacher.StoreCachedResponse(ctx, req, response); err != nil {
			s.logger.WithError(err).Debug("Failed to store cached response")
		}
	}

	fields := logrus.Fields{
		logging.FieldDuration: time.Since(startTime).Milliseconds(),
		"target_count":        len(req.GetTargetLanguages()),
	}
	if requestID, ok := logging.RequestIDFromContext(ctx); ok {
		fields[logging.FieldRequestID] = requestID
	}
	s.logger.WithFields(fields).Debug("Processing completed")

	return response, nil
}

func ensureAppError(err error, defaultCode coreerrors.ErrorCode, defaultMessage string) error {
	var appErr *coreerrors.AppError
	if errors.As(err, &appErr) {
		return appErr
	}

	msg := defaultMessage
	if msg == "" {
		msg = err.Error()
	}

	switch defaultCode {
	case coreerrors.ErrCodeValidation:
		return coreerrors.NewValidationError(msg, err)
	case coreerrors.ErrCodeAuth:
		return coreerrors.NewAuthError(msg, err)
	case coreerrors.ErrCodeLLM:
		return coreerrors.NewLLMError(msg, err)
	case coreerrors.ErrCodeParsing:
		return coreerrors.NewParsingError(msg, err)
	case coreerrors.ErrCodeInternal:
		return coreerrors.NewInternalError(msg, err)
	default:
		return coreerrors.NewInternalError(msg, err)
	}
}

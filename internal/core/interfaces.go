package core

import (
	"context"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/llm"
)

// Processor defines a minimal contract for request processors.
type Processor interface {
	Process(ctx context.Context, req any) (any, error)
	Validate(req any) error
}

// Backend defines a minimal contract for LLM backends.
type Backend interface {
	Process(ctx context.Context, req *llm.LLMRequest) (*llm.LLMResponse, error)
	HealthCheck(ctx context.Context) error
	GetName() string
}

// Cache defines a minimal cache interface.
type Cache interface {
	Get(key string) (any, bool)
	Set(key string, value any, ttl time.Duration)
	Delete(key string)
}

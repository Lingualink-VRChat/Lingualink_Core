package asr

import "context"

// Backend defines an ASR backend implementation.
type Backend interface {
	Transcribe(ctx context.Context, req *ASRRequest) (*ASRResponse, error)
	HealthCheck(ctx context.Context) error
	GetName() string
}

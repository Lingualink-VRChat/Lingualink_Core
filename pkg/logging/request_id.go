package logging

import "context"

type ctxKeyRequestID struct{}

// WithRequestID returns a new context that carries the given request ID.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	if requestID == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKeyRequestID{}, requestID)
}

// RequestIDFromContext extracts a request ID from the context, if present.
func RequestIDFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	v := ctx.Value(ctxKeyRequestID{})
	s, ok := v.(string)
	if !ok || s == "" {
		return "", false
	}
	return s, true
}

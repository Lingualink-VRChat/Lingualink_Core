// text.go contains text processing endpoints.
package handlers

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	coreerrors "github.com/Lingualink-VRChat/Lingualink_Core/internal/core/errors"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/processing"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/text"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/auth"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/logging"
	"github.com/gin-gonic/gin"
)

// ProcessText 处理文本翻译请求
func (h *Handler) ProcessText(c *gin.Context) {
	h.logger.Info("Processing text translation request")

	decoder := func(c *gin.Context) (text.ProcessRequest, error) {
		var req text.ProcessRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			return text.ProcessRequest{}, fmt.Errorf("invalid JSON: %w", err)
		}
		return req, nil
	}

	handleProcessingRequest(c, h, h.textProcessingService, h.textProcessor, decoder)
}

// ProcessTextBatch 处理批量文本翻译请求
func (h *Handler) ProcessTextBatch(c *gin.Context) {
	h.logger.Info("Processing batch text translation request")

	requestIDStr, ok := logging.RequestIDFromContext(c.Request.Context())
	if !ok {
		requestID, _ := c.Get("request_id")
		requestIDStr, _ = requestID.(string)
	}

	if h.statusStore != nil && requestIDStr != "" {
		_ = h.statusStore.Set(requestIDStr, &processing.ProcessingStatus{
			Status:   "processing",
			Progress: 0,
			Message:  "Batch processing started",
		})
	}

	identity, exists := c.Get("identity")
	if !exists {
		if h.statusStore != nil && requestIDStr != "" {
			_ = h.statusStore.Set(requestIDStr, &processing.ProcessingStatus{
				Status:   "failed",
				Progress: 0,
				Message:  "authentication required",
			})
		}
		respondError(c, http.StatusUnauthorized, coreerrors.NewAuthError("authentication required", nil))
		return
	}
	userIdentity := identity.(*auth.Identity)

	var batchReq text.BatchProcessRequest
	if err := c.ShouldBindJSON(&batchReq); err != nil {
		if h.statusStore != nil && requestIDStr != "" {
			_ = h.statusStore.Set(requestIDStr, &processing.ProcessingStatus{
				Status:   "failed",
				Progress: 0,
				Message:  err.Error(),
			})
		}
		respondError(c, http.StatusBadRequest, coreerrors.NewValidationError(fmt.Sprintf("invalid JSON: %v", err), err))
		return
	}

	if len(batchReq.Texts) == 0 {
		respondError(c, http.StatusBadRequest, coreerrors.NewValidationError("texts is required", nil))
		return
	}
	if len(batchReq.Texts) > 20 {
		respondError(c, http.StatusBadRequest, coreerrors.NewValidationError("texts exceeds maximum batch size (20)", nil))
		return
	}

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	results := make([]*text.ProcessResponse, len(batchReq.Texts))
	var wg sync.WaitGroup

	errCh := make(chan error, 1)
	sem := make(chan struct{}, 4)

	for i, sourceText := range batchReq.Texts {
		i := i
		sourceText := sourceText
		wg.Add(1)
		go func() {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			resp, err := h.textProcessingService.Process(ctx, text.ProcessRequest{
				Text:            sourceText,
				SourceLanguage:  batchReq.SourceLanguage,
				TargetLanguages: batchReq.TargetLanguages,
				Options:         batchReq.Options,
			}, h.textProcessor)
			if err != nil {
				select {
				case errCh <- fmt.Errorf("item %d: %w", i, err):
				default:
				}
				cancel()
				return
			}

			if requestIDStr != "" {
				resp.SetRequestID(requestIDStr)
			}
			results[i] = resp
		}()
	}

	wg.Wait()

	select {
	case err := <-errCh:
		for _, r := range results {
			if r != nil {
				r.Release()
			}
		}
		if h.statusStore != nil && requestIDStr != "" {
			_ = h.statusStore.Set(requestIDStr, &processing.ProcessingStatus{
				Status:   "failed",
				Progress: 100,
				Message:  err.Error(),
			})
		}
		respondError(c, 0, err)
		return
	default:
	}

	h.metrics.RecordCounter("api.process_text_batch.success", 1, map[string]string{"user_id": userIdentity.ID})
	if h.statusStore != nil && requestIDStr != "" {
		_ = h.statusStore.Set(requestIDStr, &processing.ProcessingStatus{
			Status:   "completed",
			Progress: 100,
			Message:  "Batch processing completed",
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"request_id": requestIDStr,
		"results":    results,
		"count":      len(results),
	})
	for _, r := range results {
		if r != nil {
			r.Release()
		}
	}
}

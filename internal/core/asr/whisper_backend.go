package asr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/logging"
	"github.com/sirupsen/logrus"
)

// WhisperBackend implements an OpenAI Whisper compatible API:
// POST {baseURL}/audio/transcriptions
type WhisperBackend struct {
	name       string
	baseURL    string
	model      string
	apiKey     string
	parameters map[string]interface{}
	httpClient *http.Client
	logger     *logrus.Logger
}

func previewLogText(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 240 {
		return s
	}
	return s[:240] + "...(truncated)"
}

func NewWhisperBackend(cfg config.ASRProvider, logger *logrus.Logger) *WhisperBackend {
	baseURL := strings.TrimRight(cfg.URL, "/")
	return &WhisperBackend{
		name:       cfg.Name,
		baseURL:    baseURL,
		model:      cfg.Model,
		apiKey:     cfg.APIKey,
		parameters: cfg.Parameters,
		httpClient: &http.Client{Timeout: 60 * time.Second},
		logger:     logger,
	}
}

func (w *WhisperBackend) GetName() string {
	return w.name
}

func (w *WhisperBackend) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.baseURL+"/models", nil)
	if err != nil {
		return err
	}
	if w.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+w.apiKey)
	}

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("asr health check failed: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (w *WhisperBackend) Transcribe(ctx context.Context, req *ASRRequest) (*ASRResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("nil request")
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	filename := "audio"
	if req.AudioFormat != "" {
		filename = filename + "." + req.AudioFormat
	}

	filePart, err := writer.CreateFormFile("file", path.Base(filename))
	if err != nil {
		return nil, fmt.Errorf("create file form part: %w", err)
	}
	if _, err := io.Copy(filePart, bytes.NewReader(req.Audio)); err != nil {
		return nil, fmt.Errorf("write audio: %w", err)
	}

	if err := writer.WriteField("model", w.model); err != nil {
		return nil, fmt.Errorf("write model field: %w", err)
	}
	if req.Language != "" {
		_ = writer.WriteField("language", req.Language)
	}
	if req.Prompt != "" {
		_ = writer.WriteField("prompt", req.Prompt)
	}

	for k, v := range w.parameters {
		if v == nil {
			continue
		}
		switch typed := v.(type) {
		case string:
			_ = writer.WriteField(k, typed)
		case bool:
			_ = writer.WriteField(k, strconv.FormatBool(typed))
		case int:
			_ = writer.WriteField(k, strconv.Itoa(typed))
		case int64:
			_ = writer.WriteField(k, strconv.FormatInt(typed, 10))
		case float64:
			_ = writer.WriteField(k, strconv.FormatFloat(typed, 'f', -1, 64))
		case float32:
			_ = writer.WriteField(k, strconv.FormatFloat(float64(typed), 'f', -1, 32))
		default:
			// best-effort JSON stringify for complex values
			if b, err := json.Marshal(typed); err == nil {
				_ = writer.WriteField(k, string(b))
			}
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	url := w.baseURL + "/audio/transcriptions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	if w.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+w.apiKey)
	}

	fields := logrus.Fields{
		logging.FieldBackend:     w.name,
		logging.FieldAudioFormat: req.AudioFormat,
		"audio_size":             len(req.Audio),
	}
	if requestID, ok := logging.RequestIDFromContext(ctx); ok {
		fields[logging.FieldRequestID] = requestID
	}
	if w.logger != nil {
		w.logger.WithFields(fields).Debug("Sending ASR transcription request")
	}

	resp, err := w.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("asr transcription failed: status %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed struct {
		Task     string    `json:"task"`
		Language string    `json:"language"`
		Duration float64   `json:"duration"`
		Text     string    `json:"text"`
		Segments []Segment `json:"segments"`
	}
	if err := json.Unmarshal(respBody, &parsed); err == nil && parsed.Text != "" {
		if w.logger != nil {
			fields := logrus.Fields{
				logging.FieldBackend: "qwen-asr",
				"asr_language":       parsed.Language,
				"asr_duration":       parsed.Duration,
				"asr_text_preview":   previewLogText(parsed.Text),
				"asr_raw_preview":    previewLogText(string(respBody)),
			}
			if requestID, ok := logging.RequestIDFromContext(ctx); ok {
				fields[logging.FieldRequestID] = requestID
			}
			w.logger.WithFields(fields).Debug("ASR response parsed")
		}
		return &ASRResponse{
			Text:             parsed.Text,
			DetectedLanguage: parsed.Language,
			Duration:         parsed.Duration,
			Segments:         parsed.Segments,
		}, nil
	}

	// fallback for response_format=text
	text := strings.TrimSpace(string(respBody))
	if w.logger != nil {
		fields := logrus.Fields{
			logging.FieldBackend: "qwen-asr",
			"asr_fallback_text":  previewLogText(text),
			"asr_raw_preview":    previewLogText(string(respBody)),
		}
		if requestID, ok := logging.RequestIDFromContext(ctx); ok {
			fields[logging.FieldRequestID] = requestID
		}
		w.logger.WithFields(fields).Warn("ASR response fell back to raw text")
	}
	return &ASRResponse{Text: text}, nil
}

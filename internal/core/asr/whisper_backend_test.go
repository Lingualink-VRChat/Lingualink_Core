package asr

import (
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	"github.com/sirupsen/logrus"
)

func TestWhisperBackend_HealthCheckAndTranscribe(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[]}`))
			return
		case "/v1/audio/transcriptions":
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
				t.Fatalf("unexpected content-type: %s", r.Header.Get("Content-Type"))
			}
			mr, err := r.MultipartReader()
			if err != nil {
				t.Fatalf("MultipartReader: %v", err)
			}
			foundModel := false
			foundFile := false
			for {
				part, err := mr.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("NextPart: %v", err)
				}
				switch part.FormName() {
				case "model":
					b, _ := io.ReadAll(part)
					if strings.TrimSpace(string(b)) != "whisper-1" {
						t.Fatalf("unexpected model: %s", string(b))
					}
					foundModel = true
				case "file":
					_, _ = io.Copy(io.Discard, part)
					foundFile = true
				default:
					_, _ = io.Copy(io.Discard, part)
				}
				_ = part.Close()
			}
			if !foundModel || !foundFile {
				t.Fatalf("expected model+file, got model=%v file=%v", foundModel, foundFile)
			}

			resp := map[string]any{
				"task":     "transcribe",
				"language": "",
				"duration": 1.23,
				"text":     "language Chinese<asr_text>hello world",
				"segments": []map[string]any{
					{"id": 0, "start": 0.0, "end": 1.23, "text": "hello world"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	logger := logrus.New()
	logger.SetOutput(io.Discard)

	backend := NewWhisperBackend(config.ASRProvider{
		Name:  "asr1",
		Type:  "whisper",
		URL:   srv.URL + "/v1",
		Model: "whisper-1",
		Parameters: map[string]interface{}{
			"response_format": "verbose_json",
			"temperature":     0.0,
		},
	}, logger)

	if err := backend.HealthCheck(context.Background()); err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}

	got, err := backend.Transcribe(context.Background(), &ASRRequest{
		Audio:       []byte("fake-audio"),
		AudioFormat: "wav",
	})
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	if got.Text != "hello world" {
		t.Fatalf("Text = %q", got.Text)
	}
	if got.RawText != "language Chinese<asr_text>hello world" {
		t.Fatalf("RawText = %q", got.RawText)
	}
	if got.DetectedLanguage != "Chinese" {
		t.Fatalf("DetectedLanguage = %q", got.DetectedLanguage)
	}
	if got.Duration <= 0 {
		t.Fatalf("Duration = %v", got.Duration)
	}
	if len(got.Segments) != 1 || got.Segments[0].Text != "hello world" {
		t.Fatalf("Segments = %+v", got.Segments)
	}
}

func TestWhisperBackend_Transcribe_TextFallback(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/transcriptions" {
			http.NotFound(w, r)
			return
		}
		// Ensure it's multipart even for fallback.
		if _, err := multipart.NewReader(r.Body, strings.TrimPrefix(r.Header.Get("Content-Type"), "multipart/form-data; boundary=")).NextPart(); err != nil {
			// ignore; not essential
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("plain text"))
	}))
	t.Cleanup(srv.Close)

	logger := logrus.New()
	logger.SetOutput(io.Discard)

	backend := NewWhisperBackend(config.ASRProvider{
		Name:  "asr1",
		Type:  "whisper",
		URL:   srv.URL + "/v1",
		Model: "whisper-1",
	}, logger)

	got, err := backend.Transcribe(context.Background(), &ASRRequest{Audio: []byte("x"), AudioFormat: "wav"})
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	if got.Text != "plain text" {
		t.Fatalf("Text = %q", got.Text)
	}
}

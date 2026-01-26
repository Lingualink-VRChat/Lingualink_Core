package asr

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	"github.com/sirupsen/logrus"
)

func TestManager_Transcribe(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/transcriptions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"language": "en",
			"duration": 0.5,
			"text":     "hi",
		})
	}))
	t.Cleanup(srv.Close)

	logger := logrus.New()
	logger.SetOutput(io.Discard)

	m, err := NewManager(config.ASRConfig{
		Providers: []config.ASRProvider{
			{
				Name:  "asr1",
				Type:  "whisper",
				URL:   srv.URL + "/v1",
				Model: "whisper-1",
			},
		},
	}, logger)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	resp, err := m.Transcribe(context.Background(), &ASRRequest{Audio: []byte("x"), AudioFormat: "wav"})
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	if resp.Text != "hi" {
		t.Fatalf("Text = %q", resp.Text)
	}
	if len(m.ListBackends()) != 1 {
		t.Fatalf("ListBackends = %v", m.ListBackends())
	}
}

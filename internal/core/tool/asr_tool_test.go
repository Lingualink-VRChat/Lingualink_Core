package tool

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/asr"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/testutil"
)

func newTestASRManager(t *testing.T, text string) *asr.Manager {
	t.Helper()

	asrSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[]}`))
			return
		case "/v1/audio/transcriptions":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"language": "zh",
				"duration": 1.0,
				"text":     text,
			})
			return
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(asrSrv.Close)

	logger := testutil.NewTestLogger()
	m, err := asr.NewManager(config.ASRConfig{
		Providers: []config.ASRProvider{
			{
				Name:  "asr1",
				Type:  "whisper",
				URL:   asrSrv.URL + "/v1",
				Model: "whisper-1",
			},
		},
	}, logger)
	if err != nil {
		t.Fatalf("asr.NewManager: %v", err)
	}
	return m
}

func TestASRTool_Execute(t *testing.T) {
	t.Parallel()

	m := newTestASRManager(t, "你好")
	tool := NewASRTool(m)

	out, err := tool.Execute(context.Background(), Input{
		Data: map[string]interface{}{
			"audio":  []byte{0x00, 0x01},
			"format": "wav",
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got, _ := out.Data["text"].(string); got != "你好" {
		t.Fatalf("text=%q want 你好", got)
	}
}

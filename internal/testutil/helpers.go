package testutil

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/llm"
	"github.com/sirupsen/logrus"
)

// NewTestLogger creates a logger suitable for tests.
func NewTestLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	logger.SetLevel(logrus.DebugLevel)
	logger.SetFormatter(&logrus.JSONFormatter{})
	return logger
}

// RepoRoot returns the repository root directory.
func RepoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("repo root not found from %s", dir)
		}
		dir = parent
	}
}

// LoadTestAudio loads an audio sample from the repository's test data directory.
func LoadTestAudio(t *testing.T, filename string) []byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(RepoRoot(t), "test", filename))
	if err != nil {
		t.Fatalf("read test audio %s: %v", filename, err)
	}
	return data
}

// MockLLMBackend is a configurable in-memory LLM backend for unit tests.
type MockLLMBackend struct {
	Name       string
	Response   *llm.LLMResponse
	ProcessErr error
	HealthErr  error
}

// NewMockLLMBackend creates a mock LLM backend with a fixed response.
func NewMockLLMBackend(name string, response *llm.LLMResponse) llm.LLMBackend {
	return &MockLLMBackend{
		Name:     name,
		Response: response,
	}
}

func (b *MockLLMBackend) Process(ctx context.Context, req *llm.LLMRequest) (*llm.LLMResponse, error) {
	if b.ProcessErr != nil {
		return nil, b.ProcessErr
	}
	if b.Response != nil {
		return b.Response, nil
	}
	return &llm.LLMResponse{Content: "ok", Model: "mock"}, nil
}

func (b *MockLLMBackend) HealthCheck(ctx context.Context) error {
	return b.HealthErr
}

func (b *MockLLMBackend) GetCapabilities() llm.Capabilities {
	return llm.Capabilities{}
}

func (b *MockLLMBackend) GetName() string {
	return b.Name
}

// AssertJSONEqual unmarshals and compares two JSON strings.
func AssertJSONEqual(t *testing.T, expected, actual string) {
	t.Helper()

	var want any
	if err := json.Unmarshal([]byte(expected), &want); err != nil {
		t.Fatalf("unmarshal expected JSON: %v", err)
	}
	var got any
	if err := json.Unmarshal([]byte(actual), &got); err != nil {
		t.Fatalf("unmarshal actual JSON: %v", err)
	}

	if reflect.DeepEqual(want, got) {
		return
	}
	t.Fatalf("json not equal\nexpected=%s\nactual=%s", expected, actual)
}

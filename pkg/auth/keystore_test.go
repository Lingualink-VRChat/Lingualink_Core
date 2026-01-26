package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/testutil"
)

func TestAPIKeyStore_LoadFromFile_Success(t *testing.T) {
	t.Parallel()

	logger := testutil.NewTestLogger()
	store := NewAPIKeyStore(logger)

	dir := t.TempDir()
	path := filepath.Join(dir, "keys.json")

	data := `{
  "keys": {
    "k1": {
      "id": "user-1",
      "requests_per_minute": 60,
      "enabled": true
    }
  }
}`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := store.LoadFromFile(path); err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	if _, ok := store.Keys["k1"]; !ok {
		t.Fatalf("expected key k1")
	}
}

func TestAPIKeyStore_LoadFromFile_ListFormat_ReturnsError(t *testing.T) {
	t.Parallel()

	logger := testutil.NewTestLogger()
	store := NewAPIKeyStore(logger)

	dir := t.TempDir()
	path := filepath.Join(dir, "keys.json")

	data := `{
  "keys": [
    {
      "key": "test-key",
      "name": "test",
      "enabled": true
    }
  ]
}`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	err := store.LoadFromFile(path)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "deprecated list format") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAPIKeyStore_LoadFromFile_NotFoundCreatesDefault(t *testing.T) {
	t.Parallel()

	logger := testutil.NewTestLogger()
	store := NewAPIKeyStore(logger)

	dir := t.TempDir()
	path := filepath.Join(dir, "keys.json")

	if err := store.LoadFromFile(path); err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file to be created: %v", err)
	}
	if _, ok := store.Keys["lingualink-demo-key"]; !ok {
		t.Fatalf("expected default key to exist")
	}
}

func TestAPIKeyStore_LoadFromFile_InvalidJSON(t *testing.T) {
	t.Parallel()

	logger := testutil.NewTestLogger()
	store := NewAPIKeyStore(logger)

	dir := t.TempDir()
	path := filepath.Join(dir, "keys.json")

	if err := os.WriteFile(path, []byte("{not json"), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := store.LoadFromFile(path); err == nil {
		t.Fatalf("expected error")
	}
}

func TestAPIKeyStore_GetKey(t *testing.T) {
	t.Parallel()

	logger := testutil.NewTestLogger()
	store := NewAPIKeyStore(logger)
	store.AddKey("k1", APIKeyConfig{ID: "user-1", Enabled: true})

	if _, ok := store.GetKey("k1"); !ok {
		t.Fatalf("expected key")
	}
	if _, ok := store.GetKey("nope"); ok {
		t.Fatalf("expected missing key")
	}
}

func TestAPIKeyStore_GetKey_Expired(t *testing.T) {
	t.Parallel()

	logger := testutil.NewTestLogger()
	store := NewAPIKeyStore(logger)
	store.AddKey("k1", APIKeyConfig{
		ID:        "user-1",
		Enabled:   true,
		ExpiresAt: time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
	})

	if _, ok := store.GetKey("k1"); ok {
		t.Fatalf("expected expired key to be rejected")
	}
}

func TestAPIKeyStore_ListKeys_Masked(t *testing.T) {
	t.Parallel()

	logger := testutil.NewTestLogger()
	store := NewAPIKeyStore(logger)
	store.AddKey("1234567890abcdef", APIKeyConfig{ID: "user-1", Enabled: true})

	keys := store.ListKeys()
	found := false
	for _, k := range keys {
		if strings.HasPrefix(k, "12345678") && strings.HasSuffix(k, "***") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected masked key, got: %v", keys)
	}
}

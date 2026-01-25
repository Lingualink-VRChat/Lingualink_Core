package auth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/testutil"
)

func TestWebhookAuthenticator_Authenticate_Success(t *testing.T) {
	logger := testutil.NewTestLogger()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Test") != "1" {
			http.Error(w, "missing header", http.StatusBadRequest)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "bad method", http.StatusMethodNotAllowed)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var creds Credentials
		if err := json.Unmarshal(body, &creds); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if creds.APIKey != "k" {
			http.Error(w, "bad key", http.StatusUnauthorized)
			return
		}

		resp := map[string]interface{}{
			"id":          "user-1",
			"external_id": "ext-1",
			"type":        "user",
			"permissions": []string{"audio.process", "health.check"},
			"metadata": map[string]interface{}{
				"source": "webhook",
			},
			"rate_limits": map[string]interface{}{
				"requests_per_minute": 60,
				"burst_size":          10,
				"window_size":         "60s",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(server.Close)

	auth := NewWebhookAuthenticator(server.URL, map[string]interface{}{
		"timeout_ms": 5000,
		"headers": map[string]interface{}{
			"X-Test": "1",
		},
	}, logger)

	identity, err := auth.Authenticate(context.Background(), Credentials{APIKey: "k"})
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if identity.ID != "user-1" {
		t.Fatalf("id=%q want user-1", identity.ID)
	}
	if identity.ExternalID != "ext-1" {
		t.Fatalf("external_id=%q want ext-1", identity.ExternalID)
	}
	if identity.Type != IdentityTypeUser {
		t.Fatalf("type=%q want %q", identity.Type, IdentityTypeUser)
	}
	if identity.RateLimits == nil || identity.RateLimits.WindowSize != time.Minute {
		t.Fatalf("unexpected rate limits: %+v", identity.RateLimits)
	}
}

func TestWebhookAuthenticator_Authenticate_Unauthorized(t *testing.T) {
	logger := testutil.NewTestLogger()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	auth := NewWebhookAuthenticator(server.URL, nil, logger)
	if _, err := auth.Authenticate(context.Background(), Credentials{APIKey: "k"}); err != ErrUnauthorized {
		t.Fatalf("err=%v want ErrUnauthorized", err)
	}
}

func TestWebhookAuthenticator_Authenticate_MissingEndpoint(t *testing.T) {
	logger := testutil.NewTestLogger()
	auth := NewWebhookAuthenticator("", nil, logger)
	if _, err := auth.Authenticate(context.Background(), Credentials{APIKey: "k"}); err == nil {
		t.Fatalf("expected error")
	}
}

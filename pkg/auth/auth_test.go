package auth

import (
	"context"
	"testing"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/testutil"
)

func TestAPIKeyAuthenticator_Authenticate(t *testing.T) {
	t.Parallel()

	logger := testutil.NewTestLogger()
	store := NewAPIKeyStore(logger)
	store.AddKey("valid-key", APIKeyConfig{
		ID:                "user-1",
		RequestsPerMinute: 60,
		Enabled:           true,
	})

	auth := &APIKeyAuthenticator{keyStore: store, logger: logger}

	identity, err := auth.Authenticate(context.Background(), Credentials{APIKey: "valid-key"})
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if identity.ID != "user-1" {
		t.Fatalf("id=%q want user-1", identity.ID)
	}
	if identity.Type != IdentityTypeUser {
		t.Fatalf("type=%q want %q", identity.Type, IdentityTypeUser)
	}
	if identity.RateLimits == nil {
		t.Fatalf("expected rate limits")
	}
	if identity.RateLimits.RequestsPerMinute != 60 {
		t.Fatalf("rpm=%d want 60", identity.RateLimits.RequestsPerMinute)
	}
	if identity.RateLimits.WindowSize != time.Minute {
		t.Fatalf("window=%v want %v", identity.RateLimits.WindowSize, time.Minute)
	}
}

func TestAPIKeyAuthenticator_InvalidKey(t *testing.T) {
	t.Parallel()

	logger := testutil.NewTestLogger()
	store := NewAPIKeyStore(logger)
	store.AddKey("valid-key", APIKeyConfig{ID: "user-1", RequestsPerMinute: 60, Enabled: true})

	auth := &APIKeyAuthenticator{keyStore: store, logger: logger}

	if _, err := auth.Authenticate(context.Background(), Credentials{APIKey: "nope"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestAPIKeyAuthenticator_EmptyKey(t *testing.T) {
	t.Parallel()

	logger := testutil.NewTestLogger()
	auth := &APIKeyAuthenticator{keyStore: NewAPIKeyStore(logger), logger: logger}

	if _, err := auth.Authenticate(context.Background(), Credentials{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestAPIKeyAuthenticator_UnlimitedKey(t *testing.T) {
	t.Parallel()

	logger := testutil.NewTestLogger()
	store := NewAPIKeyStore(logger)
	store.AddKey("unlimited", APIKeyConfig{
		ID:                "user-1",
		RequestsPerMinute: -1,
		Enabled:           true,
	})

	auth := &APIKeyAuthenticator{keyStore: store, logger: logger}

	identity, err := auth.Authenticate(context.Background(), Credentials{APIKey: "unlimited"})
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if identity.RateLimits == nil || identity.RateLimits.RequestsPerMinute != -1 {
		t.Fatalf("unexpected rate limits: %+v", identity.RateLimits)
	}
}

func TestAPIKeyAuthenticator_ServiceIdentity(t *testing.T) {
	t.Parallel()

	logger := testutil.NewTestLogger()
	store := NewAPIKeyStore(logger)
	store.AddKey("service-key", APIKeyConfig{
		ID:                "backend-service",
		RequestsPerMinute: 120,
		Enabled:           true,
	})

	auth := &APIKeyAuthenticator{keyStore: store, logger: logger}

	identity, err := auth.Authenticate(context.Background(), Credentials{APIKey: "service-key"})
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if identity.Type != IdentityTypeService {
		t.Fatalf("type=%q want %q", identity.Type, IdentityTypeService)
	}
}

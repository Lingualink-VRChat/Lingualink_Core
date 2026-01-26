package auth

import (
	"context"
	"testing"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/testutil"
)

type recordingAuthenticator struct {
	typ    string
	called bool
}

func (a *recordingAuthenticator) Authenticate(ctx context.Context, credentials Credentials) (*Identity, error) {
	a.called = true
	return &Identity{ID: a.typ, Type: IdentityTypeUser}, nil
}

func (a *recordingAuthenticator) GetType() string {
	return a.typ
}

func TestMultiAuthenticator_AutoDetectAPIKey(t *testing.T) {
	t.Parallel()

	apiAuth := &recordingAuthenticator{typ: "api_key"}
	jwtAuth := &recordingAuthenticator{typ: "jwt"}
	anonAuth := &recordingAuthenticator{typ: "anonymous"}

	ma := &MultiAuthenticator{
		authenticators: map[string]Authenticator{
			"api_key":   apiAuth,
			"jwt":       jwtAuth,
			"anonymous": anonAuth,
		},
		logger: testutil.NewTestLogger(),
	}

	_, err := ma.Authenticate(context.Background(), Credentials{APIKey: "k"})
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if !apiAuth.called || jwtAuth.called || anonAuth.called {
		t.Fatalf("unexpected calls: api=%v jwt=%v anon=%v", apiAuth.called, jwtAuth.called, anonAuth.called)
	}
}

func TestMultiAuthenticator_AutoDetectJWT(t *testing.T) {
	t.Parallel()

	apiAuth := &recordingAuthenticator{typ: "api_key"}
	jwtAuth := &recordingAuthenticator{typ: "jwt"}
	anonAuth := &recordingAuthenticator{typ: "anonymous"}

	ma := &MultiAuthenticator{
		authenticators: map[string]Authenticator{
			"api_key":   apiAuth,
			"jwt":       jwtAuth,
			"anonymous": anonAuth,
		},
		logger: testutil.NewTestLogger(),
	}

	_, err := ma.Authenticate(context.Background(), Credentials{Token: "Bearer x.y.z"})
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if !jwtAuth.called || apiAuth.called || anonAuth.called {
		t.Fatalf("unexpected calls: api=%v jwt=%v anon=%v", apiAuth.called, jwtAuth.called, anonAuth.called)
	}
}

func TestMultiAuthenticator_AutoDetectBearerAPIKey(t *testing.T) {
	t.Parallel()

	apiAuth := &recordingAuthenticator{typ: "api_key"}
	jwtAuth := &recordingAuthenticator{typ: "jwt"}
	anonAuth := &recordingAuthenticator{typ: "anonymous"}

	ma := &MultiAuthenticator{
		authenticators: map[string]Authenticator{
			"api_key":   apiAuth,
			"jwt":       jwtAuth,
			"anonymous": anonAuth,
		},
		logger: testutil.NewTestLogger(),
	}

	_, err := ma.Authenticate(context.Background(), Credentials{Token: "Bearer test-key"})
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if !apiAuth.called || jwtAuth.called || anonAuth.called {
		t.Fatalf("unexpected calls: api=%v jwt=%v anon=%v", apiAuth.called, jwtAuth.called, anonAuth.called)
	}
}

func TestMultiAuthenticator_AnonymousFallback(t *testing.T) {
	t.Parallel()

	anonAuth := &recordingAuthenticator{typ: "anonymous"}
	ma := &MultiAuthenticator{
		authenticators: map[string]Authenticator{
			"anonymous": anonAuth,
		},
		logger: testutil.NewTestLogger(),
	}

	_, err := ma.Authenticate(context.Background(), Credentials{})
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if !anonAuth.called {
		t.Fatalf("expected anonymous authenticator to be called")
	}
}

func TestMultiAuthenticator_UnsupportedType(t *testing.T) {
	t.Parallel()

	ma := &MultiAuthenticator{
		authenticators: map[string]Authenticator{},
		logger:         testutil.NewTestLogger(),
	}

	if _, err := ma.Authenticate(context.Background(), Credentials{Type: "nope"}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestNewMultiAuthenticator_DisabledStrategyNotLoaded(t *testing.T) {
	t.Parallel()

	logger := testutil.NewTestLogger()
	ma := NewMultiAuthenticator(config.AuthConfig{
		Strategies: []config.AuthStrategy{
			{Type: "api_key", Enabled: false},
			{Type: "jwt", Enabled: false},
		},
	}, logger)

	if _, ok := ma.authenticators["api_key"]; ok {
		t.Fatalf("expected api_key not to be loaded")
	}
	if _, ok := ma.authenticators["jwt"]; ok {
		t.Fatalf("expected jwt not to be loaded")
	}
}

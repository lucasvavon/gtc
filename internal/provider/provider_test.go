package provider

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lucasvavon/gtc/internal/models"
)

// mockProvider is a minimal Provider implementation used to verify that the
// registry can store and return concrete types without data loss.
type mockProvider struct {
	name string
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) Authenticate(_ context.Context) error {
	return nil
}
func (m *mockProvider) ListPullRequests(_ context.Context, _ models.PRListOptions) ([]models.PullRequest, error) {
	return nil, nil
}
func (m *mockProvider) GetPullRequest(_ context.Context, _ int) (*models.PullRequest, error) {
	return nil, nil
}
func (m *mockProvider) ListBranches(_ context.Context) ([]models.Branch, error) {
	return nil, nil
}
func (m *mockProvider) ListCommits(_ context.Context, _ models.CommitListOptions) ([]models.Commit, error) {
	return nil, nil
}
func (m *mockProvider) GetCIStatus(_ context.Context, _ string) (*models.CIStatus, error) {
	return nil, nil
}
func (m *mockProvider) Watch(_ context.Context, _ models.WatchOptions) (<-chan models.Event, error) {
	return nil, nil
}

// resetRegistry replaces the global registry with a fresh map and returns a
// restore function. Call it with defer to avoid test pollution.
func resetRegistry(t *testing.T) {
	t.Helper()
	saved := registry
	registry = map[string]Factory{}
	t.Cleanup(func() { registry = saved })
}

// TestRegisterAndGet verifies the happy path: register a factory, retrieve it,
// instantiate a provider, and confirm the name round-trips correctly.
func TestRegisterAndGet(t *testing.T) {
	resetRegistry(t)

	const providerName = "mock"

	Register(providerName, func(cfg ProviderConfig) (Provider, error) {
		return &mockProvider{name: cfg.Name}, nil
	})

	factory, err := Get(providerName)
	if err != nil {
		t.Fatalf("Get(%q) returned unexpected error: %v", providerName, err)
	}

	p, err := factory(ProviderConfig{Name: providerName})
	if err != nil {
		t.Fatalf("factory returned unexpected error: %v", err)
	}

	if p.Name() != providerName {
		t.Errorf("Provider.Name(): got %q, want %q", p.Name(), providerName)
	}
}

// TestGet_UnknownProvider verifies that Get returns a descriptive error for an
// unregistered name.
func TestGet_UnknownProvider(t *testing.T) {
	resetRegistry(t)

	Register("gitlab", func(cfg ProviderConfig) (Provider, error) {
		return &mockProvider{name: cfg.Name}, nil
	})

	_, err := Get("github")
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
	if !strings.Contains(err.Error(), "github") {
		t.Errorf("error should mention the requested name, got: %v", err)
	}
	if !strings.Contains(err.Error(), "gitlab") {
		t.Errorf("error should list registered providers, got: %v", err)
	}
}

// TestGet_EmptyRegistry verifies that Get returns an error when no providers
// are registered at all.
func TestGet_EmptyRegistry(t *testing.T) {
	resetRegistry(t)

	_, err := Get("anything")
	if err == nil {
		t.Fatal("expected error for empty registry, got nil")
	}
}

// TestRegister_Overwrite verifies that re-registering a name replaces the
// previous factory (last registration wins).
func TestRegister_Overwrite(t *testing.T) {
	resetRegistry(t)

	Register("mock", func(_ ProviderConfig) (Provider, error) {
		return &mockProvider{name: "original"}, nil
	})
	Register("mock", func(_ ProviderConfig) (Provider, error) {
		return &mockProvider{name: "overwritten"}, nil
	})

	factory, err := Get("mock")
	if err != nil {
		t.Fatalf("Get returned unexpected error: %v", err)
	}

	p, err := factory(ProviderConfig{})
	if err != nil {
		t.Fatalf("factory returned unexpected error: %v", err)
	}

	if p.Name() != "overwritten" {
		t.Errorf("expected overwritten factory to be active, got name %q", p.Name())
	}
}

// TestRegister_FactoryError verifies that an error returned by the Factory
// itself propagates correctly to the caller.
func TestRegister_FactoryError(t *testing.T) {
	resetRegistry(t)

	factoryErr := errors.New("invalid token")
	Register("broken", func(_ ProviderConfig) (Provider, error) {
		return nil, factoryErr
	})

	factory, err := Get("broken")
	if err != nil {
		t.Fatalf("Get returned unexpected error: %v", err)
	}

	_, err = factory(ProviderConfig{})
	if !errors.Is(err, factoryErr) {
		t.Errorf("expected factory error %v, got %v", factoryErr, err)
	}
}

// TestProviderConfig_Fields verifies that ProviderConfig holds all expected
// fields without data loss through a simple assignment round-trip.
func TestProviderConfig_Fields(t *testing.T) {
	cfg := ProviderConfig{
		Name:    "gitlab",
		BaseURL: "https://gitlab.example.com",
		Token:   "glpat-secret",
		Owner:   "mygroup",
		Repo:    "myrepo",
		Extra:   map[string]string{"tls_verify": "false"},
	}

	if cfg.Name != "gitlab" {
		t.Errorf("Name: got %q", cfg.Name)
	}
	if cfg.BaseURL != "https://gitlab.example.com" {
		t.Errorf("BaseURL: got %q", cfg.BaseURL)
	}
	if cfg.Token != "glpat-secret" {
		t.Errorf("Token: got %q", cfg.Token)
	}
	if cfg.Owner != "mygroup" {
		t.Errorf("Owner: got %q", cfg.Owner)
	}
	if cfg.Repo != "myrepo" {
		t.Errorf("Repo: got %q", cfg.Repo)
	}
	if cfg.Extra["tls_verify"] != "false" {
		t.Errorf("Extra[tls_verify]: got %q", cfg.Extra["tls_verify"])
	}
}

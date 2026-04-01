package provider

import (
	"context"
	"fmt"

	"github.com/lucasvavon/gtc/internal/models"
)

// Provider is the abstraction every VCS backend must implement.
// Each method receives a context so callers can enforce deadlines and
// cancellation across the network boundary.
type Provider interface {
	// Name returns the canonical name of the provider (e.g. "gitlab", "github").
	Name() string

	// Authenticate verifies that the configured credentials are valid.
	Authenticate(ctx context.Context) error

	// ListPullRequests returns pull requests matching the given options.
	ListPullRequests(ctx context.Context, opts models.PRListOptions) ([]models.PullRequest, error)

	// GetPullRequest returns a single pull request by its numeric ID.
	GetPullRequest(ctx context.Context, id int) (*models.PullRequest, error)

	// ListBranches returns all branches for the configured repository.
	ListBranches(ctx context.Context) ([]models.Branch, error)

	// ListCommits returns commits matching the given options.
	ListCommits(ctx context.Context, opts models.CommitListOptions) ([]models.Commit, error)

	// GetCIStatus returns the pipeline status for the given ref (branch or SHA).
	GetCIStatus(ctx context.Context, ref string) (*models.CIStatus, error)

	// Watch streams events matching opts until ctx is cancelled.
	// The returned channel is closed when the provider stops emitting events.
	Watch(ctx context.Context, opts models.WatchOptions) (<-chan models.Event, error)
}

// ProviderConfig holds the configuration passed to a Factory when creating a
// new Provider instance. Fields are tagged for both JSON and YAML so that
// config files and environment-variable unmarshalling work out of the box.
type ProviderConfig struct {
	Name    string            `yaml:"name"     json:"name"`
	BaseURL string            `yaml:"base_url" json:"base_url"`
	Token   string            `yaml:"token"    json:"token"`
	Owner   string            `yaml:"owner"    json:"owner"`
	Repo    string            `yaml:"repo"     json:"repo"`
	Extra   map[string]string `yaml:"extra"    json:"extra"`
}

// Factory is a constructor function that builds a Provider from a
// ProviderConfig. Implementations are registered at program start-up via
// Register and resolved at runtime via Get.
type Factory = func(ProviderConfig) (Provider, error)

// registry holds the mapping from provider name to its Factory.
// It is populated by Register calls, typically in package init() functions.
var registry = map[string]Factory{}

// Register associates name with the given Factory in the global registry.
// It is safe to call from multiple init() functions; the last registration
// for a given name wins (allows tests to override the default factory).
func Register(name string, f Factory) {
	registry[name] = f
}

// Get looks up the Factory registered under name.
// It returns an error listing available providers when name is not found.
func Get(name string) (Factory, error) {
	f, ok := registry[name]
	if !ok {
		available := make([]string, 0, len(registry))
		for k := range registry {
			available = append(available, k)
		}
		return nil, fmt.Errorf("unknown provider %q (registered: %v)", name, available)
	}
	return f, nil
}

package gitlab

import (
	"fmt"

	"github.com/lucasvavon/gtc/config"
	"github.com/xanzy/go-gitlab"
)

// Client wraps the go-gitlab client and exposes helpers used by services.
type Client struct {
	gl  *gitlab.Client //nolint:staticcheck
	cfg *config.Config
}

// New creates a GitLab client from the provided config.
// It validates that a token is present before returning.
func New(cfg *config.Config) (*Client, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf(
			"no GitLab token found — set GITLAB_TOKEN or add 'token' to ~/.gtc.yaml",
		)
	}

	opts := []gitlab.ClientOptionFunc{
		gitlab.WithBaseURL(cfg.BaseURL),
	}

	gl, err := gitlab.NewClient(cfg.Token, opts...) //nolint:staticcheck
	if err != nil {
		return nil, fmt.Errorf("creating gitlab client: %w", err)
	}

	return &Client{gl: gl, cfg: cfg}, nil
}

// GL exposes the underlying go-gitlab client to service packages.
func (c *Client) GL() *gitlab.Client { return c.gl } //nolint:staticcheck

// ResolveRepos returns the repos list from flags if provided,
// otherwise falls back to the ones defined in config.
func (c *Client) ResolveRepos(flagRepos []string) ([]string, error) {
	if len(flagRepos) > 0 {
		return flagRepos, nil
	}
	if len(c.cfg.Repos) > 0 {
		return c.cfg.Repos, nil
	}
	return nil, fmt.Errorf(
		"no repos specified — use --repos flag or set 'repos' in ~/.gtc.yaml",
	)
}

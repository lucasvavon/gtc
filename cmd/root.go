package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/lucasvavon/gtc/internal/config"
	"github.com/lucasvavon/gtc/internal/provider"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gtc",
	Short: "Git Tool CLI — query pull requests, branches, commits and CI across providers",
	Long: `gtc is a fast CLI to query your VCS provider (GitHub, GitLab, Gitea…).

Run 'gtc init' inside a git repository to create a .gtc.yaml config file,
then use the sub-commands to query your project:

  gtc init
  gtc pr list --state open
  gtc pr show 42
  gtc branch list
  gtc commit list --author alice --limit 20
  gtc ci status main
  gtc watch --events pr_opened,ci_failed`,
	SilenceErrors: true,
	SilenceUsage:  true,
}

// Execute is the binary entry point called from main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(
		initCmd,
		prCmd,
		branchCmd,
		commitCmd,
		ciCmd,
		watchCmd,
		versionCmd,
	)
}

// loadProvider loads the project config, resolves the registered factory for
// the configured provider, instantiates it, and verifies credentials via
// Authenticate. It returns a ready-to-use Provider or a descriptive error.
func loadProvider(ctx context.Context) (provider.Provider, error) {
	// 1. Read .gtc.yaml from the git root.
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	// 2. Resolve the factory registered under the provider name.
	factory, err := provider.Get(cfg.Provider.Name)
	if err != nil {
		return nil, err
	}

	// 3. Instantiate the provider.
	p, err := factory(provider.ProviderConfig{
		Name:    cfg.Provider.Name,
		BaseURL: cfg.Provider.BaseURL,
		Token:   cfg.Provider.Token,
		Owner:   cfg.Provider.Owner,
		Repo:    cfg.Provider.Repo,
	})
	if err != nil {
		return nil, fmt.Errorf("initialising provider %q: %w", cfg.Provider.Name, err)
	}

	// 4. Verify credentials before returning.
	if err := p.Authenticate(ctx); err != nil {
		return nil, fmt.Errorf("authenticating with %q: %w", cfg.Provider.Name, err)
	}

	return p, nil
}

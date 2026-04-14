package cmd

import (
	"os"

	"github.com/lucasvavon/gtc/internal/config"
	"github.com/spf13/cobra"
)

var initYes bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create .gtc.yaml at the git repository root",
	Long: `Creates a .gtc.yaml configuration file.

gtc init detects the provider, owner and repository from the git remote URL.
If a token is found in a standard environment variable (GITHUB_TOKEN, GH_TOKEN,
GITLAB_TOKEN…) the config is written after a single confirmation prompt.

Use --yes to skip all prompts entirely (suitable for CI or dotfile scripts).`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return config.RunInteractiveInit(os.Stdin, os.Stdout, initYes)
	},
}

func init() {
	initCmd.Flags().BoolVarP(&initYes, "yes", "y", false,
		"skip all prompts and write config automatically (requires auto-detectable token)")
}

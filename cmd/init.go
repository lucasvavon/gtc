package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/lucasvavon/gtc/internal/config"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create .gtc.yaml interactively at the git repository root",
	Long: `Guides you through creating a .gtc.yaml configuration file.

gtc init detects the VCS provider and repository from the git remote and
pre-fills the prompts where possible. The file is written to the root of the
current git repository with permissions 0600.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return config.RunInteractiveInit(os.Stdin, os.Stdout)
	},
}

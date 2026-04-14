package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags "-X github.com/lucasvavon/gtc/cmd.Version=<tag>".
var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print gtc version",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("gtc %s\n", Version)
	},
}

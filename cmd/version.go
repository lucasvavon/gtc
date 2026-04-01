package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print gtc version",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Println("gtc dev")
	},
}

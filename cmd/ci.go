package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// ciCmd is the parent for all CI sub-commands.
var ciCmd = &cobra.Command{
	Use:   "ci",
	Short: "Query CI pipeline status",
}

var ciStatusCmd = &cobra.Command{
	Use:   "status [ref]",
	Short: "Show CI pipeline status for a ref (branch, tag or SHA)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ref := ""
		if len(args) == 1 {
			ref = args[0]
		}

		p, err := loadProvider(cmd.Context())
		if err != nil {
			return err
		}

		_ = p
		if ref != "" {
			fmt.Printf("ci status %s: not yet implemented\n", ref)
		} else {
			fmt.Println("ci status: not yet implemented")
		}
		return nil
	},
}

func init() {
	ciCmd.AddCommand(ciStatusCmd)
}

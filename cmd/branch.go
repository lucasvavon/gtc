package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// branchCmd is the parent for all branch sub-commands.
var branchCmd = &cobra.Command{
	Use:   "branch",
	Short: "Manage branches",
}

var branchListCmd = &cobra.Command{
	Use:   "list",
	Short: "List branches",
	RunE: func(cmd *cobra.Command, _ []string) error {
		p, err := loadProvider(cmd.Context())
		if err != nil {
			return err
		}

		_ = p
		fmt.Println("branch list: not yet implemented")
		return nil
	},
}

func init() {
	branchCmd.AddCommand(branchListCmd)
}

package cmd

import (
	"github.com/lucasvavon/gtc/internal/output"
	"github.com/lucasvavon/gtc/internal/ui"
	"github.com/spf13/cobra"
)

// branch flags
var branchFormat string

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

		branches, err := p.ListBranches(cmd.Context())
		if err != nil {
			return err
		}

		fmt, err := output.ParseFormat(branchFormat)
		if err != nil {
			return err
		}

		if fmt == output.FormatTable {
			print(ui.RenderBranchTable(branches))
			return nil
		}
		return output.New(fmt).Print(branches)
	},
}

func init() {
	branchListCmd.Flags().StringVar(&branchFormat, "format", "table", "output format: table|json|yaml")
	branchCmd.AddCommand(branchListCmd)
}

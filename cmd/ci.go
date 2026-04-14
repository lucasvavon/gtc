package cmd

import (
	"github.com/lucasvavon/gtc/internal/output"
	"github.com/lucasvavon/gtc/internal/ui"
	"github.com/spf13/cobra"
)

// ci flags
var ciFormat string

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
		ref := "HEAD"
		if len(args) == 1 {
			ref = args[0]
		}

		p, err := loadProvider(cmd.Context())
		if err != nil {
			return err
		}

		status, err := p.GetCIStatus(cmd.Context(), ref)
		if err != nil {
			return err
		}

		fmt, err := output.ParseFormat(ciFormat)
		if err != nil {
			return err
		}

		if fmt == output.FormatTable {
			print(ui.RenderCIStatus(status))
			return nil
		}
		return output.New(fmt).Print(status)
	},
}

func init() {
	ciStatusCmd.Flags().StringVar(&ciFormat, "format", "table", "output format: table|json|yaml")
	ciCmd.AddCommand(ciStatusCmd)
}

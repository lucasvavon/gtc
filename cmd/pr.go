package cmd

import (
	"fmt"
	"strconv"

	"github.com/lucasvavon/gtc/internal/models"
	"github.com/lucasvavon/gtc/internal/output"
	"github.com/lucasvavon/gtc/internal/ui"
	"github.com/spf13/cobra"
)

// pr flags
var (
	prState  string
	prAuthor string
	prLimit  int
	prFormat string
)

// prCmd is the parent for all pull-request sub-commands.
var prCmd = &cobra.Command{
	Use:   "pr",
	Short: "Manage pull requests",
}

var prListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pull requests",
	RunE: func(cmd *cobra.Command, _ []string) error {
		p, err := loadProvider(cmd.Context())
		if err != nil {
			return err
		}

		prs, err := p.ListPullRequests(cmd.Context(), models.PRListOptions{
			State:  prState,
			Author: prAuthor,
			Limit:  prLimit,
		})
		if err != nil {
			return err
		}

		fmt, err := output.ParseFormat(prFormat)
		if err != nil {
			return err
		}

		if fmt == output.FormatTable {
			print(ui.RenderPRTable(prs))
			return nil
		}
		return output.New(fmt).Print(prs)
	},
}

var prShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show a single pull request by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid pull request ID %q: must be an integer", args[0])
		}

		p, err := loadProvider(cmd.Context())
		if err != nil {
			return err
		}

		pr, err := p.GetPullRequest(cmd.Context(), id)
		if err != nil {
			return err
		}

		fmt, err := output.ParseFormat(prFormat)
		if err != nil {
			return err
		}

		if fmt == output.FormatTable {
			print(ui.RenderPRDetail(pr))
			return nil
		}
		return output.New(fmt).Print(pr)
	},
}

func init() {
	prListCmd.Flags().StringVar(&prState, "state", "open", "filter by state: open|closed|merged|all")
	prListCmd.Flags().StringVar(&prAuthor, "author", "", "filter by author username")
	prListCmd.Flags().IntVar(&prLimit, "limit", 30, "maximum number of results")
	prListCmd.Flags().StringVar(&prFormat, "format", "table", "output format: table|json|yaml")

	prShowCmd.Flags().StringVar(&prFormat, "format", "table", "output format: table|json|yaml")

	prCmd.AddCommand(prListCmd, prShowCmd)
}

package cmd

import (
	"github.com/lucasvavon/gtc/internal/models"
	"github.com/lucasvavon/gtc/internal/output"
	"github.com/lucasvavon/gtc/internal/ui"
	"github.com/spf13/cobra"
)

// commit flags
var (
	commitBranch string
	commitAuthor string
	commitLimit  int
	commitFormat string
)

// commitCmd is the parent for all commit sub-commands.
var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Manage commits",
}

var commitListCmd = &cobra.Command{
	Use:   "list",
	Short: "List commits",
	RunE: func(cmd *cobra.Command, _ []string) error {
		p, err := loadProvider(cmd.Context())
		if err != nil {
			return err
		}

		commits, err := p.ListCommits(cmd.Context(), models.CommitListOptions{
			Branch: commitBranch,
			Author: commitAuthor,
			Limit:  commitLimit,
		})
		if err != nil {
			return err
		}

		fmt, err := output.ParseFormat(commitFormat)
		if err != nil {
			return err
		}

		if fmt == output.FormatTable {
			print(ui.RenderCommitTable(commits))
			return nil
		}
		return output.New(fmt).Print(commits)
	},
}

func init() {
	commitListCmd.Flags().StringVar(&commitBranch, "branch", "", "filter by branch name")
	commitListCmd.Flags().StringVar(&commitAuthor, "author", "", "filter by author username")
	commitListCmd.Flags().IntVar(&commitLimit, "limit", 30, "maximum number of results")
	commitListCmd.Flags().StringVar(&commitFormat, "format", "table", "output format: table|json|yaml")

	commitCmd.AddCommand(commitListCmd)
}

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/lucasvavon/gtc/internal/models"
)

// commit flags
var (
	commitBranch string
	commitAuthor string
	commitLimit  int
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

		opts := models.CommitListOptions{
			Branch: commitBranch,
			Author: commitAuthor,
			Limit:  commitLimit,
		}

		_ = p
		_ = opts
		fmt.Println("commit list: not yet implemented")
		return nil
	},
}

func init() {
	commitListCmd.Flags().StringVar(&commitBranch, "branch", "", "filter by branch name")
	commitListCmd.Flags().StringVar(&commitAuthor, "author", "", "filter by author username")
	commitListCmd.Flags().IntVar(&commitLimit, "limit", 30, "maximum number of results")

	commitCmd.AddCommand(commitListCmd)
}

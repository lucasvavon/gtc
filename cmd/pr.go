package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/lucasvavon/gtc/internal/models"
)

// pr flags
var (
	prState  string
	prAuthor string
	prLimit  int
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

		opts := models.PRListOptions{
			State:  prState,
			Author: prAuthor,
			Limit:  prLimit,
		}

		_ = p
		_ = opts
		fmt.Println("pr list: not yet implemented")
		return nil
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

		_ = p
		fmt.Printf("pr show %d: not yet implemented\n", id)
		return nil
	},
}

func init() {
	// list flags
	prListCmd.Flags().StringVar(&prState, "state", "open", "filter by state: open|closed|merged|all")
	prListCmd.Flags().StringVar(&prAuthor, "author", "", "filter by author username")
	prListCmd.Flags().IntVar(&prLimit, "limit", 30, "maximum number of results")

	prCmd.AddCommand(prListCmd, prShowCmd)
}

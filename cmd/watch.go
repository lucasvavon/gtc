package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/lucasvavon/gtc/internal/models"
)

// watch flags
var (
	watchInterval string
	watchEvents   string
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch for events and stream them to the terminal",
	Long: `Polls the provider at the configured interval and prints events as they occur.

Events can be filtered with --events (comma-separated list):
  pr_opened, pr_merged, pr_closed, ci_failed, ci_success,
  branch_created, branch_deleted, push`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		interval, err := time.ParseDuration(watchInterval)
		if err != nil {
			return fmt.Errorf("invalid --interval %q: %w", watchInterval, err)
		}

		var kinds []models.EventKind
		for _, raw := range strings.Split(watchEvents, ",") {
			raw = strings.TrimSpace(raw)
			if raw != "" {
				kinds = append(kinds, models.EventKind(raw))
			}
		}

		p, err := loadProvider(cmd.Context())
		if err != nil {
			return err
		}

		opts := models.WatchOptions{
			Events:   kinds,
			Interval: interval,
		}

		_ = p
		_ = opts
		fmt.Printf("watch (interval=%s events=%v): not yet implemented\n", watchInterval, kinds)
		return nil
	},
}

func init() {
	watchCmd.Flags().StringVar(&watchInterval, "interval", "30s", "polling interval (e.g. 30s, 1m, 5m)")
	watchCmd.Flags().StringVar(&watchEvents, "events", "", "comma-separated event kinds to watch (default: all)")
}

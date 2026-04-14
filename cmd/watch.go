package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/lucasvavon/gtc/internal/models"
	"github.com/spf13/cobra"
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

		ch, err := p.Watch(cmd.Context(), opts)
		if err != nil {
			return err
		}

		fmt.Printf("Watching %s (interval=%s", p.Name(), watchInterval)
		if len(kinds) > 0 {
			fmt.Printf(", events=%s", watchEvents)
		}
		fmt.Println(") — Ctrl-C to stop")

		for e := range ch {
			actor := ""
			if e.Actor != "" {
				actor = " by " + e.Actor
			}
			detail := ""
			if e.Detail != "" {
				detail = "  " + e.Detail
			}
			fmt.Printf("[%s] %-16s  %s%s%s\n",
				e.Timestamp.Format("15:04:05"),
				e.Kind,
				e.Title,
				actor,
				detail,
			)
		}
		return nil
	},
}

func init() {
	watchCmd.Flags().StringVar(&watchInterval, "interval", "30s", "polling interval (e.g. 30s, 1m, 5m)")
	watchCmd.Flags().StringVar(&watchEvents, "events", "", "comma-separated event kinds to watch (default: all)")
}

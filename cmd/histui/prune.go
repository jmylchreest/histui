package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/jmylchreest/histui/internal/core"
	"github.com/jmylchreest/histui/internal/model"
)

var pruneOpts struct {
	olderThan string
	keep      int
	dryRun    bool
}

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove old notifications from history",
	Long: `Remove old notifications from the persistent history.

Examples:
  # Remove notifications older than 7 days
  histui prune --older-than 7d

  # Keep only the 100 most recent notifications
  histui prune --keep 100

  # Preview what would be removed (dry run)
  histui prune --older-than 48h --dry-run`,
	RunE: runPrune,
}

func init() {
	rootCmd.AddCommand(pruneCmd)

	pruneCmd.Flags().StringVar(&pruneOpts.olderThan, "older-than", "",
		"Remove notifications older than this duration (e.g., 48h, 7d, 1w)")
	pruneCmd.Flags().IntVar(&pruneOpts.keep, "keep", 0,
		"Keep only the N most recent notifications (0=unlimited)")
	pruneCmd.Flags().BoolVar(&pruneOpts.dryRun, "dry-run", false,
		"Show what would be removed without actually removing")
}

func runPrune(cmd *cobra.Command, args []string) error {
	if pruneOpts.olderThan == "" && pruneOpts.keep == 0 {
		return fmt.Errorf("specify --older-than or --keep")
	}

	// Get all notifications
	notifications := historyStore.All()
	if len(notifications) == 0 {
		fmt.Println("No notifications in history")
		return nil
	}

	// Sort by timestamp (newest first)
	core.Sort(notifications, core.SortOptions{
		Field: core.SortByTimestamp,
		Order: core.SortDesc,
	})

	// Determine which to remove
	var toRemove []model.Notification

	if pruneOpts.olderThan != "" {
		duration, err := core.ParseDuration(pruneOpts.olderThan)
		if err != nil {
			return fmt.Errorf("invalid duration: %w", err)
		}

		cutoff := time.Now().Add(-duration)
		for _, n := range notifications {
			if time.Unix(n.Timestamp, 0).Before(cutoff) {
				toRemove = append(toRemove, n)
			}
		}
	}

	if pruneOpts.keep > 0 && len(notifications) > pruneOpts.keep {
		// Remove the oldest ones beyond the keep limit
		keepSet := make(map[string]bool)
		for i := 0; i < pruneOpts.keep && i < len(notifications); i++ {
			keepSet[notifications[i].HistuiID] = true
		}

		for _, n := range notifications {
			if !keepSet[n.HistuiID] {
				// Avoid duplicates if also removed by older-than
				found := false
				for _, r := range toRemove {
					if r.HistuiID == n.HistuiID {
						found = true
						break
					}
				}
				if !found {
					toRemove = append(toRemove, n)
				}
			}
		}
	}

	if len(toRemove) == 0 {
		fmt.Println("No notifications to remove")
		return nil
	}

	if pruneOpts.dryRun {
		fmt.Printf("Would remove %d notification(s):\n", len(toRemove))
		for i, n := range toRemove {
			if i >= 10 {
				fmt.Printf("  ... and %d more\n", len(toRemove)-10)
				break
			}
			fmt.Printf("  - [%s] %s (%s)\n", n.AppName, n.Summary, n.RelativeTime())
		}
		return nil
	}

	// Actually remove
	removed := 0
	for _, n := range toRemove {
		if err := historyStore.Delete(n.HistuiID); err == nil {
			removed++
		}
	}

	fmt.Printf("Removed %d notification(s)\n", removed)
	return nil
}

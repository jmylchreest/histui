package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/jmylchreest/histui/internal/adapter/input"
	"github.com/jmylchreest/histui/internal/core"
)

var appsOpts struct {
	source string
}

var appsCmd = &cobra.Command{
	Use:   "apps",
	Short: "List unique application names from history",
	Long: `List all unique application names that have sent notifications.

Useful for shell completion, scripting, or discovering what apps are in your history.

Examples:
  # List all apps
  histui apps

  # Use for shell completion
  histui get --app $(histui apps | fzf)`,
	RunE: runApps,
}

func init() {
	appsCmd.Flags().StringVar(&appsOpts.source, "source", "", "notification source (histuid, dunst, stdin)")

	rootCmd.AddCommand(appsCmd)
}

func runApps(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create adapter
	adapter, err := input.NewAdapter(appsOpts.source)
	if err != nil {
		return fmt.Errorf("failed to create adapter: %w", err)
	}

	// Import notifications
	notifications, err := adapter.Import(ctx)
	if err != nil {
		return fmt.Errorf("failed to import notifications: %w", err)
	}

	// Get unique apps
	apps := core.UniqueApps(notifications)

	// Output one per line
	for _, app := range apps {
		fmt.Println(app)
	}

	return nil
}

package main

import (
	"github.com/spf13/cobra"

	"github.com/jmylchreest/histui/internal/adapter/input"
	"github.com/jmylchreest/histui/internal/tui"
)

var tuiOpts struct {
	source string
}

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch interactive TUI browser",
	Long: `Launch the interactive terminal user interface for browsing notifications.

The TUI provides:
  - Scrollable list of notifications
  - Search/filter functionality
  - Detail view with full notification content
  - Copy to clipboard support

Key bindings:
  j/k, ↑/↓    Navigate list
  enter       View notification details
  c           Copy notification body to clipboard
  s           Copy summary to clipboard
  /           Search notifications
  d           Delete notification
  r           Refresh from database
  ?           Show help
  q           Quit`,
	RunE: runTUI,
}

func init() {
	rootCmd.AddCommand(tuiCmd)

	tuiCmd.Flags().StringVar(&tuiOpts.source, "source", "",
		"Notification source (histuid, stdin; auto-detects if empty)")
}

func runTUI(cmd *cobra.Command, args []string) error {
	// Determine source and create adapter
	source := tuiOpts.source
	if source == "" {
		source = input.DetectDaemon()
	}

	var adapter input.InputAdapter
	if source != "" {
		var err error
		adapter, err = input.NewAdapter(source)
		if err != nil {
			logger.Warn("failed to create input adapter", "source", source, "error", err)
		}
	}

	return tui.Run(tui.RunOptions{
		Config:  getConfig(),
		DB:      getDB(),
		Adapter: adapter,
	})
}

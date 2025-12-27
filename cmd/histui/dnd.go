package main

import (
	"fmt"
	"os"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/jmylchreest/histui/internal/store"
)

var dndOpts struct {
	quiet bool // Suppress output, return exit code only
}

// dndCmd represents the dnd command group.
var dndCmd = &cobra.Command{
	Use:   "dnd",
	Short: "Manage Do Not Disturb mode",
	Long: `Manage Do Not Disturb (DnD) mode for histuid.

When DnD is enabled, histuid suppresses notification popups and sounds
while still persisting notifications to the history store.

Use 'histui dnd status' to check the current state.
Use 'histui dnd on' to enable DnD mode.
Use 'histui dnd off' to disable DnD mode.
Use 'histui dnd toggle' to toggle DnD mode.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Default to showing status
		return dndStatusRun(cmd, args)
	},
}

// dndOnCmd enables DnD mode.
var dndOnCmd = &cobra.Command{
	Use:   "on",
	Short: "Enable Do Not Disturb mode",
	Long:  `Enable Do Not Disturb mode. Notification popups and sounds will be suppressed.`,
	RunE:  dndOnRun,
}

// dndOffCmd disables DnD mode.
var dndOffCmd = &cobra.Command{
	Use:   "off",
	Short: "Disable Do Not Disturb mode",
	Long:  `Disable Do Not Disturb mode. Notification popups and sounds will resume.`,
	RunE:  dndOffRun,
}

// dndToggleCmd toggles DnD mode.
var dndToggleCmd = &cobra.Command{
	Use:   "toggle",
	Short: "Toggle Do Not Disturb mode",
	Long:  `Toggle Do Not Disturb mode between enabled and disabled.`,
	RunE:  dndToggleRun,
}

// dndStatusCmd shows DnD status.
var dndStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Do Not Disturb status",
	Long:  `Show whether Do Not Disturb mode is currently enabled or disabled.`,
	RunE:  dndStatusRun,
}

func init() {
	// Add subcommands
	dndCmd.AddCommand(dndOnCmd)
	dndCmd.AddCommand(dndOffCmd)
	dndCmd.AddCommand(dndToggleCmd)
	dndCmd.AddCommand(dndStatusCmd)

	// Add flags to all subcommands
	for _, cmd := range []*cobra.Command{dndCmd, dndOnCmd, dndOffCmd, dndToggleCmd, dndStatusCmd} {
		cmd.Flags().BoolVarP(&dndOpts.quiet, "quiet", "q", false,
			"Suppress output, return exit code only (0=off, 1=on)")
	}

	// Add to root
	rootCmd.AddCommand(dndCmd)
}

func dndOnRun(cmd *cobra.Command, args []string) error {
	state, err := store.LoadSharedState()
	if err != nil {
		if !dndOpts.quiet {
			fmt.Fprintf(os.Stderr, "Failed to load state: %v\n", err)
		}
		return err
	}

	state.SetDnD(true, store.DnDTriggerUser, "dnd on", "cli", "")
	if err := store.SaveSharedState(state); err != nil {
		if !dndOpts.quiet {
			fmt.Fprintf(os.Stderr, "Failed to save state: %v\n", err)
		}
		return err
	}

	if !dndOpts.quiet {
		fmt.Println("Do Not Disturb: enabled")
	}

	// Exit code 1 means DnD is now on
	os.Exit(1)
	return nil
}

func dndOffRun(cmd *cobra.Command, args []string) error {
	state, err := store.LoadSharedState()
	if err != nil {
		if !dndOpts.quiet {
			fmt.Fprintf(os.Stderr, "Failed to load state: %v\n", err)
		}
		return err
	}

	state.SetDnD(false, store.DnDTriggerUser, "dnd off", "cli", "")
	if err := store.SaveSharedState(state); err != nil {
		if !dndOpts.quiet {
			fmt.Fprintf(os.Stderr, "Failed to save state: %v\n", err)
		}
		return err
	}

	if !dndOpts.quiet {
		fmt.Println("Do Not Disturb: disabled")
	}

	// Exit code 0 means DnD is now off
	return nil
}

func dndToggleRun(cmd *cobra.Command, args []string) error {
	state, err := store.LoadSharedState()
	if err != nil {
		if !dndOpts.quiet {
			fmt.Fprintf(os.Stderr, "Failed to load state: %v\n", err)
		}
		return err
	}

	newEnabled := state.ToggleDnD(store.DnDTriggerUser, "dnd toggle", "cli", "")
	if err := store.SaveSharedState(state); err != nil {
		if !dndOpts.quiet {
			fmt.Fprintf(os.Stderr, "Failed to save state: %v\n", err)
		}
		return err
	}

	if !dndOpts.quiet {
		if newEnabled {
			fmt.Println("Do Not Disturb: enabled")
		} else {
			fmt.Println("Do Not Disturb: disabled")
		}
	}

	// Exit code: 0=off, 1=on
	if newEnabled {
		os.Exit(1)
	}
	return nil
}

func dndStatusRun(cmd *cobra.Command, args []string) error {
	state, err := store.LoadSharedState()
	if err != nil {
		if !dndOpts.quiet {
			fmt.Fprintf(os.Stderr, "Failed to load state: %v\n", err)
		}
		return err
	}

	if !dndOpts.quiet {
		if state.DnDEnabled {
			fmt.Println("Do Not Disturb: enabled")
		} else {
			fmt.Println("Do Not Disturb: disabled")
		}

		// Show enhanced transition info if available
		if state.DnDLastTransition != nil {
			t := state.DnDLastTransition
			fmt.Printf("  Last change: %s\n", formatTransitionTime(t.Timestamp))
			fmt.Printf("  Trigger: %s\n", t.Trigger)
			if t.Reason != "" {
				fmt.Printf("  Reason: %s\n", t.Reason)
			}
			if t.Source != "" {
				fmt.Printf("  Source: %s\n", t.Source)
			}
			if t.RuleName != "" {
				fmt.Printf("  Rule: %s\n", t.RuleName)
			}
		} else if state.DnDEnabledBy != "" {
			// Fallback to legacy field
			fmt.Printf("  Enabled by: %s\n", state.DnDEnabledBy)
		}
	}

	// Exit code: 0=off, 1=on
	if state.DnDEnabled {
		os.Exit(1)
	}
	return nil
}

// formatTransitionTime formats a unix timestamp as a human-readable relative time.
func formatTransitionTime(timestamp int64) string {
	return humanize.Time(time.Unix(timestamp, 0))
}

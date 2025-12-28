package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jmylchreest/histui/internal/ipc"
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
while still persisting notifications to the database.

Use 'histui dnd status' to check the current state.
Use 'histui dnd on' to enable DnD mode.
Use 'histui dnd off' to disable DnD mode.
Use 'histui dnd toggle' to toggle DnD mode.

Note: DnD commands communicate with the running histuid daemon via IPC.
If the daemon is not running, these commands have no effect.`,
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
	client, err := ipc.NewClient()
	if err != nil {
		if !dndOpts.quiet {
			fmt.Fprintf(os.Stderr, "Failed to create IPC client: %v\n", err)
		}
		return err
	}

	if err := client.SetDnD(true); err != nil {
		if !dndOpts.quiet {
			fmt.Fprintf(os.Stderr, "Failed to set DnD: %v\n", err)
		}
		return err
	}

	if !dndOpts.quiet {
		if client.IsRunning() {
			fmt.Println("Do Not Disturb: enabled")
		} else {
			fmt.Println("Do Not Disturb: enabled (daemon not running)")
		}
	}

	// Exit code 1 means DnD is now on
	os.Exit(1)
	return nil
}

func dndOffRun(cmd *cobra.Command, args []string) error {
	client, err := ipc.NewClient()
	if err != nil {
		if !dndOpts.quiet {
			fmt.Fprintf(os.Stderr, "Failed to create IPC client: %v\n", err)
		}
		return err
	}

	if err := client.SetDnD(false); err != nil {
		if !dndOpts.quiet {
			fmt.Fprintf(os.Stderr, "Failed to set DnD: %v\n", err)
		}
		return err
	}

	if !dndOpts.quiet {
		if client.IsRunning() {
			fmt.Println("Do Not Disturb: disabled")
		} else {
			fmt.Println("Do Not Disturb: disabled (daemon not running)")
		}
	}

	// Exit code 0 means DnD is now off
	return nil
}

func dndToggleRun(cmd *cobra.Command, args []string) error {
	client, err := ipc.NewClient()
	if err != nil {
		if !dndOpts.quiet {
			fmt.Fprintf(os.Stderr, "Failed to create IPC client: %v\n", err)
		}
		return err
	}

	newEnabled, err := client.ToggleDnD()
	if err != nil {
		if !dndOpts.quiet {
			fmt.Fprintf(os.Stderr, "Failed to toggle DnD: %v\n", err)
		}
		return err
	}

	if !dndOpts.quiet {
		if newEnabled {
			fmt.Println("Do Not Disturb: enabled")
		} else {
			fmt.Println("Do Not Disturb: disabled")
		}
		if !client.IsRunning() {
			fmt.Println("  (daemon not running)")
		}
	}

	// Exit code: 0=off, 1=on
	if newEnabled {
		os.Exit(1)
	}
	return nil
}

func dndStatusRun(cmd *cobra.Command, args []string) error {
	client, err := ipc.NewClient()
	if err != nil {
		if !dndOpts.quiet {
			fmt.Fprintf(os.Stderr, "Failed to create IPC client: %v\n", err)
		}
		return err
	}

	enabled, err := client.GetDnD()
	if err != nil {
		if !dndOpts.quiet {
			fmt.Fprintf(os.Stderr, "Failed to get DnD status: %v\n", err)
		}
		return err
	}

	if !dndOpts.quiet {
		if enabled {
			fmt.Println("Do Not Disturb: enabled")
		} else {
			fmt.Println("Do Not Disturb: disabled")
		}
		if !client.IsRunning() {
			fmt.Println("  (daemon not running)")
		}
	}

	// Exit code: 0=off, 1=on
	if enabled {
		os.Exit(1)
	}
	return nil
}

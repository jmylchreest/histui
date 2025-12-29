package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jmylchreest/histui/internal/dbus"
)

var audioOpts struct {
	quiet bool // Suppress output
}

// audioCmd represents the audio command group.
var audioCmd = &cobra.Command{
	Use:   "audio",
	Short: "Control audio playback",
	Long: `Control audio playback for histuid.

Use 'histui audio stop' to stop any currently playing notification sound.`,
}

// audioStopCmd stops audio playback.
var audioStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop audio playback",
	Long:  `Stop any currently playing notification sound.`,
	RunE:  audioStopRun,
}

func init() {
	// Add subcommands
	audioCmd.AddCommand(audioStopCmd)

	// Add flags
	audioStopCmd.Flags().BoolVarP(&audioOpts.quiet, "quiet", "q", false,
		"Suppress output")

	// Add to root
	rootCmd.AddCommand(audioCmd)
}

func audioStopRun(cmd *cobra.Command, args []string) error {
	client := dbus.NewDaemonClient(nil)
	defer func() { _ = client.Close() }()

	if err := client.StopAudio(); err != nil {
		if !audioOpts.quiet {
			fmt.Fprintf(os.Stderr, "Failed to stop audio: %v\n", err)
		}
		return err
	}

	if !audioOpts.quiet {
		if client.IsAvailable() {
			fmt.Println("Audio stop requested")
		} else {
			fmt.Println("Audio stop requested (daemon not running)")
		}
	}

	return nil
}

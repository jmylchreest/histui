package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jmylchreest/histui/internal/store"
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
	state, err := store.LoadSharedState()
	if err != nil {
		if !audioOpts.quiet {
			fmt.Fprintf(os.Stderr, "Failed to load state: %v\n", err)
		}
		return err
	}

	state.RequestStopAudio()
	if err := store.SaveSharedState(state); err != nil {
		if !audioOpts.quiet {
			fmt.Fprintf(os.Stderr, "Failed to save state: %v\n", err)
		}
		return err
	}

	if !audioOpts.quiet {
		fmt.Println("Audio stop requested")
	}

	return nil
}

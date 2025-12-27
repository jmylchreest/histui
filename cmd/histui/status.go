package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/jmylchreest/histui/internal/adapter/input"
)

var statusOpts struct {
	source  string
	since   string
	urgency string
	all     bool // Include history (acknowledged) notifications
}

// WaybarStatus represents the Waybar custom module JSON format.
type WaybarStatus struct {
	Text       string `json:"text"`
	Alt        string `json:"alt,omitempty"`
	Tooltip    string `json:"tooltip,omitempty"`
	Class      string `json:"class,omitempty"`
	Percentage int    `json:"percentage,omitempty"`
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Output Waybar-compatible JSON status",
	Long: `Output notification status in Waybar's custom module JSON format.

By default, shows only ACTIVE (unacknowledged) notifications - those currently
displayed or waiting to be displayed. Use --all to include history.

This is designed to be used with Waybar's custom module:

  "custom/notifications": {
    "exec": "histui status",
    "interval": 5,
    "return-type": "json",
    "on-click": "histui tui"
  }

The output includes:
  - text: Number of active notifications
  - alt: Urgency class (low, normal, critical, empty)
  - tooltip: Breakdown by type (displayed/waiting/history)
  - class: CSS class based on urgency level`,
	RunE: runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)

	statusCmd.Flags().StringVar(&statusOpts.source, "source", "",
		"Notification source (dunst; auto-detects if empty)")
	statusCmd.Flags().BoolVar(&statusOpts.all, "all", false,
		"Include history (acknowledged) notifications in count")
	statusCmd.Flags().StringVar(&statusOpts.since, "since", "",
		"Only count notifications from the last duration (for --all)")
	statusCmd.Flags().StringVar(&statusOpts.urgency, "urgency", "",
		"Only count notifications of this urgency level")
}

func runStatus(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Currently only dunst is supported for status
	adapter := input.NewDunstAdapter()

	// Get counts
	counts, err := adapter.GetCounts(ctx)
	if err != nil {
		return outputStatus(WaybarStatus{Text: "", Alt: "error", Class: "error"})
	}

	// Calculate active count (displayed + waiting)
	_ = counts.Displayed + counts.Waiting // activeCount used in generateStatusFromCounts

	// Generate status
	status := generateStatusFromCounts(counts, statusOpts.all)
	return outputStatus(status)
}

// generateStatusFromCounts creates a WaybarStatus from dunst counts.
func generateStatusFromCounts(counts *input.DunstCounts, includeHistory bool) WaybarStatus {
	activeCount := counts.Displayed + counts.Waiting

	// Determine what to show
	displayCount := activeCount
	if includeHistory {
		displayCount += counts.History
	}

	if displayCount == 0 {
		return WaybarStatus{
			Text:  "",
			Alt:   "empty",
			Class: "empty",
		}
	}

	// Determine urgency class based on whether there are active notifications
	// Active notifications are more urgent than just history
	urgencyClass := "normal"
	if activeCount == 0 {
		urgencyClass = "low" // Only history, already acknowledged
	} else if counts.Displayed > 0 {
		urgencyClass = "critical" // Notifications currently on screen
	}

	// Build tooltip with breakdown
	tooltip := buildCountsTooltip(counts, includeHistory)

	// Text: show active count (or total if --all)
	text := fmt.Sprintf("%d", displayCount)

	return WaybarStatus{
		Text:       text,
		Alt:        urgencyClass,
		Tooltip:    tooltip,
		Class:      urgencyClass,
		Percentage: min(displayCount, 100),
	}
}

// buildCountsTooltip creates a tooltip showing notification breakdown.
func buildCountsTooltip(counts *input.DunstCounts, includeHistory bool) string {
	var lines []string

	if counts.Displayed > 0 {
		lines = append(lines, fmt.Sprintf("Displayed: %d", counts.Displayed))
	}
	if counts.Waiting > 0 {
		lines = append(lines, fmt.Sprintf("Waiting: %d", counts.Waiting))
	}
	if includeHistory && counts.History > 0 {
		lines = append(lines, fmt.Sprintf("History: %d", counts.History))
	}

	if len(lines) == 0 {
		return "No notifications"
	}

	activeCount := counts.Displayed + counts.Waiting
	if activeCount > 0 {
		return fmt.Sprintf("%d active\n%s", activeCount, joinLines(lines))
	}

	return joinLines(lines)
}

func joinLines(lines []string) string {
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}

// outputStatus writes the status as JSON.
func outputStatus(status WaybarStatus) error {
	encoder := json.NewEncoder(os.Stdout)
	return encoder.Encode(status)
}

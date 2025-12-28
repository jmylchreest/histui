package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jmylchreest/histui/internal/adapter/input"
	"github.com/jmylchreest/histui/internal/ipc"
)

var statusOpts struct {
	source   string
	since    string
	urgency  string
	all      bool // Include history (acknowledged) notifications
	detailed bool // Output detailed status format
}

// WaybarStatus represents the Waybar custom module JSON format.
type WaybarStatus struct {
	Text       string `json:"text"`
	Alt        string `json:"alt,omitempty"`
	Tooltip    string `json:"tooltip,omitempty"`
	Class      string `json:"class,omitempty"`
	Percentage int    `json:"percentage,omitempty"`
}

// NotificationCounts holds notification counts from any daemon.
type NotificationCounts struct {
	Displayed      int    // Currently visible (seen but not dismissed)
	History        int    // Dismissed, in history
	Waiting        int    // Queued, waiting to be displayed
	HighestUrgency string // Highest urgency among active notifications
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
		"Notification source (histuid; auto-detects if empty)")
	statusCmd.Flags().BoolVar(&statusOpts.all, "all", false,
		"Include history (acknowledged) notifications in count")
	statusCmd.Flags().StringVar(&statusOpts.since, "since", "",
		"Only count notifications from the last duration (for --all)")
	statusCmd.Flags().StringVar(&statusOpts.urgency, "urgency", "",
		"Only count notifications of this urgency level")
	statusCmd.Flags().BoolVar(&statusOpts.detailed, "detailed", false,
		"Output detailed status with urgency breakdown")
}

func runStatus(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Load DnD state via IPC
	dndEnabled := false
	if client, err := ipc.NewClient(); err == nil {
		dndEnabled, _ = client.GetDnD()
	}

	// Detect which daemon to use
	source := statusOpts.source
	if source == "" {
		source = input.DetectDaemon()
	}

	// Use detailed output for histuid when --detailed flag is set
	if statusOpts.detailed && source == "histuid" {
		adapter := input.NewHistuidAdapter("")
		detailed, err := adapter.GetDetailedCounts(ctx)
		if err != nil {
			return outputStatus(WaybarStatus{Text: "", Alt: "error", Class: "error"})
		}
		detailed.DnDEnabled = dndEnabled
		status := generateDetailedStatus(detailed)
		return outputStatus(status)
	}

	var counts *NotificationCounts

	switch source {
	case "histuid":
		adapter := input.NewHistuidAdapter("")
		histuidCounts, err := adapter.GetCounts(ctx)
		if err != nil {
			return outputStatus(WaybarStatus{Text: "", Alt: "error", Class: "error"})
		}
		// Get highest urgency for active notifications
		urgency, _ := adapter.GetHighestActiveUrgency(ctx)
		counts = &NotificationCounts{
			Displayed:      histuidCounts.Displayed,
			History:        histuidCounts.History,
			Waiting:        histuidCounts.Waiting,
			HighestUrgency: urgency,
		}
	default:
		return outputStatus(WaybarStatus{Text: "", Alt: "error", Tooltip: "No notification daemon detected", Class: "error"})
	}

	// Generate status
	status := generateStatusFromCounts(counts, statusOpts.all, dndEnabled)
	return outputStatus(status)
}

// generateStatusFromCounts creates a WaybarStatus from notification counts.
// The output is designed for Waybar custom modules with return-type: json.
// Use format: "{icon} {text}" in Waybar config with format-icons for icons.
func generateStatusFromCounts(counts *NotificationCounts, includeHistory bool, dndEnabled bool) WaybarStatus {
	activeCount := counts.Displayed + counts.Waiting

	// Determine what to show
	displayCount := activeCount
	if includeHistory {
		displayCount += counts.History
	}

	// If DnD is enabled, adjust the class and alt
	if dndEnabled {
		tooltip := "Do Not Disturb: enabled"
		text := ""
		if displayCount > 0 {
			tooltip += fmt.Sprintf("\n%d notification(s) suppressed", displayCount)
			text = fmt.Sprintf("%d", displayCount)
		}
		return WaybarStatus{
			Text:    text,
			Alt:     "dnd",
			Tooltip: tooltip,
			Class:   "dnd",
		}
	}

	if displayCount == 0 {
		return WaybarStatus{
			Text:  "",
			Alt:   "empty",
			Class: "empty",
		}
	}

	// Determine urgency class based on highest active urgency
	urgencyClass := counts.HighestUrgency
	if urgencyClass == "" || urgencyClass == "empty" {
		if activeCount == 0 {
			urgencyClass = "low" // Only history, already acknowledged
		} else {
			urgencyClass = "normal"
		}
	}

	// Build tooltip with breakdown
	tooltip := buildCountsTooltip(counts, includeHistory)

	// Build class string - include both has-notifications and urgency
	class := "has-notifications"
	if urgencyClass != "" {
		class += " " + urgencyClass
	}

	// Text: just the count, icon comes from Waybar format-icons
	text := fmt.Sprintf("%d", displayCount)

	return WaybarStatus{
		Text:       text,
		Alt:        urgencyClass,
		Tooltip:    tooltip,
		Class:      class,
		Percentage: min(displayCount, 100),
	}
}

// buildCountsTooltip creates a tooltip showing notification breakdown.
func buildCountsTooltip(counts *NotificationCounts, includeHistory bool) string {
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
	return strings.Join(lines, "\n")
}

// outputStatus writes the status as JSON.
func outputStatus(status WaybarStatus) error {
	encoder := json.NewEncoder(os.Stdout)
	return encoder.Encode(status)
}

// generateDetailedStatus creates a WaybarStatus with detailed breakdown.
// Format:
//
//	DnD: enabled/disabled
//	Pending: X
//	  Critical: Y
//	  Normal: Y
//	  Low: Y
//	Missed: X
//	  Critical: Y
//	  Normal: Y
//	  Low: Y
//	Dismissed: X
//	Tracked: X
func generateDetailedStatus(counts *input.DetailedCounts) WaybarStatus {
	// Build tooltip with detailed breakdown
	var tooltip strings.Builder

	// DnD status
	if counts.DnDEnabled {
		tooltip.WriteString("DnD: enabled\n\n")
	} else {
		tooltip.WriteString("DnD: disabled\n\n")
	}

	// Pending (currently visible)
	tooltip.WriteString("Pending: ")
	tooltip.WriteString(strconv.Itoa(counts.Pending))
	tooltip.WriteString("\n")
	if counts.Pending > 0 {
		tooltip.WriteString("  Critical: ")
		tooltip.WriteString(strconv.Itoa(counts.PendingCritical))
		tooltip.WriteString("\n  Normal: ")
		tooltip.WriteString(strconv.Itoa(counts.PendingNormal))
		tooltip.WriteString("\n  Low: ")
		tooltip.WriteString(strconv.Itoa(counts.PendingLow))
		tooltip.WriteString("\n")
	}

	// Missed (not seen, not dismissed)
	tooltip.WriteString("Missed: ")
	tooltip.WriteString(strconv.Itoa(counts.Missed))
	tooltip.WriteString("\n")
	if counts.Missed > 0 {
		tooltip.WriteString("  Critical: ")
		tooltip.WriteString(strconv.Itoa(counts.MissedCritical))
		tooltip.WriteString("\n  Normal: ")
		tooltip.WriteString(strconv.Itoa(counts.MissedNormal))
		tooltip.WriteString("\n  Low: ")
		tooltip.WriteString(strconv.Itoa(counts.MissedLow))
		tooltip.WriteString("\n")
	}

	// Dismissed and Tracked
	tooltip.WriteString("Dismissed: ")
	tooltip.WriteString(strconv.Itoa(counts.Dismissed))
	tooltip.WriteString("\nTracked: ")
	tooltip.WriteString(strconv.Itoa(counts.Tracked))

	// Top apps section
	if len(counts.TopApps) > 0 {
		tooltip.WriteString("\n\nTop 5 Applications:")
		for i, app := range counts.TopApps {
			tooltip.WriteString("\n  ")
			tooltip.WriteString(strconv.Itoa(i + 1))
			tooltip.WriteString(". ")
			tooltip.WriteString(app.AppName)
			tooltip.WriteString(": ")
			tooltip.WriteString(strconv.Itoa(app.Count))
		}
	}

	// Determine urgency class based on pending + missed
	// Priority: critical > normal > low
	urgencyClass := "empty"
	if counts.PendingCritical > 0 || counts.MissedCritical > 0 {
		urgencyClass = "critical"
	} else if counts.PendingNormal > 0 || counts.MissedNormal > 0 {
		urgencyClass = "normal"
	} else if counts.PendingLow > 0 || counts.MissedLow > 0 {
		urgencyClass = "low"
	}

	// If DnD is enabled, use dnd class
	if counts.DnDEnabled {
		urgencyClass = "dnd"
	}

	// Text is the count of active (pending + missed)
	activeCount := counts.Pending + counts.Missed
	text := ""
	if activeCount > 0 {
		text = strconv.Itoa(activeCount)
	}

	// Build class string
	class := urgencyClass
	if activeCount > 0 {
		class = "has-notifications " + urgencyClass
	}

	return WaybarStatus{
		Text:       text,
		Alt:        urgencyClass,
		Tooltip:    tooltip.String(),
		Class:      class,
		Percentage: min(activeCount, 100),
	}
}

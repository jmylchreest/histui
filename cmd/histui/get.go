package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jmylchreest/histui/internal/adapter/input"
	"github.com/jmylchreest/histui/internal/adapter/output"
	"github.com/jmylchreest/histui/internal/core"
	"github.com/jmylchreest/histui/internal/model"
)

var getOpts struct {
	// Input options
	source string

	// Filter options
	since   string
	app     string
	urgency string
	limit   int
	search  string

	// Sort options
	sortBy    string
	sortOrder string

	// Output options
	format   string
	field    string
	template string

	// Lookup options
	index int
	id    string
}

var getCmd = &cobra.Command{
	Use:   "get [index|id]",
	Short: "Query and output notification history",
	Long: `Query notification history and output in various formats.

Without arguments, outputs all notifications in dmenu format (suitable for
fuzzel, walker, rofi, etc.).

With an index (1-based) or ID argument, outputs that specific notification.

Examples:
  # List all notifications in dmenu format
  histui get

  # Filter by app and time
  histui get --app firefox --since 1h

  # Get specific notification by index
  histui get 3

  # Get notification and output body field
  histui get 3 --field body

  # Output as JSON
  histui get --format json

  # Use with fuzzel for clipboard workflow
  histui get | fuzzel -d | histui get --field body | wl-copy`,
	RunE: runGet,
}

func init() {
	rootCmd.AddCommand(getCmd)

	// Input flags
	getCmd.Flags().StringVar(&getOpts.source, "source", "",
		"Notification source (dunst, stdin; auto-detects if empty)")

	// Filter flags
	getCmd.Flags().StringVar(&getOpts.since, "since", "",
		"Show notifications from the last duration (e.g., 1h, 7d, 1w)")
	getCmd.Flags().StringVar(&getOpts.app, "app", "",
		"Filter by application name (exact match)")
	getCmd.Flags().StringVar(&getOpts.urgency, "urgency", "",
		"Filter by urgency (low, normal, critical)")
	getCmd.Flags().IntVarP(&getOpts.limit, "limit", "n", 0,
		"Maximum number of notifications to show (0=unlimited)")
	getCmd.Flags().StringVarP(&getOpts.search, "search", "s", "",
		"Search in summary and body")

	// Sort flags
	getCmd.Flags().StringVar(&getOpts.sortBy, "sort", "timestamp",
		"Sort by field (timestamp, app, urgency)")
	getCmd.Flags().StringVar(&getOpts.sortOrder, "order", "desc",
		"Sort order (asc, desc)")

	// Output flags
	getCmd.Flags().StringVarP(&getOpts.format, "format", "f", "dmenu",
		"Output format (dmenu, json, plain)")
	getCmd.Flags().StringVar(&getOpts.field, "field", "",
		"Output single field from notification (id, app, summary, body, all)")
	getCmd.Flags().StringVar(&getOpts.template, "template", "",
		"Custom Go template for output formatting")

	// Lookup flags
	getCmd.Flags().IntVar(&getOpts.index, "index", 0,
		"Lookup notification by 1-based index")
	getCmd.Flags().StringVar(&getOpts.id, "id", "",
		"Lookup notification by histui ID")
}

func runGet(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check for positional argument (index or ID)
	if len(args) > 0 {
		arg := args[0]
		// Try as index first
		if idx, err := strconv.Atoi(arg); err == nil && idx > 0 {
			getOpts.index = idx
		} else {
			// Treat as ID
			getOpts.id = arg
		}
	}

	// Fetch notifications
	notifications, err := fetchNotifications(ctx)
	if err != nil {
		return err
	}

	// If looking up specific notification
	if getOpts.index > 0 || getOpts.id != "" {
		return handleLookup(notifications)
	}

	// Apply filters and sort
	notifications = applyFilters(notifications)
	applySort(notifications)

	// Output
	return outputNotifications(notifications)
}

// fetchNotifications retrieves notifications from the configured source.
func fetchNotifications(ctx context.Context) ([]model.Notification, error) {
	// Determine source
	source := getOpts.source
	if source == "" {
		source = input.DetectDaemon()
		if source == "" {
			return nil, fmt.Errorf("no notification daemon detected; specify --source")
		}
	}

	logger.Debug("fetching notifications", "source", source)

	adapter, err := input.NewAdapter(source)
	if err != nil {
		return nil, fmt.Errorf("failed to create adapter: %w", err)
	}

	notifications, err := adapter.Import(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to import notifications: %w", err)
	}

	logger.Debug("fetched notifications", "count", len(notifications))

	// Add to store (persistence is always enabled)
	if len(notifications) > 0 {
		_ = historyStore.AddBatch(notifications)
	}

	return notifications, nil
}

// applyFilters applies filter options to notifications.
func applyFilters(notifications []model.Notification) []model.Notification {
	opts := core.FilterOptions{
		AppFilter: getOpts.app,
		Limit:     getOpts.limit,
	}

	// Parse since duration
	if getOpts.since != "" {
		d, err := core.ParseDuration(getOpts.since)
		if err != nil {
			logger.Warn("invalid since duration", "value", getOpts.since, "error", err)
		} else {
			opts.Since = d
		}
	}

	// Parse urgency
	if getOpts.urgency != "" {
		u, err := core.ParseUrgency(getOpts.urgency)
		if err != nil {
			logger.Warn("invalid urgency", "value", getOpts.urgency, "error", err)
		} else {
			opts.Urgency = &u
		}
	}

	// Apply filter
	notifications = core.Filter(notifications, opts)

	// Apply search if specified
	if getOpts.search != "" {
		notifications = core.Search(notifications, getOpts.search)
	}

	return notifications
}

// applySort sorts notifications based on options.
func applySort(notifications []model.Notification) {
	field, _ := core.ParseSortField(getOpts.sortBy)
	order, _ := core.ParseSortOrder(getOpts.sortOrder)

	core.Sort(notifications, core.SortOptions{
		Field: field,
		Order: order,
	})
}

// handleLookup handles single notification lookup and output.
func handleLookup(notifications []model.Notification) error {
	var n *model.Notification

	if getOpts.index > 0 {
		// First apply filters and sort to get consistent indexing
		notifications = applyFilters(notifications)
		applySort(notifications)
		n = core.LookupByIndex(notifications, getOpts.index)
		if n == nil {
			return fmt.Errorf("notification at index %d not found", getOpts.index)
		}
	} else if getOpts.id != "" {
		// Parse ID from potential dmenu output (first field before separator)
		id := parseDmenuSelection(getOpts.id)
		n = core.LookupByID(notifications, id)
		if n == nil {
			return fmt.Errorf("notification with ID %s not found", getOpts.id)
		}
	}

	// Output specific field if requested
	if getOpts.field != "" {
		fmt.Println(output.FormatField(n, getOpts.field))
		return nil
	}

	// Output as JSON by default for single notification
	if getOpts.format == "dmenu" {
		getOpts.format = "json"
	}

	formatter := createFormatter()
	return formatter.Format(os.Stdout, []model.Notification{*n})
}

// parseDmenuSelection extracts the notification ID from dmenu selection.
// Input could be the full line: "1 | 5m | Firefox | Download Complete: file.zip"
// or just an ID/index.
func parseDmenuSelection(selection string) string {
	selection = strings.TrimSpace(selection)

	// If it looks like a raw ID (alphanumeric), return as-is
	if !strings.Contains(selection, " ") && !strings.Contains(selection, "|") {
		return selection
	}

	// Try to parse as index from dmenu output
	// Format: "index | time | app | summary"
	parts := strings.SplitN(selection, "|", 2)
	if len(parts) > 0 {
		idxStr := strings.TrimSpace(parts[0])
		if idx, err := strconv.Atoi(idxStr); err == nil && idx > 0 {
			// Return as string index for lookup
			return idxStr
		}
	}

	return selection
}

// outputNotifications outputs the notification list.
func outputNotifications(notifications []model.Notification) error {
	if len(notifications) == 0 {
		logger.Debug("no notifications to output")
		return nil
	}

	formatter := createFormatter()
	return formatter.Format(os.Stdout, notifications)
}

// createFormatter creates the output formatter based on options.
func createFormatter() output.Formatter {
	var format output.FormatType
	switch strings.ToLower(getOpts.format) {
	case "json":
		format = output.FormatJSON
	case "plain":
		format = output.FormatPlain
	default:
		format = output.FormatDmenu
	}

	opts := output.DefaultFormatterOptions()
	opts.Template = getOpts.template

	// Apply config defaults if available
	if cfg != nil {
		if cfg.Templates.Dmenu != "" && opts.Template == "" {
			opts.Template = cfg.Templates.Dmenu
		}
	}

	return output.NewFormatter(format, opts)
}

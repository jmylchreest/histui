package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/jmylchreest/histui/internal/adapter/input"
	"github.com/jmylchreest/histui/internal/core"
	"github.com/jmylchreest/histui/internal/dbus"
	"github.com/jmylchreest/histui/internal/model"
	"github.com/jmylchreest/histui/internal/store"
)

var replayOpts struct {
	// Lookup options
	index int
	id    string
	stdin bool // Read from stdin

	// Display options
	timeout int // Custom timeout in milliseconds (-1 = server default)

	// Filter for batch replay
	since   string
	app     string
	urgency string
	limit   int
	all     bool
}

var replayCmd = &cobra.Command{
	Use:   "replay [index|id]",
	Short: "Replay a notification through D-Bus",
	Long: `Replay a historical notification through the active notification daemon.

The notification is sent to whatever notification daemon is currently running
(histuid, dunst, mako, etc.) with custom hints to identify it as a replay.

If histuid is running, it will:
- Mark the original notification as replayed (R indicator in TUI)
- Not create a duplicate history entry
- Preserve the original notification's images and metadata

When stdin is a pipe and no arguments are provided, the command automatically
reads a notification selection from stdin. This enables clean pipe workflows:

  histui get | fuzzel -d | histui replay

Examples:
  # Replay the most recent notification
  histui replay 1

  # Replay by notification ID
  histui replay 01HXYZ123...

  # Replay from pipe (auto-detected)
  histui get | fuzzel -d | histui replay
  histui get | rofi -dmenu | histui replay
  histui get | fzf | histui replay

  # Replay with custom timeout
  histui replay 1 --timeout 10000

  # Replay all notifications from the last hour
  histui replay --since 1h --all`,
	Args: cobra.MaximumNArgs(1),
	RunE: runReplay,
}

func init() {
	rootCmd.AddCommand(replayCmd)

	// Lookup flags
	replayCmd.Flags().IntVar(&replayOpts.index, "index", 0,
		"Lookup notification by 1-based index")
	replayCmd.Flags().StringVar(&replayOpts.id, "id", "",
		"Lookup notification by histui ID")
	replayCmd.Flags().BoolVar(&replayOpts.stdin, "stdin", false,
		"Read notification selection from stdin (for pipe workflows)")

	// Display flags
	replayCmd.Flags().IntVar(&replayOpts.timeout, "timeout", -1,
		"Custom timeout in milliseconds (-1 = server default)")

	// Batch replay flags
	replayCmd.Flags().StringVar(&replayOpts.since, "since", "",
		"Replay notifications from the last duration (e.g., 1h, 7d)")
	replayCmd.Flags().StringVar(&replayOpts.app, "app", "",
		"Filter by application name")
	replayCmd.Flags().StringVar(&replayOpts.urgency, "urgency", "",
		"Filter by urgency (low, normal, critical)")
	replayCmd.Flags().IntVar(&replayOpts.limit, "limit", 10,
		"Maximum notifications to replay (default 10)")
	replayCmd.Flags().BoolVar(&replayOpts.all, "all", false,
		"Replay all matching notifications (required for batch)")
}

func runReplay(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Auto-detect stdin: if stdin is a pipe (not TTY) and no args, read from it
	// Also read if --stdin was explicitly set
	stdinIsPipe := !isatty.IsTerminal(os.Stdin.Fd()) && !isatty.IsCygwinTerminal(os.Stdin.Fd())
	shouldReadStdin := replayOpts.stdin || (stdinIsPipe && len(args) == 0 && replayOpts.index == 0 && replayOpts.id == "")

	if shouldReadStdin {
		selection, err := readReplayStdin()
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
		if selection == "" {
			// If nothing on stdin, that's OK - maybe user just wants to use filters
			if !stdinIsPipe {
				return fmt.Errorf("no selection received from stdin")
			}
		} else {
			// Parse the selection as if it were a positional argument
			selection = parseDmenuID(selection)
			if idx, err := strconv.Atoi(selection); err == nil && idx > 0 {
				replayOpts.index = idx
			} else {
				replayOpts.id = selection
			}
		}
	}

	// Parse positional argument
	if len(args) > 0 {
		arg := args[0]
		// Handle dmenu-style output
		arg = parseDmenuID(arg)
		// Try as index first
		if idx, err := strconv.Atoi(arg); err == nil && idx > 0 {
			replayOpts.index = idx
		} else {
			replayOpts.id = arg
		}
	}

	// Validate options
	hasLookup := replayOpts.index > 0 || replayOpts.id != ""
	hasFilter := replayOpts.since != "" || replayOpts.app != "" || replayOpts.urgency != ""

	if !hasLookup && !hasFilter {
		return fmt.Errorf("specify a notification index/ID or use filter flags with --all")
	}

	if hasFilter && !replayOpts.all && !hasLookup {
		return fmt.Errorf("use --all flag for batch replay with filters")
	}

	// Fetch notifications
	notifications, err := fetchReplayNotifications(ctx)
	if err != nil {
		return err
	}

	if len(notifications) == 0 {
		return fmt.Errorf("no notifications found")
	}

	// Create replayer
	var imageStore *store.ImageStore
	if imagePath, err := store.DefaultImageStorePath(); err == nil {
		imageStore, _ = store.NewImageStore(imagePath)
	}

	replayer, err := dbus.NewReplayer(imageStore)
	if err != nil {
		return fmt.Errorf("failed to create replayer: %w", err)
	}
	defer replayer.Close()

	// Replay single notification
	if hasLookup {
		n, err := lookupReplayNotification(notifications)
		if err != nil {
			return err
		}
		return replayNotification(replayer, n)
	}

	// Batch replay with filters
	notifications = applyReplayFilters(notifications)
	if len(notifications) == 0 {
		logger.Info("no notifications match the filter")
		return nil
	}

	// Apply limit
	if replayOpts.limit > 0 && len(notifications) > replayOpts.limit {
		notifications = notifications[:replayOpts.limit]
	}

	// Replay in reverse order (oldest first)
	for i := len(notifications) - 1; i >= 0; i-- {
		n := &notifications[i]
		if err := replayNotification(replayer, n); err != nil {
			logger.Warn("failed to replay notification", "id", n.HistuiID, "error", err)
		}
		// Small delay between notifications
		if i > 0 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	fmt.Printf("Replayed %d notification(s)\n", len(notifications))
	return nil
}

// fetchReplayNotifications loads notifications from the store.
func fetchReplayNotifications(ctx context.Context) ([]model.Notification, error) {
	adapter, err := input.NewAdapter(input.DetectDaemon())
	if err != nil {
		return nil, fmt.Errorf("failed to create adapter: %w", err)
	}

	notifications, err := adapter.Import(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load notifications: %w", err)
	}

	return notifications, nil
}

// lookupReplayNotification finds a single notification by index or ID.
func lookupReplayNotification(notifications []model.Notification) (*model.Notification, error) {
	// Sort by timestamp desc for consistent indexing
	core.Sort(notifications, core.SortOptions{
		Field: core.SortByTimestamp,
		Order: core.SortDesc,
	})

	if replayOpts.index > 0 {
		n := core.LookupByIndex(notifications, replayOpts.index)
		if n == nil {
			return nil, fmt.Errorf("notification at index %d not found", replayOpts.index)
		}
		return n, nil
	}

	if replayOpts.id != "" {
		n := core.LookupByID(notifications, replayOpts.id)
		if n == nil {
			return nil, fmt.Errorf("notification with ID %s not found", replayOpts.id)
		}
		return n, nil
	}

	return nil, fmt.Errorf("no notification lookup specified")
}

// applyReplayFilters filters notifications for batch replay.
func applyReplayFilters(notifications []model.Notification) []model.Notification {
	opts := core.FilterOptions{
		AppFilter: replayOpts.app,
	}

	if replayOpts.since != "" {
		d, err := core.ParseDuration(replayOpts.since)
		if err == nil {
			opts.Since = d
		}
	}

	if replayOpts.urgency != "" {
		u, err := core.ParseUrgency(replayOpts.urgency)
		if err == nil {
			opts.Urgency = &u
		}
	}

	// Apply filter
	notifications = core.Filter(notifications, opts)

	// Sort by timestamp desc
	core.Sort(notifications, core.SortOptions{
		Field: core.SortByTimestamp,
		Order: core.SortDesc,
	})

	return notifications
}

// replayNotification sends a single notification to the D-Bus daemon.
func replayNotification(replayer *dbus.Replayer, n *model.Notification) error {
	var id uint32
	var err error

	if replayOpts.timeout >= 0 {
		id, err = replayer.ReplayWithTimeout(n, int32(replayOpts.timeout))
	} else {
		id, err = replayer.Replay(n)
	}

	if err != nil {
		return err
	}

	logger.Debug("replayed notification", "histui_id", n.HistuiID, "dbus_id", id)
	fmt.Printf("Replayed: %s (%s) - %s\n", n.AppName, n.HistuiID[:8], n.Summary)
	return nil
}

// parseDmenuID extracts the ID from dmenu-style output.
func parseDmenuID(selection string) string {
	selection = strings.TrimSpace(selection)

	// If it looks like a raw ID or index, return as-is
	if !strings.Contains(selection, "|") {
		return selection
	}

	// Parse dmenu format: "index | time | app | summary"
	parts := strings.SplitN(selection, "|", 2)
	if len(parts) > 0 {
		return strings.TrimSpace(parts[0])
	}

	return selection
}

// readReplayStdin reads a single line from stdin for replay selection.
func readReplayStdin() (string, error) {
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text()), nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", nil
}

package input

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/jmylchreest/histui/internal/model"
)

// HistuidAdapter fetches notifications from histuid's native history file.
type HistuidAdapter struct {
	historyPath string
}

// NewHistuidAdapter creates a new HistuidAdapter.
// If historyPath is empty, uses the default location.
func NewHistuidAdapter(historyPath string) *HistuidAdapter {
	if historyPath == "" {
		homeDir, _ := os.UserHomeDir()
		historyPath = filepath.Join(homeDir, ".local", "share", "histui", "history.jsonl")
	}
	return &HistuidAdapter{historyPath: historyPath}
}

// Name returns the adapter identifier.
func (a *HistuidAdapter) Name() string {
	return "histuid"
}

// Import fetches all notifications from the history file.
func (a *HistuidAdapter) Import(ctx context.Context) ([]model.Notification, error) {
	file, err := os.Open(a.historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No history yet
		}
		return nil, &AdapterError{
			Source:  "histuid",
			Message: "failed to open history file",
			Err:     err,
		}
	}
	defer func() { _ = file.Close() }()

	var notifications []model.Notification
	scanner := bufio.NewScanner(file)
	// Increase buffer size for long lines
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return notifications, ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var n model.Notification
		if err := json.Unmarshal(line, &n); err != nil {
			// Skip malformed lines
			continue
		}
		notifications = append(notifications, n)
	}

	if err := scanner.Err(); err != nil {
		return notifications, &AdapterError{
			Source:  "histuid",
			Message: "error reading history file",
			Err:     err,
		}
	}

	return notifications, nil
}

// HistuidCounts holds notification counts from histuid.
type HistuidCounts struct {
	Displayed int // Currently visible (seen but not dismissed)
	History   int // Dismissed, in history
	Waiting   int // Received but not yet displayed
}

// GetCounts returns notification counts from the histuid history file.
// A notification is:
//   - Waiting: has histui_imported_at but no histui_seen_at
//   - Displayed: has histui_seen_at but no histui_dismissed_at
//   - History: has histui_dismissed_at
func (a *HistuidAdapter) GetCounts(ctx context.Context) (*HistuidCounts, error) {
	notifications, err := a.Import(ctx)
	if err != nil {
		return nil, err
	}

	counts := &HistuidCounts{}

	for _, n := range notifications {
		if n.HistuiDismissedAt > 0 {
			counts.History++
		} else if n.HistuiSeenAt > 0 {
			counts.Displayed++
		} else {
			counts.Waiting++
		}
	}

	return counts, nil
}

// GetActiveCount returns the count of active (displayed + waiting) notifications.
func (a *HistuidAdapter) GetActiveCount(ctx context.Context) (int, error) {
	counts, err := a.GetCounts(ctx)
	if err != nil {
		return 0, err
	}
	return counts.Displayed + counts.Waiting, nil
}

// GetCountsByUrgency returns notification counts grouped by urgency level.
func (a *HistuidAdapter) GetCountsByUrgency(ctx context.Context) (map[string]*HistuidCounts, error) {
	notifications, err := a.Import(ctx)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*HistuidCounts)
	for _, urgency := range []string{"low", "normal", "critical"} {
		result[urgency] = &HistuidCounts{}
	}

	for _, n := range notifications {
		urgencyName := n.UrgencyName
		if urgencyName == "" {
			urgencyName = "normal"
		}

		counts, ok := result[urgencyName]
		if !ok {
			counts = &HistuidCounts{}
			result[urgencyName] = counts
		}

		if n.HistuiDismissedAt > 0 {
			counts.History++
		} else if n.HistuiSeenAt > 0 {
			counts.Displayed++
		} else {
			counts.Waiting++
		}
	}

	return result, nil
}

// GetHighestActiveUrgency returns the highest urgency level among active notifications.
// Returns "empty" if no active notifications.
func (a *HistuidAdapter) GetHighestActiveUrgency(ctx context.Context) (string, error) {
	notifications, err := a.Import(ctx)
	if err != nil {
		return "empty", err
	}

	highest := -1
	for _, n := range notifications {
		// Only consider active (not dismissed) notifications
		if n.HistuiDismissedAt > 0 {
			continue
		}

		if n.Urgency > highest {
			highest = n.Urgency
		}
	}

	switch highest {
	case model.UrgencyCritical:
		return "critical", nil
	case model.UrgencyNormal:
		return "normal", nil
	case model.UrgencyLow:
		return "low", nil
	default:
		return "empty", nil
	}
}

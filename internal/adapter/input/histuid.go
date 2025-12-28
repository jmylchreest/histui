package input

import (
	"context"
	"sort"

	"github.com/jmylchreest/histui/internal/db"
	"github.com/jmylchreest/histui/internal/model"
)

// HistuidAdapter fetches notifications from histuid's SQLite database.
type HistuidAdapter struct {
	dbPath string
}

// NewHistuidAdapter creates a new HistuidAdapter.
// If dbPath is empty, uses the default database location.
func NewHistuidAdapter(dbPath string) *HistuidAdapter {
	return &HistuidAdapter{dbPath: dbPath}
}

// Name returns the adapter identifier.
func (a *HistuidAdapter) Name() string {
	return "histuid"
}

// Import fetches all notifications from the database.
func (a *HistuidAdapter) Import(ctx context.Context) ([]model.Notification, error) {
	database, err := db.Open(a.dbPath)
	if err != nil {
		return nil, &AdapterError{
			Source:  "histuid",
			Message: "failed to open database",
			Err:     err,
		}
	}
	defer func() { _ = database.Close() }()

	notifications, err := database.All()
	if err != nil {
		return nil, &AdapterError{
			Source:  "histuid",
			Message: "failed to query database",
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

// DetailedCounts holds comprehensive notification statistics with urgency breakdown.
type DetailedCounts struct {
	DnDEnabled bool // Do Not Disturb state

	// Pending: currently visible (seen but not dismissed)
	Pending         int
	PendingCritical int
	PendingNormal   int
	PendingLow      int

	// Missed: not visible and not dismissed (not seen, not dismissed)
	Missed         int
	MissedCritical int
	MissedNormal   int
	MissedLow      int

	// Dismissed: all dismissed notifications
	Dismissed int

	// Tracked: total notifications in history
	Tracked int

	// Top applications by notification count
	TopApps []AppCount
}

// AppCount holds an application name and its notification count.
type AppCount struct {
	AppName string
	Count   int
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

// GetDetailedCounts returns comprehensive notification statistics with urgency breakdown.
// Categories:
//   - Pending: seen but not dismissed (currently visible)
//   - Missed: not seen and not dismissed (never displayed or expired without being seen)
//   - Dismissed: explicitly dismissed
//   - Tracked: total count
func (a *HistuidAdapter) GetDetailedCounts(ctx context.Context) (*DetailedCounts, error) {
	notifications, err := a.Import(ctx)
	if err != nil {
		return nil, err
	}

	counts := &DetailedCounts{
		Tracked: len(notifications),
	}

	// Count apps for top apps calculation
	appCounts := make(map[string]int)

	for _, n := range notifications {
		// Track app counts
		if n.AppName != "" {
			appCounts[n.AppName]++
		}

		urgency := n.Urgency

		if n.HistuiDismissedAt > 0 {
			// Dismissed
			counts.Dismissed++
		} else if n.HistuiSeenAt > 0 {
			// Pending (seen but not dismissed)
			counts.Pending++
			switch urgency {
			case model.UrgencyCritical:
				counts.PendingCritical++
			case model.UrgencyLow:
				counts.PendingLow++
			default:
				counts.PendingNormal++
			}
		} else {
			// Missed (not seen, not dismissed)
			counts.Missed++
			switch urgency {
			case model.UrgencyCritical:
				counts.MissedCritical++
			case model.UrgencyLow:
				counts.MissedLow++
			default:
				counts.MissedNormal++
			}
		}
	}

	// Build top apps list (top 5)
	counts.TopApps = buildTopApps(appCounts, 5)

	return counts, nil
}

// buildTopApps returns the top N apps by notification count.
func buildTopApps(appCounts map[string]int, limit int) []AppCount {
	// Convert map to slice
	apps := make([]AppCount, 0, len(appCounts))
	for name, count := range appCounts {
		apps = append(apps, AppCount{AppName: name, Count: count})
	}

	// Sort by count descending, then alphabetically by name
	sort.Slice(apps, func(i, j int) bool {
		if apps[i].Count != apps[j].Count {
			return apps[i].Count > apps[j].Count // Higher count first
		}
		return apps[i].AppName < apps[j].AppName // Alphabetical when equal
	})

	// Limit to top N
	if len(apps) > limit {
		apps = apps[:limit]
	}

	return apps
}

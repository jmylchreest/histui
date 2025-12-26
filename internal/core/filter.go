// Package core provides filtering, sorting, and lookup logic.
package core

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jmylchreest/histui/internal/model"
)

// FilterOptions specifies criteria for filtering notifications.
type FilterOptions struct {
	Since     time.Duration // Filter to notifications newer than now-since (0=all)
	AppFilter string        // Exact match on app name
	Urgency   *int          // Filter by urgency level (nil=any)
	Limit     int           // Maximum results (0=unlimited)
}

// Filter filters notifications based on the provided options.
func Filter(notifications []model.Notification, opts FilterOptions) []model.Notification {
	now := time.Now()
	var result []model.Notification

	for _, n := range notifications {
		// Time filter
		if opts.Since > 0 {
			cutoff := now.Add(-opts.Since)
			if time.Unix(n.Timestamp, 0).Before(cutoff) {
				continue
			}
		}

		// App filter
		if opts.AppFilter != "" && n.AppName != opts.AppFilter {
			continue
		}

		// Urgency filter
		if opts.Urgency != nil && n.Urgency != *opts.Urgency {
			continue
		}

		result = append(result, n)
	}

	// Apply limit
	if opts.Limit > 0 && len(result) > opts.Limit {
		result = result[:opts.Limit]
	}

	return result
}

// ParseDuration parses a duration string with extended formats.
// Supports: 48h, 7d, 1w, 0 (all time)
func ParseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)

	// Special case: 0 means no filter (all time)
	if s == "0" || s == "" {
		return 0, nil
	}

	// Handle day suffix (7d -> 168h)
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, fmt.Errorf("invalid duration: %s", s)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}

	// Handle week suffix (1w -> 168h)
	if strings.HasSuffix(s, "w") {
		weeks, err := strconv.Atoi(strings.TrimSuffix(s, "w"))
		if err != nil {
			return 0, fmt.Errorf("invalid duration: %s", s)
		}
		return time.Duration(weeks) * 7 * 24 * time.Hour, nil
	}

	// Standard Go duration parsing
	return time.ParseDuration(s)
}

// ParseUrgency parses an urgency string to its integer value.
// Accepts: low, normal, critical, 0, 1, 2
func ParseUrgency(s string) (int, error) {
	s = strings.ToLower(strings.TrimSpace(s))

	switch s {
	case "low", "0":
		return model.UrgencyLow, nil
	case "normal", "1":
		return model.UrgencyNormal, nil
	case "critical", "2":
		return model.UrgencyCritical, nil
	default:
		return 0, fmt.Errorf("invalid urgency: %s (use low, normal, or critical)", s)
	}
}

// Package core provides filtering, sorting, and lookup logic.
package core

import (
	"sort"
	"strings"

	"github.com/jmylchreest/histui/internal/model"
)

// SortField represents a field to sort by.
type SortField string

const (
	SortByTimestamp SortField = "timestamp"
	SortByApp       SortField = "app"
	SortByUrgency   SortField = "urgency"
)

// SortOrder represents ascending or descending order.
type SortOrder string

const (
	SortAsc  SortOrder = "asc"
	SortDesc SortOrder = "desc"
)

// SortOptions specifies sorting criteria.
type SortOptions struct {
	Field SortField // Field to sort by
	Order SortOrder // Sort order (asc/desc)
}

// DefaultSortOptions returns default sort options (newest first).
func DefaultSortOptions() SortOptions {
	return SortOptions{
		Field: SortByTimestamp,
		Order: SortDesc,
	}
}

// Sort sorts notifications in place based on the provided options.
func Sort(notifications []model.Notification, opts SortOptions) {
	if len(notifications) == 0 {
		return
	}

	sort.SliceStable(notifications, func(i, j int) bool {
		var less bool

		switch opts.Field {
		case SortByTimestamp:
			less = notifications[i].Timestamp < notifications[j].Timestamp
		case SortByApp:
			less = strings.ToLower(notifications[i].AppName) < strings.ToLower(notifications[j].AppName)
		case SortByUrgency:
			less = notifications[i].Urgency < notifications[j].Urgency
		default:
			less = notifications[i].Timestamp < notifications[j].Timestamp
		}

		if opts.Order == SortDesc {
			return !less
		}
		return less
	})
}

// ParseSortField parses a sort field string.
func ParseSortField(s string) (SortField, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "timestamp", "time", "t":
		return SortByTimestamp, nil
	case "app", "appname", "a":
		return SortByApp, nil
	case "urgency", "u":
		return SortByUrgency, nil
	default:
		return SortByTimestamp, nil
	}
}

// ParseSortOrder parses a sort order string.
func ParseSortOrder(s string) (SortOrder, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "asc", "ascending", "a":
		return SortAsc, nil
	case "desc", "descending", "d":
		return SortDesc, nil
	default:
		return SortDesc, nil
	}
}

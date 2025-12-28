// Package core provides filtering, sorting, and lookup logic.
package core

import (
	"sort"
	"strings"

	"github.com/jmylchreest/histui/internal/model"
)

// LookupByID finds a notification by its HistuiID.
// Returns nil if not found.
func LookupByID(notifications []model.Notification, id string) *model.Notification {
	for i := range notifications {
		if notifications[i].HistuiID == id {
			return &notifications[i]
		}
	}
	return nil
}

// LookupByIndex finds a notification by its index (1-based for user-friendliness).
// Returns nil if index is out of bounds.
func LookupByIndex(notifications []model.Notification, index int) *model.Notification {
	// Convert to 0-based
	idx := index - 1
	if idx < 0 || idx >= len(notifications) {
		return nil
	}
	return &notifications[idx]
}

// Search finds notifications matching a search term in summary or body.
// Case-insensitive substring match.
func Search(notifications []model.Notification, term string) []model.Notification {
	if term == "" {
		return notifications
	}

	term = strings.ToLower(term)
	var result []model.Notification

	for _, n := range notifications {
		if strings.Contains(strings.ToLower(n.Summary), term) ||
			strings.Contains(strings.ToLower(n.Body), term) {
			result = append(result, n)
		}
	}

	return result
}

// UniqueApps returns a sorted list of unique app names from notifications.
// Empty app names are excluded.
func UniqueApps(notifications []model.Notification) []string {
	seen := make(map[string]struct{})
	for _, n := range notifications {
		if n.AppName != "" {
			seen[n.AppName] = struct{}{}
		}
	}

	apps := make([]string, 0, len(seen))
	for app := range seen {
		apps = append(apps, app)
	}
	sort.Strings(apps)
	return apps
}

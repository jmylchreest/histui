// Package core provides filtering, sorting, and lookup logic.
package core

import (
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
func UniqueApps(notifications []model.Notification) []string {
	seen := make(map[string]bool)
	var apps []string

	for _, n := range notifications {
		if n.AppName != "" && !seen[n.AppName] {
			seen[n.AppName] = true
			apps = append(apps, n.AppName)
		}
	}

	// Sort alphabetically
	sortStrings(apps)
	return apps
}

// sortStrings sorts strings in place (simple insertion sort for small lists).
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && strings.ToLower(s[j]) < strings.ToLower(s[j-1]); j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

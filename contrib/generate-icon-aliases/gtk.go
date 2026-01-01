package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	gtkIconsCacheFile = "gtk-icons.json"
	gtkIconsCacheAge  = 7 * 24 * time.Hour // Cache for 7 days
)

// GitHubTreeResponse represents the GitHub API response for tree listings.
type GitHubTreeResponse struct {
	Tree []GitHubTreeEntry `json:"tree"`
}

// GitHubTreeEntry represents a single entry in the GitHub tree.
type GitHubTreeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"` // "blob" for files, "tree" for directories
}

// FetchGtkIcons fetches GTK symbolic icons from the Adwaita icon theme repository.
// It caches results to avoid hitting the GitHub API rate limit.
func FetchGtkIcons(apiURL string, forceRefresh bool, verbose bool) ([]GtkIconInfo, error) {
	// Check cache first
	if !forceRefresh {
		icons, err := loadGtkIconsCache()
		if err == nil {
			if verbose {
				fmt.Printf("Loaded %d GTK icons from cache\n", len(icons))
			}
			return icons, nil
		}
	}

	if verbose {
		fmt.Println("Fetching GTK icons from GitHub API...")
	}

	// Fetch from GitHub API
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Add User-Agent header (required by GitHub API)
	req.Header.Set("User-Agent", "histui-icon-generator")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var treeResp GitHubTreeResponse
	if err := json.Unmarshal(data, &treeResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	// Parse the tree to extract symbolic icons
	icons := parseGtkIcons(treeResp.Tree)
	if verbose {
		fmt.Printf("Found %d GTK symbolic icons\n", len(icons))
	}

	// Save to cache
	if err := saveGtkIconsCache(icons); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not cache GTK icons: %v\n", err)
	}

	return icons, nil
}

// parseGtkIcons extracts icon info from the GitHub tree entries.
func parseGtkIcons(entries []GitHubTreeEntry) []GtkIconInfo {
	icons := make([]GtkIconInfo, 0, len(entries)/4) // Estimate: ~25% of entries are symbolic icons
	seen := make(map[string]bool)

	for _, entry := range entries {
		// We only care about files under Adwaita/symbolic/
		if entry.Type != "blob" {
			continue
		}

		// Check if it's a symbolic icon file
		if !strings.Contains(entry.Path, "Adwaita/symbolic/") {
			continue
		}

		if !strings.HasSuffix(entry.Path, "-symbolic.svg") {
			continue
		}

		// Extract the category and icon name
		// Path format: Adwaita/symbolic/<category>/<name>-symbolic.svg
		parts := strings.Split(entry.Path, "/")
		if len(parts) < 4 {
			continue
		}

		category := parts[2] // The directory under symbolic/
		filename := parts[len(parts)-1]
		iconName := strings.TrimSuffix(filename, "-symbolic.svg")

		// Skip duplicates
		key := category + "/" + iconName
		if seen[key] {
			continue
		}
		seen[key] = true

		icons = append(icons, GtkIconInfo{
			Name:     iconName,
			Category: category,
			FullName: iconName + "-symbolic",
		})
	}

	return icons
}

// loadGtkIconsCache loads GTK icons from the cache file.
func loadGtkIconsCache() ([]GtkIconInfo, error) {
	data, err := os.ReadFile(gtkIconsCacheFile)
	if err != nil {
		return nil, err
	}

	var cache GtkIconCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}

	// Check if cache is still fresh
	if time.Since(cache.FetchedAt) > gtkIconsCacheAge {
		return nil, fmt.Errorf("cache expired")
	}

	return cache.Icons, nil
}

// saveGtkIconsCache saves GTK icons to the cache file.
func saveGtkIconsCache(icons []GtkIconInfo) error {
	cache := GtkIconCache{
		Icons:     icons,
		FetchedAt: time.Now(),
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(gtkIconsCacheFile, data, 0644)
}

// FilterGtkIconsForCategories filters GTK icons to those most relevant for category fallbacks.
// It prioritizes icons from actions, status, and apps categories.
func FilterGtkIconsForCategories(icons []GtkIconInfo) []GtkIconInfo {
	// Priority order for icon categories
	priorityCategories := map[string]int{
		"actions":    1,
		"status":     2,
		"emblems":    3,
		"categories": 4,
		"apps":       5,
		"mimetypes":  6,
		"places":     7,
		"devices":    8,
		"ui":         9,
		"legacy":     10,
	}

	var filtered []GtkIconInfo
	for _, icon := range icons {
		// Include all categories but we'll sort by priority
		if _, ok := priorityCategories[icon.Category]; ok {
			filtered = append(filtered, icon)
		}
	}

	// Sort by category priority, then by name
	for i := 0; i < len(filtered)-1; i++ {
		for j := i + 1; j < len(filtered); j++ {
			pi := priorityCategories[filtered[i].Category]
			pj := priorityCategories[filtered[j].Category]
			if pi > pj || (pi == pj && filtered[i].Name > filtered[j].Name) {
				filtered[i], filtered[j] = filtered[j], filtered[i]
			}
		}
	}

	return filtered
}

// GetGtkIconsCachePath returns the path to the GTK icons cache file.
func GetGtkIconsCachePath() string {
	return filepath.Join(".", gtkIconsCacheFile)
}

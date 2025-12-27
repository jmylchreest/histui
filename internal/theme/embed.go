// Package theme provides CSS theming for notification popups.
package theme

import (
	"embed"
	"io/fs"
	"path/filepath"
	"strings"
)

// EmbeddedThemes contains all bundled theme CSS files.
//
//go:embed themes/*.css
var EmbeddedThemes embed.FS

// DefaultThemeName is the name of the built-in default theme.
const DefaultThemeName = "default"

// BundledThemes lists all embedded theme names.
var BundledThemes = []string{"default", "minimal", "catppuccin"}

// GetEmbeddedTheme retrieves a bundled theme by name.
// Returns the CSS content and whether it was found.
// For themes with @import, the imports are NOT processed here - use LoadTheme instead.
func GetEmbeddedTheme(name string) (string, bool) {
	path := "themes/" + name + ".css"
	data, err := EmbeddedThemes.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(data), true
}

// GetEmbeddedPartial retrieves a bundled partial (files starting with _).
// Returns the CSS content and whether it was found.
func GetEmbeddedPartial(name string) (string, bool) {
	// Ensure name starts with underscore
	if !strings.HasPrefix(name, "_") {
		name = "_" + name
	}
	// Ensure it has .css extension
	if !strings.HasSuffix(name, ".css") {
		name = name + ".css"
	}

	path := "themes/" + name
	data, err := EmbeddedThemes.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(data), true
}

// ListEmbeddedThemes returns names of all embedded themes.
// Excludes partial files (starting with _) which are meant to be imported.
func ListEmbeddedThemes() []string {
	var themes []string

	entries, err := fs.ReadDir(EmbeddedThemes, "themes")
	if err != nil {
		return BundledThemes // Fallback to known list
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Skip partials (files starting with _)
		if strings.HasPrefix(name, "_") {
			continue
		}
		if ext := filepath.Ext(name); ext == ".css" {
			themeName := strings.TrimSuffix(name, ext)
			themes = append(themes, themeName)
		}
	}

	return themes
}

// IsEmbeddedTheme checks if a theme name is bundled.
func IsEmbeddedTheme(name string) bool {
	_, found := GetEmbeddedTheme(name)
	return found
}

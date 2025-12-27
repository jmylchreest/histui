// Package theme provides CSS theming for notification popups.
package theme

import (
	"embed"
	"io/fs"
	"strings"
)

// EmbeddedThemes contains all bundled theme directories.
// Each theme is a directory containing theme.css and optional assets.
//
//go:embed themes
var EmbeddedThemes embed.FS

// DefaultThemeName is the name of the built-in default theme.
const DefaultThemeName = "default"

// BundledThemes lists all embedded theme names.
var BundledThemes = []string{"default", "minimal", "compact", "detailed", "catppuccin"}

// GetEmbeddedTheme retrieves a bundled theme by name.
// Returns the CSS content and whether it was found.
// Themes are stored as directories: themes/{name}/theme.css
// For themes with @import, the imports are NOT processed here - use LoadTheme instead.
func GetEmbeddedTheme(name string) (string, bool) {
	// Try directory-based theme first: themes/{name}/theme.css
	path := "themes/" + name + "/theme.css"
	data, err := EmbeddedThemes.ReadFile(path)
	if err == nil {
		return string(data), true
	}

	// Fallback: try themes/{name}/{name}.css
	path = "themes/" + name + "/" + name + ".css"
	data, err = EmbeddedThemes.ReadFile(path)
	if err == nil {
		return string(data), true
	}

	return "", false
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
// Themes are stored as directories containing theme.css.
func ListEmbeddedThemes() []string {
	var themes []string

	entries, err := fs.ReadDir(EmbeddedThemes, "themes")
	if err != nil {
		return BundledThemes // Fallback to known list
	}

	for _, entry := range entries {
		// Only directories are valid themes
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Skip hidden directories
		if strings.HasPrefix(name, ".") {
			continue
		}
		// Verify it has a theme.css or {name}.css
		if _, found := GetEmbeddedTheme(name); found {
			themes = append(themes, name)
		}
	}

	return themes
}

// IsEmbeddedTheme checks if a theme name is bundled.
func IsEmbeddedTheme(name string) bool {
	_, found := GetEmbeddedTheme(name)
	return found
}

// GetEmbeddedLayout retrieves a bundled layout by theme name.
// Returns the layout XML content and whether it was found.
// Layouts are stored as: themes/{name}/layout.xml
func GetEmbeddedLayout(name string) (string, bool) {
	path := "themes/" + name + "/layout.xml"
	data, err := EmbeddedThemes.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(data), true
}

// ListEmbeddedLayouts returns names of all embedded layouts.
// Only themes that have a layout.xml are included.
func ListEmbeddedLayouts() []string {
	var layouts []string

	for _, name := range ListEmbeddedThemes() {
		if _, found := GetEmbeddedLayout(name); found {
			layouts = append(layouts, name)
		}
	}

	return layouts
}

// GetEmbeddedManifest retrieves a bundled manifest by theme name.
// Returns the manifest TOML content and whether it was found.
// Manifests are stored as: themes/{name}/manifest.toml
func GetEmbeddedManifest(name string) (string, bool) {
	path := "themes/" + name + "/manifest.toml"
	data, err := EmbeddedThemes.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(data), true
}

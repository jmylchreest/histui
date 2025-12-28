// Package theme provides CSS theming for notification popups.
package theme

import (
	"embed"
	"io/fs"
	"os"
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

// ExtractEmbeddedSounds extracts all embedded sounds for a theme to a directory.
// Returns a map of original paths to extracted paths.
// The extractDir should be a writable directory (e.g., a temp dir or cache dir).
func ExtractEmbeddedSounds(themeName, extractDir string) (map[string]string, error) {
	pathMap := make(map[string]string)

	// List files in the theme's sounds directory
	soundsDir := "themes/" + themeName + "/sounds"
	entries, err := fs.ReadDir(EmbeddedThemes, soundsDir)
	if err != nil {
		// No sounds directory is not an error
		return pathMap, nil
	}

	// Create the extraction directory
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		srcPath := soundsDir + "/" + name
		dstPath := extractDir + "/" + name

		// Read embedded file
		data, err := EmbeddedThemes.ReadFile(srcPath)
		if err != nil {
			continue
		}

		// Write to extraction directory
		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			continue
		}

		// Map relative path (sounds/name) to extracted absolute path
		relativePath := "sounds/" + name
		pathMap[relativePath] = dstPath
	}

	return pathMap, nil
}

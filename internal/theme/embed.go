// Package theme provides CSS theming for notification popups.
package theme

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"strings"

	histuiembed "github.com/jmylchreest/histui/embed"
)

// EmbeddedThemes provides access to bundled theme directories.
// Each theme is a directory containing theme.css and optional assets.
var EmbeddedThemes embed.FS = histuiembed.Themes

// DefaultThemeName is the name of the built-in default theme.
const DefaultThemeName = "default"

// BundledThemes lists all embedded theme names.
var BundledThemes = []string{"default", "minimal", "compact", "detailed", "catppuccin", "glass"}

// RequiredCSSClasses are the selectors every theme must define so that
// notification popups render correctly across all urgency levels.
var RequiredCSSClasses = []string{
	".notification-popup",
	".notification-summary",
	".notification-body",
	".notification-appname",
	".notification-close",
	".urgency-low",
	".urgency-normal",
	".urgency-critical",
}

// ValidateCSS checks theme CSS for the required selectors and basic structural
// validity. It returns a slice of human-readable problems; an empty slice means
// the CSS passed all checks.
func ValidateCSS(css string) []string {
	var problems []string

	for _, class := range RequiredCSSClasses {
		if !strings.Contains(css, class) {
			problems = append(problems, fmt.Sprintf("missing required selector %q", class))
		}
	}

	if open, closed := strings.Count(css, "{"), strings.Count(css, "}"); open != closed {
		problems = append(problems, fmt.Sprintf("unbalanced braces: %d '{' vs %d '}'", open, closed))
	}
	if strings.Contains(css, "{{") {
		problems = append(problems, "contains '{{' (likely a template or syntax error)")
	}
	if strings.Contains(css, "}}") {
		problems = append(problems, "contains '}}' (likely a template or syntax error)")
	}

	return problems
}

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

// GetEmbeddedPartial retrieves a bundled partial CSS file.
// Returns the CSS content and whether it was found.
// Partials are standalone CSS files in the themes directory (e.g., animations.css).
func GetEmbeddedPartial(name string) (string, bool) {
	// Strip leading underscore if present (legacy partial naming)
	name = strings.TrimPrefix(name, "_")

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

// GetEmbeddedAliases retrieves a bundled aliases file by theme name.
// Returns the aliases TOML content and whether it was found.
// Aliases are stored as: themes/{name}/aliases.toml
func GetEmbeddedAliases(name string) (string, bool) {
	path := "themes/" + name + "/aliases.toml"
	data, err := EmbeddedThemes.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(data), true
}

// GetEmbeddedIcon retrieves a bundled icon by theme and icon name.
// Returns the icon data and whether it was found.
// Icons are stored as: themes/{themeName}/icons/{iconName}.{ext}
// Checks for .svg, .png, .xpm extensions.
func GetEmbeddedIcon(themeName, iconName string) ([]byte, string, bool) {
	extensions := []string{".svg", ".png", ".xpm"}
	basePath := "themes/" + themeName + "/icons/" + iconName

	for _, ext := range extensions {
		path := basePath + ext
		data, err := EmbeddedThemes.ReadFile(path)
		if err == nil {
			return data, ext, true
		}
	}

	return nil, "", false
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

// ExtractEmbeddedIcons extracts all embedded icons for a theme to a directory.
// Returns the icons directory path if any icons were extracted, empty string otherwise.
// The extractDir should be a writable directory (e.g., a cache dir).
func ExtractEmbeddedIcons(themeName, extractDir string) (string, error) {
	// List files in the theme's icons directory
	iconsDir := "themes/" + themeName + "/icons"
	entries, err := fs.ReadDir(EmbeddedThemes, iconsDir)
	if err != nil {
		// No icons directory is not an error
		return "", nil
	}

	// Create the icons extraction directory
	iconsExtractDir := extractDir + "/icons"
	if err := os.MkdirAll(iconsExtractDir, 0755); err != nil {
		return "", err
	}

	extracted := 0
	for _, entry := range entries {
		if entry.IsDir() {
			// Handle subdirectories like scalable/ or 48x48/
			subDir := iconsDir + "/" + entry.Name()
			subEntries, err := fs.ReadDir(EmbeddedThemes, subDir)
			if err != nil {
				continue
			}

			subExtractDir := iconsExtractDir + "/" + entry.Name()
			if err := os.MkdirAll(subExtractDir, 0755); err != nil {
				continue
			}

			for _, subEntry := range subEntries {
				if subEntry.IsDir() {
					continue
				}

				srcPath := subDir + "/" + subEntry.Name()
				dstPath := subExtractDir + "/" + subEntry.Name()

				data, err := EmbeddedThemes.ReadFile(srcPath)
				if err != nil {
					continue
				}

				if err := os.WriteFile(dstPath, data, 0644); err != nil {
					continue
				}
				extracted++
			}
			continue
		}

		name := entry.Name()
		srcPath := iconsDir + "/" + name
		dstPath := iconsExtractDir + "/" + name

		// Read embedded file
		data, err := EmbeddedThemes.ReadFile(srcPath)
		if err != nil {
			continue
		}

		// Write to extraction directory
		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			continue
		}
		extracted++
	}

	if extracted == 0 {
		return "", nil
	}

	return iconsExtractDir, nil
}

// Package theme provides CSS theming support for histuid popups.
package theme

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// importRegex matches @import "file.css"; or @import 'file.css'; or @import url("file.css");
var importRegex = regexp.MustCompile(`@import\s+(?:url\s*\(\s*)?["']([^"']+)["']\s*\)?;?`)

// Theme represents a CSS theme with metadata.
type Theme struct {
	Name      string    // Theme name (without .css extension)
	Path      string    // Full path to the CSS file (empty for default)
	Dir       string    // Theme directory (for directory-based themes)
	CSS       string    // The CSS content
	ModTime   time.Time // Last modification time
	IsDefault bool      // True if this is the embedded default theme
	Manifest  *Manifest // Theme manifest (nil if no manifest)
}

// NewTheme creates a new Theme by loading a CSS file.
// CSS @import statements are resolved and inlined.
func NewTheme(name, path string) (*Theme, error) {
	css, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// Process @import statements
	baseDir := filepath.Dir(path)
	processedCSS := ProcessImports(string(css), baseDir, nil)

	return &Theme{
		Name:    name,
		Path:    path,
		Dir:     baseDir,
		CSS:     processedCSS,
		ModTime: info.ModTime(),
	}, nil
}

// NewThemeFromDir creates a Theme from a directory containing theme.css and optional manifest.
// This supports the directory-based theme format for themes with bundled assets.
func NewThemeFromDir(name, themeDir string) (*Theme, error) {
	// Look for CSS file (theme.css, <name>.css, or style.css)
	cssPath := ""
	candidates := []string{
		filepath.Join(themeDir, "theme.css"),
		filepath.Join(themeDir, name+".css"),
		filepath.Join(themeDir, "style.css"),
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			cssPath = path
			break
		}
	}

	if cssPath == "" {
		return nil, os.ErrNotExist
	}

	// Load the theme
	theme, err := NewTheme(name, cssPath)
	if err != nil {
		return nil, err
	}

	theme.Dir = themeDir

	// Load manifest if present
	if manifestPath, found := FindManifest(themeDir); found {
		manifest, err := LoadManifest(manifestPath)
		if err == nil {
			theme.Manifest = manifest

			// Resolve relative sound paths to absolute paths
			theme.resolveManifestPaths()
		}
	}

	return theme, nil
}

// resolveManifestPaths resolves relative paths in the manifest to absolute paths.
func (t *Theme) resolveManifestPaths() {
	if t.Manifest == nil || t.Dir == "" {
		return
	}

	resolvePath := func(path string) string {
		if path == "" || filepath.IsAbs(path) {
			return path
		}
		return filepath.Join(t.Dir, path)
	}

	t.Manifest.Audio.Low.Path = resolvePath(t.Manifest.Audio.Low.Path)
	t.Manifest.Audio.Normal.Path = resolvePath(t.Manifest.Audio.Normal.Path)
	t.Manifest.Audio.Critical.Path = resolvePath(t.Manifest.Audio.Critical.Path)
}

// GetIconSize returns the icon size from the manifest, or the default (48).
func (t *Theme) GetIconSize() int {
	if t.Manifest != nil {
		return t.Manifest.GetIconSize()
	}
	return 48
}

// GetSoundConfig returns the sound configuration for a given urgency level.
// Returns nil if no manifest or no sound is configured.
func (t *Theme) GetSoundConfig(urgency int) *SoundConfig {
	if t.Manifest == nil {
		return nil
	}
	config := t.Manifest.GetSoundConfig(urgency)
	if !config.IsEnabled() {
		return nil
	}
	return config
}

// ProcessImports resolves and inlines @import statements in CSS.
// Imports are resolved relative to baseDir.
// The seen map prevents circular imports.
func ProcessImports(css string, baseDir string, seen map[string]bool) string {
	if seen == nil {
		seen = make(map[string]bool)
	}

	return importRegex.ReplaceAllStringFunc(css, func(match string) string {
		// Extract the file path from the @import statement
		submatch := importRegex.FindStringSubmatch(match)
		if len(submatch) < 2 {
			return match // Keep original if parsing fails
		}

		importPath := submatch[1]

		// Resolve the path
		var fullPath string
		if filepath.IsAbs(importPath) {
			fullPath = importPath
		} else {
			fullPath = filepath.Join(baseDir, importPath)
		}

		// Prevent circular imports
		if seen[fullPath] {
			return "/* circular import prevented: " + importPath + " */"
		}
		seen[fullPath] = true

		// Try to read the imported file
		importedCSS, err := os.ReadFile(fullPath)
		if err != nil {
			// Check if it's an embedded partial (files starting with underscore)
			baseName := filepath.Base(importPath)
			if strings.HasPrefix(baseName, "_") {
				// Try embedded partials
				if embeddedCSS, found := GetEmbeddedPartial(baseName); found {
					return "/* imported (embedded): " + importPath + " */\n" + embeddedCSS
				}
			}
			// Also try as a regular embedded theme
			themeName := strings.TrimSuffix(baseName, ".css")
			if embeddedCSS, found := GetEmbeddedTheme(themeName); found {
				return "/* imported (embedded): " + importPath + " */\n" + embeddedCSS
			}
			return "/* import failed: " + importPath + " - " + err.Error() + " */"
		}

		// Recursively process imports in the imported file
		importedBaseDir := filepath.Dir(fullPath)
		processedImport := ProcessImports(string(importedCSS), importedBaseDir, seen)

		return "/* imported: " + importPath + " */\n" + processedImport
	})
}

// NewDefaultTheme creates the embedded default theme.
func NewDefaultTheme() *Theme {
	css, _ := GetEmbeddedTheme(DefaultThemeName)
	return &Theme{
		Name:      DefaultThemeName,
		Path:      "",
		CSS:       css,
		ModTime:   time.Time{},
		IsDefault: true,
	}
}

// Reload reloads the theme from disk.
// Returns true if the content changed.
func (t *Theme) Reload() (bool, error) {
	if t.IsDefault {
		return false, nil
	}

	info, err := os.Stat(t.Path)
	if err != nil {
		return false, err
	}

	// Check if modification time changed
	if !info.ModTime().After(t.ModTime) {
		return false, nil
	}

	css, err := os.ReadFile(t.Path)
	if err != nil {
		return false, err
	}

	// Process @import statements
	baseDir := filepath.Dir(t.Path)
	processedCSS := ProcessImports(string(css), baseDir, nil)

	oldCSS := t.CSS
	t.CSS = processedCSS
	t.ModTime = info.ModTime()

	return oldCSS != t.CSS, nil
}

// ThemeInfo provides basic theme information for listing.
type ThemeInfo struct {
	Name        string
	Path        string
	IsDefault   bool
	IsBundled   bool // True if this is a bundled/embedded theme
	IsDirectory bool // True if this is a directory-based theme
	HasManifest bool // True if the theme has a manifest file
}

// ListAvailableThemes lists all available themes (bundled + user).
func ListAvailableThemes() ([]ThemeInfo, error) {
	seen := make(map[string]bool)
	var themes []ThemeInfo

	// Add bundled themes first
	for _, name := range ListEmbeddedThemes() {
		if !seen[name] {
			seen[name] = true
			themes = append(themes, ThemeInfo{
				Name:      name,
				Path:      "",
				IsDefault: name == DefaultThemeName,
				IsBundled: true,
			})
		}
	}

	// Add user themes
	themesDir, err := ThemesDir()
	if err != nil {
		return themes, nil
	}

	entries, err := os.ReadDir(themesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return themes, nil
		}
		return themes, err
	}

	for _, entry := range entries {
		name := entry.Name()

		if entry.IsDir() {
			// Check if this is a directory-based theme
			themeDir := filepath.Join(themesDir, name)
			if isThemeDir(themeDir) {
				if !seen[name] {
					seen[name] = true
					_, hasManifest := FindManifest(themeDir)
					themes = append(themes, ThemeInfo{
						Name:        name,
						Path:        themeDir,
						IsDirectory: true,
						HasManifest: hasManifest,
					})
				}
			}
			continue
		}

		// Single CSS file theme
		if filepath.Ext(name) == ".css" {
			themeName := name[:len(name)-4]
			if !seen[themeName] {
				seen[themeName] = true
				themes = append(themes, ThemeInfo{
					Name: themeName,
					Path: filepath.Join(themesDir, name),
				})
			}
		}
	}

	return themes, nil
}

// isThemeDir checks if a directory is a valid theme directory.
// A valid theme directory must contain at least one of: theme.css, style.css, or <dirname>.css
func isThemeDir(dir string) bool {
	name := filepath.Base(dir)
	candidates := []string{
		filepath.Join(dir, "theme.css"),
		filepath.Join(dir, name+".css"),
		filepath.Join(dir, "style.css"),
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

// CreateThemesDir creates the themes directory if it doesn't exist.
func CreateThemesDir() error {
	themesDir, err := ThemesDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(themesDir, 0755)
}

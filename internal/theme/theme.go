// Package theme provides CSS theming support for histuid popups.
package theme

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// importLogger is used for debugging import resolution.
// Set via SetImportLogger to enable debug logging.
var importLogger *slog.Logger

// SetImportLogger sets the logger for import debugging.
func SetImportLogger(logger *slog.Logger) {
	importLogger = logger
}

// importRegex matches @import "file.css"; or @import 'file.css'; or @import url("file.css");
var importRegex = regexp.MustCompile(`@import\s+(?:url\s*\(\s*)?["']([^"']+)["']\s*\)?;?`)

// commentRegex matches CSS block comments /* ... */
var commentRegex = regexp.MustCompile(`/\*[\s\S]*?\*/`)

// commentPlaceholder is used to temporarily replace comments during import processing.
const commentPlaceholder = "\x00COMMENT_%d\x00"

// replaceCommentsWithPlaceholders replaces CSS comments with numbered placeholders.
// Returns the modified CSS and a slice of the original comments.
func replaceCommentsWithPlaceholders(css string) (string, []string) {
	var comments []string
	i := 0
	result := commentRegex.ReplaceAllStringFunc(css, func(match string) string {
		comments = append(comments, match)
		placeholder := fmt.Sprintf(commentPlaceholder, i)
		i++
		return placeholder
	})
	return result, comments
}

// restoreComments replaces placeholders with original comments.
func restoreComments(css string, comments []string) string {
	for i, comment := range comments {
		placeholder := fmt.Sprintf(commentPlaceholder, i)
		css = strings.Replace(css, placeholder, comment, 1)
	}
	return css
}

// Theme represents a CSS theme with metadata.
type Theme struct {
	Name      string    // Theme name (without .css extension)
	Path      string    // Full path to the CSS file (empty for default)
	Dir       string    // Theme directory (for directory-based themes)
	CSS       string    // The CSS content
	Layout    string    // Layout XML content (empty uses default)
	ModTime   time.Time // Last modification time
	IsDefault bool      // True if this is the embedded default theme
	Manifest  *Manifest // Theme manifest (nil if no manifest)

	// Icon-related fields
	Aliases  map[string]string // Theme-specific icon aliases (app name -> canonical icon name)
	Symbols  map[string]string // Theme-specific symbols (canonical name -> glyph character)
	GtkIcons map[string]string // Theme-specific GTK icons (canonical name -> GTK icon name)
	IconsDir string            // Path to theme's icons/ folder (for custom icon images)

	// Import tracking for hot-reload
	ImportedFiles []string // List of imported file paths (for watching)
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

	// Process @import statements and track imported files
	baseDir := filepath.Dir(path)
	var importedFiles []string
	processedCSS := ProcessImportsWithTracking(string(css), baseDir, nil, &importedFiles)

	return &Theme{
		Name:          name,
		Path:          path,
		Dir:           baseDir,
		CSS:           processedCSS,
		ModTime:       info.ModTime(),
		ImportedFiles: importedFiles,
	}, nil
}

// NewThemeFromDir creates a Theme from a directory containing theme.css and optional manifest.
// This supports the directory-based theme format for themes with bundled assets.
//
// Directory structure:
//
//	mytheme/
//	  ├── theme.css         (required - or <name>.css or style.css)
//	  ├── manifest.toml     (optional - theme metadata and audio config)
//	  ├── aliases.toml      (optional - icon aliases for this theme)
//	  ├── layout.xml        (optional - notification layout)
//	  ├── sounds/           (optional - bundled sound files)
//	  └── icons/            (optional - bundled icon images)
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

	// Load theme-specific icon aliases, symbols, and GTK icons if present
	if aliasesFile, found := loadThemeAliasesFile(themeDir); found {
		theme.Aliases = aliasesFile.Aliases
		theme.Symbols = aliasesFile.Symbols
		theme.GtkIcons = aliasesFile.GtkIcons
	}

	// Check for icons directory
	iconsDir := filepath.Join(themeDir, "icons")
	if info, err := os.Stat(iconsDir); err == nil && info.IsDir() {
		theme.IconsDir = iconsDir
	}

	return theme, nil
}

// themeAliasesFileData represents the structure of a theme's aliases.toml file.
type themeAliasesFileData struct {
	Aliases  map[string]string `toml:"aliases"`
	Symbols  map[string]string `toml:"symbols"`
	GtkIcons map[string]string `toml:"gtk-icons"`
}

// loadThemeAliasesFile loads icon aliases, symbols, and GTK icons from a theme directory.
// Looks for aliases.toml in the theme directory.
// Returns the parsed file and whether it was found.
func loadThemeAliasesFile(themeDir string) (*themeAliasesFileData, bool) {
	aliasesPath := filepath.Join(themeDir, "aliases.toml")

	data, err := os.ReadFile(aliasesPath)
	if err != nil {
		return nil, false
	}

	var file themeAliasesFileData
	if err := toml.Unmarshal(data, &file); err != nil {
		return nil, false
	}

	// Initialize nil maps to empty maps
	if file.Aliases == nil {
		file.Aliases = make(map[string]string)
	}
	if file.Symbols == nil {
		file.Symbols = make(map[string]string)
	}
	if file.GtkIcons == nil {
		file.GtkIcons = make(map[string]string)
	}

	// Return true if any section has content
	if len(file.Aliases) == 0 && len(file.Symbols) == 0 && len(file.GtkIcons) == 0 {
		return nil, false
	}

	return &file, true
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
//
// Import resolution order:
//  1. Relative to current file's directory (baseDir)
//  2. User themes directory (~/.config/histui/themes/)
//  3. Embedded themes
//
// Supported import formats:
//   - @import "default.css";           - imports embedded theme "default"
//   - @import "default/theme.css";     - imports embedded theme "default"
//   - @import "mytheme/theme.css";     - imports user theme "mytheme" (or embedded if not found)
//   - @import "_base.css";             - imports embedded partial "_base.css"
func ProcessImports(css string, baseDir string, seen map[string]bool) string {
	return ProcessImportsWithTracking(css, baseDir, seen, nil)
}

// ProcessImportsWithTracking is like ProcessImports but also tracks imported file paths.
// The importedFiles slice will be populated with absolute paths of all imported files
// (excluding embedded imports). This is useful for hot-reload watching.
func ProcessImportsWithTracking(css string, baseDir string, seen map[string]bool, importedFiles *[]string) string {
	if seen == nil {
		seen = make(map[string]bool)
	}

	// Get user themes directory for fallback resolution
	userThemesDir, _ := ThemesDir()

	if importLogger != nil {
		importLogger.Debug("processing imports", "base_dir", baseDir, "user_themes_dir", userThemesDir)
	}

	// Replace comments with placeholders to avoid matching @import inside comments
	cssWithPlaceholders, comments := replaceCommentsWithPlaceholders(css)

	// Find all imports in the placeholder-replaced CSS
	imports := importRegex.FindAllStringSubmatch(cssWithPlaceholders, -1)
	if len(imports) == 0 {
		return css // No imports found, return original
	}

	// Step 1: Replace each import with a unique placeholder
	// This prevents resolved content from being matched by subsequent replacements
	importPlaceholder := "\x00IMPORT_%d\x00"
	result := cssWithPlaceholders
	resolvedImports := make([]string, 0, len(imports))

	for i, match := range imports {
		if len(match) < 2 {
			continue
		}
		importPath := match[1]
		fullMatch := match[0]

		// Resolve the import
		resolved := resolveImportWithTracking(importPath, baseDir, userThemesDir, seen, importedFiles)
		resolvedImports = append(resolvedImports, resolved)

		// Replace with unique placeholder
		placeholder := fmt.Sprintf(importPlaceholder, i)
		result = strings.Replace(result, fullMatch, placeholder, 1)
	}

	// Step 2: Replace import placeholders with resolved content
	for i, resolved := range resolvedImports {
		placeholder := fmt.Sprintf(importPlaceholder, i)
		result = strings.Replace(result, placeholder, resolved, 1)
	}

	// Restore comments from placeholders
	result = restoreComments(result, comments)

	return result
}

// resolveImportWithTracking resolves a single @import statement and returns the replacement content.
// If importedFiles is non-nil, absolute paths of resolved files (not embedded) are appended.
func resolveImportWithTracking(importPath, baseDir, userThemesDir string, seen map[string]bool, importedFiles *[]string) string {
	if importLogger != nil {
		importLogger.Debug("resolving import", "import_path", importPath)
	}

	// Resolve the path relative to current file
	var fullPath string
	if filepath.IsAbs(importPath) {
		fullPath = importPath
	} else {
		fullPath = filepath.Join(baseDir, importPath)
	}

	// Prevent circular imports
	if seen[fullPath] {
		if importLogger != nil {
			importLogger.Debug("circular import prevented", "path", fullPath)
		}
		return "/* circular import prevented: " + importPath + " */"
	}
	seen[fullPath] = true

	// Try to read the imported file from disk (relative to current file)
	if importLogger != nil {
		importLogger.Debug("trying relative path", "full_path", fullPath)
	}
	if importedCSS, err := os.ReadFile(fullPath); err == nil {
		if importLogger != nil {
			importLogger.Debug("import resolved (relative)", "path", fullPath, "size", len(importedCSS))
		}
		// Track the imported file
		if importedFiles != nil {
			absPath, _ := filepath.Abs(fullPath)
			*importedFiles = append(*importedFiles, absPath)
		}
		importedBaseDir := filepath.Dir(fullPath)
		processedImport := ProcessImportsWithTracking(string(importedCSS), importedBaseDir, seen, importedFiles)
		return "/* imported: " + importPath + " */\n" + processedImport
	} else if importLogger != nil {
		importLogger.Debug("relative path failed", "path", fullPath, "error", err)
	}

	// Try user themes directory
	if userThemesDir != "" && !filepath.IsAbs(importPath) {
		userPath := filepath.Join(userThemesDir, importPath)
		if importLogger != nil {
			importLogger.Debug("trying user themes path", "user_path", userPath)
		}
		if importedCSS, err := os.ReadFile(userPath); err == nil {
			if importLogger != nil {
				importLogger.Debug("import resolved (user)", "path", userPath, "size", len(importedCSS))
			}
			// Track the imported file
			if importedFiles != nil {
				absPath, _ := filepath.Abs(userPath)
				*importedFiles = append(*importedFiles, absPath)
			}
			seen[userPath] = true
			importedBaseDir := filepath.Dir(userPath)
			processedImport := ProcessImportsWithTracking(string(importedCSS), importedBaseDir, seen, importedFiles)
			return "/* imported (user): " + importPath + " */\n" + processedImport
		} else if importLogger != nil {
			importLogger.Debug("user themes path failed", "path", userPath, "error", err)
		}
	}

	// Try embedded themes (not tracked - they can't be modified by user)
	if importLogger != nil {
		importLogger.Debug("trying embedded import", "import_path", importPath)
	}
	if embeddedCSS, found := resolveEmbeddedImport(importPath); found {
		if importLogger != nil {
			importLogger.Debug("import resolved (embedded)", "path", importPath, "size", len(embeddedCSS))
		}
		processedImport := ProcessImportsWithTracking(embeddedCSS, "", seen, importedFiles)
		return "/* imported (embedded): " + importPath + " */\n" + processedImport
	}

	if importLogger != nil {
		importLogger.Warn("import failed", "path", importPath)
	}
	return "/* import failed: " + importPath + " - file not found */"
}

// resolveEmbeddedImport attempts to resolve an import path to embedded CSS.
// Supports multiple formats:
//   - "default.css" → embedded theme "default"
//   - "default/theme.css" → embedded theme "default"
//   - "animations.css" → embedded partial "animations.css"
//   - "_animations.css" → embedded partial "animations.css" (legacy)
func resolveEmbeddedImport(importPath string) (string, bool) {
	baseName := filepath.Base(importPath)
	dirName := filepath.Dir(importPath)

	// Check for embedded partial (animations.css, _animations.css, etc.)
	// GetEmbeddedPartial handles both with and without underscore prefix
	if embeddedCSS, found := GetEmbeddedPartial(baseName); found {
		return embeddedCSS, true
	}

	// Try "themename/theme.css" or "themename/themename.css" format
	// Extract theme name from directory component
	if dirName != "." && dirName != "" {
		// Get the first directory component as the theme name
		parts := strings.Split(filepath.ToSlash(importPath), "/")
		if len(parts) >= 2 {
			themeName := parts[0]
			if embeddedCSS, found := GetEmbeddedTheme(themeName); found {
				return embeddedCSS, true
			}
		}
	}

	// Try as a direct theme name: "default.css" → theme "default"
	themeName := strings.TrimSuffix(baseName, ".css")
	if embeddedCSS, found := GetEmbeddedTheme(themeName); found {
		return embeddedCSS, true
	}

	return "", false
}

// Reload reloads the theme from disk if the main file has changed.
// Returns true if the content changed.
// Use ForceReload to reload regardless of modification time (e.g., when imported files change).
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

	return t.ForceReload()
}

// ForceReload reloads the theme from disk regardless of modification time.
// This is useful when imported files have changed.
// Returns true if the content changed.
func (t *Theme) ForceReload() (bool, error) {
	if t.IsDefault {
		return false, nil
	}

	css, err := os.ReadFile(t.Path)
	if err != nil {
		return false, err
	}

	info, err := os.Stat(t.Path)
	if err != nil {
		return false, err
	}

	// Process @import statements and track imported files
	baseDir := filepath.Dir(t.Path)
	var importedFiles []string
	processedCSS := ProcessImportsWithTracking(string(css), baseDir, nil, &importedFiles)

	oldCSS := t.CSS
	t.CSS = processedCSS
	t.ModTime = info.ModTime()
	t.ImportedFiles = importedFiles

	return oldCSS != t.CSS, nil
}

// IsImportedFile checks if the given path is an imported file for this theme.
func (t *Theme) IsImportedFile(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	for _, imp := range t.ImportedFiles {
		if imp == absPath {
			return true
		}
	}
	return false
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

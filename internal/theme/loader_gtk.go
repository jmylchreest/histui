//go:build cgo

package theme

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/pelletier/go-toml/v2"
)

// embeddedAliasesFile represents parsed aliases data from embedded TOML.
type embeddedAliasesFile struct {
	Aliases  map[string]string
	Symbols  map[string]string
	GtkIcons map[string]string
}

// parseEmbeddedAliasesFile parses TOML aliases data from a string.
// Returns all sections: aliases, symbols, and gtk-icons.
func parseEmbeddedAliasesFile(data string) *embeddedAliasesFile {
	type aliasesFile struct {
		Aliases  map[string]string `toml:"aliases"`
		Symbols  map[string]string `toml:"symbols"`
		GtkIcons map[string]string `toml:"gtk-icons"`
	}

	var file aliasesFile
	if err := toml.Unmarshal([]byte(data), &file); err != nil {
		return nil
	}

	return &embeddedAliasesFile{
		Aliases:  file.Aliases,
		Symbols:  file.Symbols,
		GtkIcons: file.GtkIcons,
	}
}

// Loader handles loading and applying CSS themes with hot-reload support.
type Loader struct {
	mu           sync.RWMutex
	logger       *slog.Logger
	provider     *gtk.CSSProvider
	fontProvider *gtk.CSSProvider // Separate provider for font overrides (higher priority)
	themesDir    string
	currentName  string
	theme        *Theme
	watcher      *Watcher
	display      *gdk.Display
	fontFamily   string
	fontSize     int

	// Callback for hot-reload events (called when CSS files change)
	onHotReload func(themeName string, changedFile string)
}

// NewLoader creates a new theme loader.
func NewLoader(logger *slog.Logger) *Loader {
	if logger == nil {
		logger = slog.Default()
	}

	themesDir, err := ThemesDir()
	if err != nil {
		logger.Warn("failed to get themes directory", "error", err)
		themesDir = ""
	}

	return &Loader{
		logger:    logger,
		provider:  gtk.NewCSSProvider(),
		themesDir: themesDir,
	}
}

// LoadTheme loads a theme by name.
// Theme resolution order:
//  1. User themes directory (~/.config/histui/themes/)
//     a. Directory-based theme (name/ with theme.css and optional manifest)
//     b. Single CSS file (name.css)
//  2. Embedded/bundled themes
//
// Partial themes automatically inherit from the default theme:
//   - Missing layout.xml: uses default's layout
//   - Missing/partial manifest.toml: merged with default's manifest (sounds, icons)
//
// This allows users to create themes that only override colors or specific elements.
func (l *Loader) LoadTheme(name string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Enable import debugging
	SetImportLogger(l.logger)

	if name == "" {
		name = DefaultThemeName
	}

	// First, check user themes directory
	if l.themesDir != "" {
		// Check for directory-based theme first
		themeDir := filepath.Join(l.themesDir, name)
		if info, err := os.Stat(themeDir); err == nil && info.IsDir() {
			theme, err := NewThemeFromDir(name, themeDir)
			if err == nil {
				// Apply defaults from embedded default theme
				l.applyDefaults(theme)
				l.provider.LoadFromString(theme.CSS)
				l.currentName = name
				l.theme = theme
				l.logger.Info("loaded user theme (directory)", "name", name, "path", themeDir,
					"has_manifest", theme.Manifest != nil, "has_layout", theme.Layout != "")
				return nil
			}
			l.logger.Debug("failed to load directory theme", "name", name, "error", err)
		}

		// Check for single CSS file theme
		themePath := filepath.Join(l.themesDir, name+".css")
		if _, err := os.Stat(themePath); err == nil {
			theme, err := NewTheme(name, themePath)
			if err != nil {
				l.logger.Warn("failed to load user theme, trying bundled", "theme", name, "error", err)
			} else {
				// Apply defaults from embedded default theme
				l.applyDefaults(theme)
				l.provider.LoadFromString(theme.CSS)
				l.currentName = name
				l.theme = theme
				l.logger.Info("loaded user theme", "name", name, "path", themePath,
					"has_manifest", theme.Manifest != nil, "has_layout", theme.Layout != "")
				return nil
			}
		}
	}

	// Second, check embedded themes
	if css, found := GetEmbeddedTheme(name); found {
		// Process @import statements in embedded themes
		processedCSS := ProcessImports(css, "", nil)
		l.theme = &Theme{
			Name:      name,
			Path:      "",
			CSS:       processedCSS,
			IsDefault: name == DefaultThemeName,
		}

		// Get cache directory for extracting embedded assets
		cacheDir, err := l.getSoundsCacheDir(name)
		if err == nil {
			// Load embedded manifest if present
			if manifestData, found := GetEmbeddedManifest(name); found {
				manifest, err := ParseManifest([]byte(manifestData), ".toml")
				if err == nil {
					l.theme.Manifest = manifest

					// Extract embedded sounds to cache directory
					pathMap, err := ExtractEmbeddedSounds(name, cacheDir)
					if err == nil && len(pathMap) > 0 {
						l.theme.Dir = cacheDir
						// Update manifest paths to point to extracted sounds
						l.updateManifestSoundPaths(pathMap)
						l.logger.Debug("extracted embedded sounds", "theme", name, "count", len(pathMap))
					}
				} else {
					l.logger.Warn("failed to parse embedded manifest", "theme", name, "error", err)
				}
			}

		}

		// Extract embedded icons to theme cache directory (not sounds cache)
		themeCacheDir, err := l.getThemeCacheDir(name)
		if err == nil {
			iconsDir, err := ExtractEmbeddedIcons(name, themeCacheDir)
			if err == nil && iconsDir != "" {
				l.theme.IconsDir = iconsDir
				l.logger.Debug("extracted embedded icons", "theme", name, "icons_dir", iconsDir)
			}
		}

		// Load theme-specific aliases, symbols, and GTK icons if present
		// (These are overrides only - global aliases come from embed/aliases_default.toml)
		if aliasesData, found := GetEmbeddedAliases(name); found {
			aliasesFile := parseEmbeddedAliasesFile(aliasesData)
			if aliasesFile != nil {
				l.theme.Aliases = aliasesFile.Aliases
				l.theme.Symbols = aliasesFile.Symbols
				l.theme.GtkIcons = aliasesFile.GtkIcons
				// Only log if there are actual overrides (0 is expected for default themes)
				if len(aliasesFile.Aliases) > 0 || len(aliasesFile.Symbols) > 0 || len(aliasesFile.GtkIcons) > 0 {
					l.logger.Debug("loaded theme icon overrides", "theme", name,
						"aliases_count", len(aliasesFile.Aliases),
						"symbols_count", len(aliasesFile.Symbols),
						"gtk_icons_count", len(aliasesFile.GtkIcons))
				}
			}
		}

		l.provider.LoadFromString(processedCSS)
		l.currentName = name
		l.logger.Info("loaded bundled theme", "name", name,
			"has_manifest", l.theme.Manifest != nil,
			"has_aliases", len(l.theme.Aliases) > 0,
			"has_icons", l.theme.IconsDir != "")
		return nil
	}

	// Fallback to default theme
	l.logger.Warn("theme not found, using default", "theme", name)
	css, _ := GetEmbeddedTheme(DefaultThemeName)
	processedCSS := ProcessImports(css, "", nil)
	l.theme = &Theme{
		Name:      DefaultThemeName,
		Path:      "",
		CSS:       processedCSS,
		IsDefault: true,
	}

	// Get cache directory for extracting embedded assets
	cacheDir, err := l.getSoundsCacheDir(DefaultThemeName)
	if err == nil {
		// Load embedded manifest for default theme
		if manifestData, found := GetEmbeddedManifest(DefaultThemeName); found {
			manifest, err := ParseManifest([]byte(manifestData), ".toml")
			if err == nil {
				l.theme.Manifest = manifest

				// Extract embedded sounds to cache directory
				pathMap, err := ExtractEmbeddedSounds(DefaultThemeName, cacheDir)
				if err == nil && len(pathMap) > 0 {
					l.theme.Dir = cacheDir
					l.updateManifestSoundPaths(pathMap)
				}
			}
		}

	}

	// Extract embedded icons to theme cache directory (not sounds cache)
	themeCacheDir, err := l.getThemeCacheDir(DefaultThemeName)
	if err == nil {
		iconsDir, err := ExtractEmbeddedIcons(DefaultThemeName, themeCacheDir)
		if err == nil && iconsDir != "" {
			l.theme.IconsDir = iconsDir
			l.logger.Debug("extracted embedded icons", "theme", DefaultThemeName, "icons_dir", iconsDir)
		}
	}

	// Load embedded aliases, symbols, and GTK icons for default theme
	if aliasesData, found := GetEmbeddedAliases(DefaultThemeName); found {
		aliasesFile := parseEmbeddedAliasesFile(aliasesData)
		if aliasesFile != nil {
			l.theme.Aliases = aliasesFile.Aliases
			l.theme.Symbols = aliasesFile.Symbols
			l.theme.GtkIcons = aliasesFile.GtkIcons
		}
	}

	l.provider.LoadFromString(processedCSS)
	l.currentName = DefaultThemeName
	l.logger.Info("loaded default theme", "has_manifest", l.theme.Manifest != nil,
		"has_aliases", len(l.theme.Aliases) > 0,
		"has_icons", l.theme.IconsDir != "")
	return nil
}

// GetTheme returns the currently loaded theme.
func (l *Loader) GetTheme() *Theme {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.theme
}

// Apply applies the loaded theme to a display.
// This should be called after the GTK application is initialized.
func (l *Loader) Apply(display *gdk.Display) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if display == nil {
		display = gdk.DisplayGetDefault()
	}
	if display == nil {
		l.logger.Warn("no display available, cannot apply theme")
		return
	}

	l.display = display
	gtk.StyleContextAddProviderForDisplay(
		display,
		l.provider,
		gtk.STYLE_PROVIDER_PRIORITY_APPLICATION,
	)
	l.logger.Debug("applied theme to display", "name", l.currentName)
}

// StartHotReload starts watching the current theme for changes.
// Changes are automatically applied to the display.
// Also watches for user theme overrides of bundled themes.
func (l *Loader) StartHotReload(ctx context.Context) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Stop existing watcher if any
	if l.watcher != nil {
		l.watcher.Stop()
	}

	l.watcher = NewWatcher(l.theme, l.logger)

	// Handle theme file changes
	// Must use glib.IdleAdd to marshal GTK operations to the main thread
	l.watcher.SetChangeCallback(func(css string, changedFile string) {
		glib.IdleAdd(func() {
			l.mu.Lock()
			l.provider.LoadFromString(css)
			callback := l.onHotReload
			themeName := l.currentName
			l.mu.Unlock()
			l.logger.Info("hot-reloaded theme", "name", themeName)
			if callback != nil {
				callback(themeName, changedFile)
			}
		})
	})

	// Handle user override detection for bundled themes
	// Must use glib.IdleAdd to marshal GTK operations to the main thread
	l.watcher.SetOverrideCallback(func(name string, path string) {
		glib.IdleAdd(func() {
			// A user created a theme file that overrides the current bundled theme
			// Reload to pick up the user version
			l.logger.Info("switching to user override of bundled theme", "name", name, "path", path)
			if err := l.LoadTheme(name); err != nil {
				l.logger.Warn("failed to load user override theme", "name", name, "error", err)
			} else {
				// Update watcher to watch new theme's imported files
				l.RefreshWatcher()
			}
		})
	})

	if err := l.watcher.Start(ctx); err != nil {
		l.logger.Warn("failed to start theme watcher", "error", err)
	}
}

// StopHotReload stops watching the theme for changes.
func (l *Loader) StopHotReload() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.watcher != nil {
		l.watcher.Stop()
		l.watcher = nil
	}
}

// RefreshWatcher updates the watcher to watch the current theme's imported files.
// This should be called after LoadTheme when hot-reload is already running.
func (l *Loader) RefreshWatcher() {
	l.mu.RLock()
	watcher := l.watcher
	theme := l.theme
	l.mu.RUnlock()

	if watcher != nil && theme != nil {
		watcher.SetTheme(theme)
	}
}

// SetHotReloadCallback sets a callback to be invoked when theme CSS is hot-reloaded.
// The callback receives the theme name and the changed file path (empty for main theme file).
func (l *Loader) SetHotReloadCallback(callback func(themeName string, changedFile string)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.onHotReload = callback
}

// ApplyFontOverrides applies font family and size overrides via a separate CSS provider.
// This provider has higher priority than the theme, so it overrides theme font settings.
// Empty fontFamily or zero fontSize means use theme default.
func (l *Loader) ApplyFontOverrides(fontFamily string, fontSize int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.fontFamily = fontFamily
	l.fontSize = fontSize

	// Generate CSS for font overrides
	css := l.generateFontCSS(fontFamily, fontSize)
	if css == "" {
		// No overrides - remove font provider if it exists
		if l.fontProvider != nil && l.display != nil {
			gtk.StyleContextRemoveProviderForDisplay(l.display, l.fontProvider)
			l.fontProvider = nil
		}
		return
	}

	// Create or update font provider
	if l.fontProvider == nil {
		l.fontProvider = gtk.NewCSSProvider()
	}
	l.fontProvider.LoadFromString(css)

	// Apply with higher priority than theme
	if l.display != nil {
		gtk.StyleContextAddProviderForDisplay(
			l.display,
			l.fontProvider,
			gtk.STYLE_PROVIDER_PRIORITY_APPLICATION+1, // Higher than theme
		)
	}

	l.logger.Debug("applied font overrides", "family", fontFamily, "size", fontSize)
}

// generateFontCSS generates CSS for font overrides.
func (l *Loader) generateFontCSS(fontFamily string, fontSize int) string {
	if fontFamily == "" && fontSize == 0 {
		return ""
	}

	var css string
	css = "* {\n"
	if fontFamily != "" {
		css += fmt.Sprintf("    --histui-font-family: %q;\n", fontFamily)
	}
	if fontSize > 0 {
		css += fmt.Sprintf("    --histui-font-size: %dpx;\n", fontSize)
	}
	css += "}\n"

	return css
}

// getSoundsCacheDir returns the cache directory for a theme's extracted sounds.
func (l *Loader) getSoundsCacheDir(themeName string) (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "histui", "themes", themeName, "sounds"), nil
}

// getThemeCacheDir returns the cache directory for a theme's extracted assets.
func (l *Loader) getThemeCacheDir(themeName string) (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "histui", "themes", themeName), nil
}

// updateManifestSoundPaths updates the manifest's sound paths using the path map.
// The pathMap maps relative paths (e.g., "sounds/normal.wav") to absolute paths.
func (l *Loader) updateManifestSoundPaths(pathMap map[string]string) {
	if l.theme == nil || l.theme.Manifest == nil {
		return
	}

	m := l.theme.Manifest

	// Update each urgency level's sound path
	if newPath, ok := pathMap[m.Audio.Low.Path]; ok {
		m.Audio.Low.Path = newPath
	}
	if newPath, ok := pathMap[m.Audio.Normal.Path]; ok {
		m.Audio.Normal.Path = newPath
	}
	if newPath, ok := pathMap[m.Audio.Critical.Path]; ok {
		m.Audio.Critical.Path = newPath
	}
}

// applyDefaults applies default theme settings to a user theme.
// This enables partial themes that only override specific elements:
//   - Layout: uses default's layout.xml if user theme has none
//   - Manifest: merges with default's manifest (sounds, icons)
//   - Sounds: extracts default sounds for inherited sound paths
func (l *Loader) applyDefaults(theme *Theme) {
	if theme == nil {
		return
	}

	// Load layout from theme directory if present, otherwise use default
	if theme.Dir != "" {
		layoutPath := filepath.Join(theme.Dir, "layout.xml")
		if data, err := os.ReadFile(layoutPath); err == nil {
			theme.Layout = string(data)
			l.logger.Debug("loaded theme layout", "theme", theme.Name)
		}
	}
	// Layout fallback happens in Theme.GetLayout()

	// Get the default manifest for merging
	defaultManifest := GetEmbeddedDefaultManifest()
	if defaultManifest == nil {
		l.logger.Debug("no default manifest available for merging")
		return
	}

	// If theme has no manifest, create one from defaults
	if theme.Manifest == nil {
		theme.Manifest = defaultManifest
		l.logger.Debug("using default manifest for theme", "theme", theme.Name)
	} else {
		// Merge theme's manifest with defaults (theme values take precedence)
		theme.Manifest.MergeWith(defaultManifest)
		l.logger.Debug("merged theme manifest with defaults", "theme", theme.Name)
	}

	// Extract default sounds for any inherited sound paths
	// (paths that still reference "sounds/..." from the default theme)
	l.extractInheritedSounds(theme, defaultManifest)
}

// extractInheritedSounds extracts embedded default sounds for paths that were inherited.
// This is needed when a user theme inherits sounds from the default theme.
func (l *Loader) extractInheritedSounds(theme *Theme, defaultManifest *Manifest) {
	if theme.Manifest == nil || defaultManifest == nil {
		return
	}

	// Check each sound config - if it matches the default path and isn't an absolute path,
	// it was inherited and we need to extract the embedded sound
	needsExtraction := false
	soundsToCheck := []struct {
		themePath   string
		defaultPath string
	}{
		{theme.Manifest.Audio.Low.Path, defaultManifest.Audio.Low.Path},
		{theme.Manifest.Audio.Normal.Path, defaultManifest.Audio.Normal.Path},
		{theme.Manifest.Audio.Critical.Path, defaultManifest.Audio.Critical.Path},
	}

	for _, s := range soundsToCheck {
		if s.themePath != "" && s.themePath == s.defaultPath && !filepath.IsAbs(s.themePath) {
			needsExtraction = true
			break
		}
	}

	if !needsExtraction {
		return
	}

	// Extract default theme's embedded sounds
	cacheDir, err := l.getSoundsCacheDir(DefaultThemeName)
	if err != nil {
		l.logger.Warn("failed to get sounds cache dir", "error", err)
		return
	}

	pathMap, err := ExtractEmbeddedSounds(DefaultThemeName, cacheDir)
	if err != nil {
		l.logger.Warn("failed to extract inherited sounds", "error", err)
		return
	}

	if len(pathMap) == 0 {
		return
	}

	// Update inherited sound paths to point to extracted files
	m := theme.Manifest
	if newPath, ok := pathMap[m.Audio.Low.Path]; ok {
		m.Audio.Low.Path = newPath
	}
	if newPath, ok := pathMap[m.Audio.Normal.Path]; ok {
		m.Audio.Normal.Path = newPath
	}
	if newPath, ok := pathMap[m.Audio.Critical.Path]; ok {
		m.Audio.Critical.Path = newPath
	}

	l.logger.Debug("extracted inherited sounds from default theme",
		"theme", theme.Name, "count", len(pathMap))
}

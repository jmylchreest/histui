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
)

// Loader handles loading and applying CSS themes with hot-reload support.
type Loader struct {
	mu            sync.RWMutex
	logger        *slog.Logger
	provider      *gtk.CSSProvider
	fontProvider  *gtk.CSSProvider // Separate provider for font overrides (higher priority)
	themesDir     string
	currentName   string
	theme         *Theme
	watcher       *Watcher
	display       *gdk.Display
	fontFamily    string
	fontSize      int
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

// ThemesDir returns the path to the user's themes directory.
func ThemesDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "histui", "themes"), nil
}

// LoadTheme loads a theme by name.
// Theme resolution order:
//  1. User themes directory (~/.config/histui/themes/)
//     a. Directory-based theme (name/ with theme.css and optional manifest)
//     b. Single CSS file (name.css)
//  2. Embedded/bundled themes
//
// This allows users to override bundled themes by placing a file with the same name
// in their themes directory.
func (l *Loader) LoadTheme(name string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

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
				l.provider.LoadFromString(theme.CSS)
				l.currentName = name
				l.theme = theme
				l.logger.Info("loaded user theme (directory)", "name", name, "path", themeDir,
					"has_manifest", theme.Manifest != nil)
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
				l.provider.LoadFromString(theme.CSS)
				l.currentName = name
				l.theme = theme
				l.logger.Info("loaded user theme", "name", name, "path", themePath)
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
		l.provider.LoadFromString(processedCSS)
		l.currentName = name
		l.logger.Info("loaded bundled theme", "name", name)
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
	l.provider.LoadFromString(processedCSS)
	l.currentName = DefaultThemeName
	l.logger.Info("loaded default theme")
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

// Reload reloads the current theme from disk.
// This is useful for hot-reloading theme changes.
func (l *Loader) Reload() error {
	l.mu.RLock()
	name := l.currentName
	l.mu.RUnlock()
	return l.LoadTheme(name)
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
	l.watcher.SetChangeCallback(func(css string) {
		glib.IdleAdd(func() {
			l.mu.Lock()
			l.provider.LoadFromString(css)
			l.mu.Unlock()
			l.logger.Info("hot-reloaded theme", "name", l.currentName)
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

// Provider returns the underlying CSS provider.
func (l *Loader) Provider() *gtk.CSSProvider {
	return l.provider
}

// CurrentTheme returns the name of the currently loaded theme.
func (l *Loader) CurrentTheme() string {
	return l.currentName
}

// ListThemes returns a list of available theme names.
// Returns both bundled themes and user themes, with duplicates removed.
func (l *Loader) ListThemes() []string {
	seen := make(map[string]bool)
	var themes []string

	// Add bundled themes first
	for _, name := range ListEmbeddedThemes() {
		if !seen[name] {
			seen[name] = true
			themes = append(themes, name)
		}
	}

	// Add user themes (may include overrides)
	if l.themesDir != "" {
		entries, err := os.ReadDir(l.themesDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				name := entry.Name()
				if filepath.Ext(name) == ".css" {
					themeName := name[:len(name)-4]
					if !seen[themeName] {
						seen[themeName] = true
						themes = append(themes, themeName)
					}
				}
			}
		} else {
			l.logger.Debug("failed to read themes directory", "error", err)
		}
	}

	return themes
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
	css = ":root {\n"
	if fontFamily != "" {
		css += fmt.Sprintf("    --histui-font-family: %q;\n", fontFamily)
	}
	if fontSize > 0 {
		css += fmt.Sprintf("    --histui-font-size: %dpx;\n", fontSize)
	}
	css += "}\n"

	return css
}

// GetFontSettings returns the current font override settings.
func (l *Loader) GetFontSettings() (fontFamily string, fontSize int) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.fontFamily, l.fontSize
}

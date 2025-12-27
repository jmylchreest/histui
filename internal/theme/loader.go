package theme

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// Loader handles loading and applying CSS themes with hot-reload support.
type Loader struct {
	mu          sync.RWMutex
	logger      *slog.Logger
	provider    *gtk.CSSProvider
	themesDir   string
	currentName string
	theme       *Theme
	watcher     *Watcher
	display     *gdk.Display
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
func (l *Loader) StartHotReload(ctx context.Context) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.theme == nil || l.theme.IsDefault {
		l.logger.Debug("not starting hot-reload for default theme")
		return
	}

	// Stop existing watcher if any
	if l.watcher != nil {
		l.watcher.Stop()
	}

	l.watcher = NewWatcher(l.theme, l.logger)
	l.watcher.SetChangeCallback(func(css string) {
		l.mu.Lock()
		l.provider.LoadFromString(css)
		l.mu.Unlock()
		l.logger.Info("hot-reloaded theme", "name", l.currentName)
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

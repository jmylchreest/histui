package theme

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// Watcher watches a theme file for changes and triggers hot-reload.
// Uses inotify (via fsnotify) for efficient file system event notification.
// Also watches for user theme overrides of bundled themes.
type Watcher struct {
	mu       sync.RWMutex
	logger   *slog.Logger
	watcher  *fsnotify.Watcher
	watchErr error

	// Theme being watched
	theme *Theme

	// User themes directory (for watching overrides)
	themesDir string
	absPath   string

	// Callbacks
	onChangeCallback   func(css string)
	onOverrideCallback func(name string, path string)

	// Control channels
	stopCh chan struct{}
	doneCh chan struct{}

	running bool
}

// NewWatcher creates a new theme watcher.
func NewWatcher(theme *Theme, logger *slog.Logger) *Watcher {
	if logger == nil {
		logger = slog.Default()
	}

	w := &Watcher{
		logger: logger,
		theme:  theme,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}

	// Get themes directory
	if dir, err := ThemesDir(); err == nil {
		w.themesDir = dir
	}

	// Calculate the user theme path for the current theme
	if w.themesDir != "" && theme != nil {
		w.absPath = filepath.Join(w.themesDir, theme.Name+".css")
	}

	// Create fsnotify watcher
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		w.watchErr = err
		logger.Warn("failed to create inotify watcher for theme", "error", err)
	} else {
		w.watcher = fsw
	}

	return w
}

// SetChangeCallback sets the callback to invoke when the theme changes.
// The callback receives the new CSS content.
func (w *Watcher) SetChangeCallback(callback func(css string)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onChangeCallback = callback
}

// SetOverrideCallback sets the callback to invoke when a user override is detected.
// The callback receives the theme name and new path.
// This is called when a user creates a theme file that overrides a bundled theme.
func (w *Watcher) SetOverrideCallback(callback func(name string, path string)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onOverrideCallback = callback
}

// Start begins watching the theme for changes.
func (w *Watcher) Start(ctx context.Context) error {
	if w.watcher == nil {
		w.logger.Warn("inotify watcher not available, theme hot-reload disabled")
		return nil
	}

	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return nil
	}
	w.running = true
	w.stopCh = make(chan struct{})
	w.doneCh = make(chan struct{})
	theme := w.theme
	themesDir := w.themesDir
	w.mu.Unlock()

	// Watch the user themes directory for overrides
	if themesDir != "" {
		// Ensure directory exists
		if err := os.MkdirAll(themesDir, 0755); err == nil {
			if err := w.watcher.Add(themesDir); err != nil {
				w.logger.Debug("failed to watch themes directory", "dir", themesDir, "error", err)
			} else {
				w.logger.Debug("watching themes directory for overrides", "dir", themesDir)
			}
		}
	}

	// Also watch the theme file's directory if it's a user theme
	if theme != nil && theme.Path != "" && !theme.IsDefault {
		dir := filepath.Dir(theme.Path)
		if dir != themesDir { // Avoid adding the same directory twice
			if err := w.watcher.Add(dir); err != nil {
				w.logger.Debug("failed to watch theme directory", "dir", dir, "error", err)
			}
		}
	}

	go w.eventLoop(ctx)

	w.logger.Debug("theme inotify watcher started", "theme", theme.Name)
	return nil
}

// Stop stops watching the theme file.
func (w *Watcher) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		if w.watcher != nil {
			_ = w.watcher.Close()
		}
		return
	}
	w.running = false
	close(w.stopCh)
	w.mu.Unlock()

	<-w.doneCh

	if w.watcher != nil {
		_ = w.watcher.Close()
	}

	w.logger.Debug("theme inotify watcher stopped")
}

// UpdateTheme switches to watching a different theme.
func (w *Watcher) UpdateTheme(theme *Theme) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.theme = theme
	if w.themesDir != "" && theme != nil {
		w.absPath = filepath.Join(w.themesDir, theme.Name+".css")
	}
}

// eventLoop processes inotify events.
func (w *Watcher) eventLoop(ctx context.Context) {
	defer close(w.doneCh)

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			w.logger.Warn("theme inotify watcher error", "error", err)
		}
	}
}

// handleEvent processes a single file system event.
func (w *Watcher) handleEvent(event fsnotify.Event) {
	// Only care about write and create events
	if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
		return
	}

	// Skip non-CSS files
	if filepath.Ext(event.Name) != ".css" {
		return
	}

	eventPath, err := filepath.Abs(event.Name)
	if err != nil {
		return
	}

	w.mu.RLock()
	theme := w.theme
	absPath := w.absPath
	changeCallback := w.onChangeCallback
	overrideCallback := w.onOverrideCallback
	w.mu.RUnlock()

	if theme == nil {
		return
	}

	// Case 1: Current user theme file was modified
	if theme.Path != "" && !theme.IsDefault {
		themePath, _ := filepath.Abs(theme.Path)
		if eventPath == themePath {
			w.logger.Debug("theme file changed", "path", theme.Path, "op", event.Op.String())
			changed, err := theme.Reload()
			if err != nil {
				w.logger.Warn("failed to reload theme", "path", theme.Path, "error", err)
				return
			}
			if changed && changeCallback != nil {
				w.logger.Info("hot-reloaded theme", "name", theme.Name)
				changeCallback(theme.CSS)
			}
			return
		}
	}

	// Case 2: A user override file was created/modified for a bundled theme
	// Check if the modified file matches the current theme name
	if absPath != "" && eventPath == absPath {
		// User created/modified a theme with the same name as the current theme
		if theme.IsDefault || theme.Path == "" {
			// Currently using a bundled theme - user created an override
			w.logger.Info("detected user override for bundled theme", "name", theme.Name, "path", eventPath)
			if overrideCallback != nil {
				overrideCallback(theme.Name, eventPath)
			}
		} else {
			// Currently using a user theme - it was modified
			w.logger.Debug("user theme file changed", "path", eventPath, "op", event.Op.String())
			changed, err := theme.Reload()
			if err != nil {
				w.logger.Warn("failed to reload theme", "path", eventPath, "error", err)
				return
			}
			if changed && changeCallback != nil {
				w.logger.Info("hot-reloaded user theme", "name", theme.Name)
				changeCallback(theme.CSS)
			}
		}
		return
	}

	// Case 3: A different CSS file in the themes directory was modified
	// We don't care about this unless it matches the current theme name
}

// IsRunning returns whether the watcher is currently running.
func (w *Watcher) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

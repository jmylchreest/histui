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
	onChangeCallback   func(css string, changedFile string)
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
// The callback receives the new CSS content and the path of the file that changed
// (empty string if it was the main theme file).
func (w *Watcher) SetChangeCallback(callback func(css string, changedFile string)) {
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

	// Watch directories containing imported files
	if theme != nil {
		watchedDirs := make(map[string]bool)
		watchedDirs[themesDir] = true // Already watching
		if theme.Path != "" {
			watchedDirs[filepath.Dir(theme.Path)] = true // Already watching
		}

		for _, importPath := range theme.ImportedFiles {
			dir := filepath.Dir(importPath)
			if !watchedDirs[dir] {
				if err := w.watcher.Add(dir); err != nil {
					w.logger.Debug("failed to watch imported file directory", "dir", dir, "error", err)
				} else {
					w.logger.Debug("watching imported file directory", "dir", dir)
					watchedDirs[dir] = true
				}
			}
		}

		if len(theme.ImportedFiles) > 0 {
			w.logger.Debug("watching imported files", "count", len(theme.ImportedFiles), "files", theme.ImportedFiles)
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

// SetTheme updates the theme being watched and refreshes the watch list.
// This should be called when the theme changes (e.g., via config reload).
func (w *Watcher) SetTheme(theme *Theme) {
	if theme == nil {
		return
	}

	w.mu.Lock()
	w.theme = theme
	if w.themesDir != "" {
		w.absPath = filepath.Join(w.themesDir, theme.Name+".css")
	}
	w.mu.Unlock()

	// Update watched directories for the new theme
	w.UpdateWatches()

	w.logger.Debug("theme watcher updated for new theme", "name", theme.Name, "imported_count", len(theme.ImportedFiles))
}

// UpdateWatches updates the watched directories based on the current theme's imported files.
// This should be called after ForceReload to add/remove watches as needed.
func (w *Watcher) UpdateWatches() {
	if w.watcher == nil {
		return
	}

	w.mu.RLock()
	theme := w.theme
	themesDir := w.themesDir
	w.mu.RUnlock()

	if theme == nil {
		return
	}

	// Get the set of directories that should be watched
	requiredDirs := make(map[string]bool)
	requiredDirs[themesDir] = true
	if theme.Path != "" && !theme.IsDefault {
		requiredDirs[filepath.Dir(theme.Path)] = true
	}
	for _, importPath := range theme.ImportedFiles {
		requiredDirs[filepath.Dir(importPath)] = true
	}

	// Get current watched directories
	currentDirs := make(map[string]bool)
	for _, path := range w.watcher.WatchList() {
		currentDirs[path] = true
	}

	// Remove watches for directories no longer needed
	for dir := range currentDirs {
		if !requiredDirs[dir] {
			if err := w.watcher.Remove(dir); err != nil {
				w.logger.Debug("failed to remove watch for directory", "dir", dir, "error", err)
			} else {
				w.logger.Debug("removed watch for directory no longer imported", "dir", dir)
			}
		}
	}

	// Add watches for new directories
	for dir := range requiredDirs {
		if !currentDirs[dir] {
			if err := w.watcher.Add(dir); err != nil {
				w.logger.Debug("failed to add watch for new imported directory", "dir", dir, "error", err)
			} else {
				w.logger.Debug("added watch for newly imported directory", "dir", dir)
			}
		}
	}

	if len(theme.ImportedFiles) > 0 {
		w.logger.Debug("watch list updated", "imported_count", len(theme.ImportedFiles), "files", theme.ImportedFiles)
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
			if changed {
				// Update watches in case imports changed
				w.UpdateWatches()
				if changeCallback != nil {
					w.logger.Info("hot-reloaded theme", "name", theme.Name)
					changeCallback(theme.CSS, "")
				}
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
			if changed {
				// Update watches in case imports changed
				w.UpdateWatches()
				if changeCallback != nil {
					w.logger.Info("hot-reloaded user theme", "name", theme.Name)
					changeCallback(theme.CSS, "")
				}
			}
		}
		return
	}

	// Case 3: An imported file was modified
	if theme.IsImportedFile(eventPath) {
		w.logger.Debug("imported file changed", "path", eventPath, "op", event.Op.String())
		// Use ForceReload since the main theme file hasn't changed
		changed, err := theme.ForceReload()
		if err != nil {
			w.logger.Warn("failed to reload theme after imported file change", "path", eventPath, "error", err)
			return
		}
		if changed {
			// Update watches in case the imported file's imports changed
			w.UpdateWatches()
			if changeCallback != nil {
				w.logger.Info("hot-reloaded theme (imported file changed)", "name", theme.Name, "imported_file", filepath.Base(eventPath))
				changeCallback(theme.CSS, eventPath)
			}
		}
		return
	}

	// Case 4: A different CSS file in the themes directory was modified
	// We don't care about this unless it matches the current theme name
}

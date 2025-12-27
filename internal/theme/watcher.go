package theme

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"
)

// Watcher watches a theme file for changes and triggers hot-reload.
type Watcher struct {
	mu     sync.RWMutex
	logger *slog.Logger

	// Theme being watched
	theme *Theme

	// Polling interval
	pollInterval time.Duration

	// Callback for changes
	onChangeCallback func(css string)

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

	return &Watcher{
		logger:       logger,
		theme:        theme,
		pollInterval: 1 * time.Second, // Check every second
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}
}

// SetPollInterval sets the polling interval for file changes.
func (w *Watcher) SetPollInterval(interval time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pollInterval = interval
}

// SetChangeCallback sets the callback to invoke when the theme changes.
// The callback receives the new CSS content.
func (w *Watcher) SetChangeCallback(callback func(css string)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onChangeCallback = callback
}

// Start begins watching the theme file for changes.
func (w *Watcher) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return nil
	}

	// Don't watch the default theme (it's embedded)
	if w.theme.IsDefault {
		w.mu.Unlock()
		w.logger.Debug("not watching default theme (embedded)")
		return nil
	}

	w.running = true
	w.stopCh = make(chan struct{})
	w.doneCh = make(chan struct{})
	w.mu.Unlock()

	go w.watchLoop(ctx)

	w.logger.Debug("theme watcher started", "path", w.theme.Path, "interval", w.pollInterval)
	return nil
}

// Stop stops watching the theme file.
func (w *Watcher) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.running = false
	close(w.stopCh)
	w.mu.Unlock()

	// Wait for goroutine to finish
	<-w.doneCh
	w.logger.Debug("theme watcher stopped")
}

// UpdateTheme switches to watching a different theme.
func (w *Watcher) UpdateTheme(theme *Theme) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.theme = theme
}

// watchLoop is the main polling loop.
func (w *Watcher) watchLoop(ctx context.Context) {
	defer close(w.doneCh)

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.checkForChanges()
		}
	}
}

// checkForChanges checks if the theme file has been modified.
func (w *Watcher) checkForChanges() {
	w.mu.RLock()
	theme := w.theme
	callback := w.onChangeCallback
	w.mu.RUnlock()

	if theme == nil || theme.IsDefault {
		return
	}

	// Check if file still exists
	if _, err := os.Stat(theme.Path); err != nil {
		if os.IsNotExist(err) {
			w.logger.Debug("theme file no longer exists", "path", theme.Path)
		}
		return
	}

	// Try to reload
	changed, err := theme.Reload()
	if err != nil {
		w.logger.Warn("failed to reload theme", "path", theme.Path, "error", err)
		return
	}

	if changed {
		w.logger.Info("theme file changed, reloading", "path", theme.Path)
		if callback != nil {
			callback(theme.CSS)
		}
	}
}

// IsRunning returns whether the watcher is currently running.
func (w *Watcher) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

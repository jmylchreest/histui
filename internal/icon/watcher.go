package icon

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// AliasesWatcher watches the user's icon-aliases.toml file for changes.
// Uses inotify (via fsnotify) for efficient file system event notification.
type AliasesWatcher struct {
	mu       sync.RWMutex
	logger   *slog.Logger
	watcher  *fsnotify.Watcher
	watchErr error

	// Path to watch
	aliasesPath string
	absPath     string
	dir         string

	// Callback for changes
	onReloadCallback func(aliases map[string]string)
	onErrorCallback  func(err error)

	// Control channels
	stopCh chan struct{}
	doneCh chan struct{}

	running bool
}

// NewAliasesWatcher creates a new AliasesWatcher for the user's icon-aliases.toml.
func NewAliasesWatcher(logger *slog.Logger) (*AliasesWatcher, error) {
	aliasesPath, err := UserAliasesPath()
	if err != nil {
		return nil, err
	}

	w := &AliasesWatcher{
		logger:      logger,
		aliasesPath: aliasesPath,
		stopCh:      make(chan struct{}),
		doneCh:      make(chan struct{}),
	}

	// Resolve absolute path
	if absPath, err := filepath.Abs(aliasesPath); err == nil {
		w.absPath = absPath
		w.dir = filepath.Dir(absPath)
	}

	// Create fsnotify watcher
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		w.watchErr = err
		logger.Warn("failed to create inotify watcher for icon aliases", "error", err)
	} else {
		w.watcher = fsw
	}

	return w, nil
}

// SetReloadCallback sets the callback to invoke when aliases are successfully reloaded.
// The callback receives the new aliases map.
func (w *AliasesWatcher) SetReloadCallback(callback func(aliases map[string]string)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onReloadCallback = callback
}

// SetErrorCallback sets the callback to invoke when alias reload fails.
func (w *AliasesWatcher) SetErrorCallback(callback func(err error)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onErrorCallback = callback
}

// Start begins watching the icon aliases file for changes.
func (w *AliasesWatcher) Start(ctx context.Context) error {
	if w.watcher == nil {
		w.logger.Warn("inotify watcher not available, icon aliases file watching disabled")
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

	// Ensure the directory exists
	if w.dir != "" {
		if err := os.MkdirAll(w.dir, 0755); err != nil {
			w.mu.Unlock()
			return err
		}
	}

	w.mu.Unlock()

	// Watch the directory containing the file
	if err := w.watcher.Add(w.dir); err != nil {
		w.logger.Warn("failed to watch icon aliases directory", "dir", w.dir, "error", err)
		return err
	}

	go w.eventLoop(ctx)

	w.logger.Debug("icon aliases inotify watcher started", "path", w.aliasesPath)
	return nil
}

// Stop stops watching the aliases file.
func (w *AliasesWatcher) Stop() {
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

	w.logger.Debug("icon aliases inotify watcher stopped")
}

// eventLoop processes inotify events.
func (w *AliasesWatcher) eventLoop(ctx context.Context) {
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
			w.logger.Warn("icon aliases inotify watcher error", "error", err)
		}
	}
}

// handleEvent processes a single file system event.
func (w *AliasesWatcher) handleEvent(event fsnotify.Event) {
	// Check if this is our file
	eventPath, err := filepath.Abs(event.Name)
	if err != nil {
		return
	}

	if eventPath != w.absPath {
		return
	}

	// Only care about write and create events
	if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
		w.logger.Debug("icon aliases file changed", "path", w.aliasesPath, "op", event.Op.String())

		w.mu.RLock()
		reloadCallback := w.onReloadCallback
		errorCallback := w.onErrorCallback
		w.mu.RUnlock()

		// Try to load the new aliases
		aliases, err := LoadUserAliases()
		if err != nil {
			w.logger.Warn("icon aliases file changed but loading failed", "error", err)
			if errorCallback != nil {
				errorCallback(err)
			}
			return
		}

		w.logger.Info("icon aliases reloaded successfully", "count", len(aliases))
		if reloadCallback != nil {
			reloadCallback(aliases)
		}
	}
}

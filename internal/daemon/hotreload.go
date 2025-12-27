// Package daemon provides the main orchestration for histuid.
package daemon

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"

	"github.com/jmylchreest/histui/internal/config"
)

// StoreWatcher watches the notification history file for external changes.
// This allows histuid to detect when notifications are dismissed via histui CLI.
// Uses inotify (via fsnotify) for efficient file system event notification.
type StoreWatcher struct {
	mu       sync.RWMutex
	logger   *slog.Logger
	watcher  *fsnotify.Watcher
	watchErr error

	// Path to watch
	historyPath string
	absPath     string
	dir         string

	// Callback for changes
	onChangeCallback func()

	// Control channels
	stopCh chan struct{}
	doneCh chan struct{}

	running bool
}

// NewStoreWatcher creates a new StoreWatcher for the given history file path.
func NewStoreWatcher(historyPath string, logger *slog.Logger) *StoreWatcher {
	w := &StoreWatcher{
		logger:      logger,
		historyPath: historyPath,
		stopCh:      make(chan struct{}),
		doneCh:      make(chan struct{}),
	}

	// Resolve absolute path
	if absPath, err := filepath.Abs(historyPath); err == nil {
		w.absPath = absPath
		w.dir = filepath.Dir(absPath)
	}

	// Create fsnotify watcher
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		w.watchErr = err
		logger.Warn("failed to create inotify watcher for store", "error", err)
	} else {
		w.watcher = fsw
	}

	return w
}

// SetChangeCallback sets the callback to invoke when the store file changes.
func (w *StoreWatcher) SetChangeCallback(callback func()) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onChangeCallback = callback
}

// Start begins watching the store file for changes.
func (w *StoreWatcher) Start(ctx context.Context) error {
	if w.watcher == nil {
		w.logger.Warn("inotify watcher not available, store file watching disabled")
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
		w.logger.Warn("failed to watch store directory", "dir", w.dir, "error", err)
		return err
	}

	go w.eventLoop(ctx)

	w.logger.Debug("store inotify watcher started", "path", w.historyPath)
	return nil
}

// Stop stops watching the store file.
func (w *StoreWatcher) Stop() {
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

	w.logger.Debug("store inotify watcher stopped")
}

// eventLoop processes inotify events.
func (w *StoreWatcher) eventLoop(ctx context.Context) {
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
			w.logger.Warn("store inotify watcher error", "error", err)
		}
	}
}

// handleEvent processes a single file system event.
func (w *StoreWatcher) handleEvent(event fsnotify.Event) {
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
		w.logger.Debug("store file changed", "path", w.historyPath, "op", event.Op.String())

		w.mu.RLock()
		callback := w.onChangeCallback
		w.mu.RUnlock()

		if callback != nil {
			callback()
		}
	}
}

// StateWatcher watches the shared state file for DnD changes.
// Uses inotify (via fsnotify) for efficient file system event notification.
type StateWatcher struct {
	mu       sync.RWMutex
	logger   *slog.Logger
	watcher  *fsnotify.Watcher
	watchErr error

	// Path to watch
	statePath string
	absPath   string
	dir       string

	// Callback for changes
	onChangeCallback func()

	// Control channels
	stopCh chan struct{}
	doneCh chan struct{}

	running bool
}

// NewStateWatcher creates a new StateWatcher for the given state file path.
func NewStateWatcher(statePath string, logger *slog.Logger) *StateWatcher {
	w := &StateWatcher{
		logger:    logger,
		statePath: statePath,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}

	// Resolve absolute path
	if absPath, err := filepath.Abs(statePath); err == nil {
		w.absPath = absPath
		w.dir = filepath.Dir(absPath)
	}

	// Create fsnotify watcher
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		w.watchErr = err
		logger.Warn("failed to create inotify watcher for state", "error", err)
	} else {
		w.watcher = fsw
	}

	return w
}

// SetChangeCallback sets the callback to invoke when the state file changes.
func (w *StateWatcher) SetChangeCallback(callback func()) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onChangeCallback = callback
}

// Start begins watching the state file for changes.
func (w *StateWatcher) Start(ctx context.Context) error {
	if w.watcher == nil {
		w.logger.Warn("inotify watcher not available, state file watching disabled")
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
		w.logger.Warn("failed to watch state directory", "dir", w.dir, "error", err)
		return err
	}

	go w.eventLoop(ctx)

	w.logger.Debug("state inotify watcher started", "path", w.statePath)
	return nil
}

// Stop stops watching the state file.
func (w *StateWatcher) Stop() {
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

	w.logger.Debug("state inotify watcher stopped")
}

// eventLoop processes inotify events.
func (w *StateWatcher) eventLoop(ctx context.Context) {
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
			w.logger.Warn("state inotify watcher error", "error", err)
		}
	}
}

// handleEvent processes a single file system event.
func (w *StateWatcher) handleEvent(event fsnotify.Event) {
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
		w.logger.Debug("state file changed", "path", w.statePath, "op", event.Op.String())

		w.mu.RLock()
		callback := w.onChangeCallback
		w.mu.RUnlock()

		if callback != nil {
			callback()
		}
	}
}

// ConfigWatcher watches the daemon config file for changes and validates new configs.
// Uses inotify (via fsnotify) for efficient file system event notification.
type ConfigWatcher struct {
	mu       sync.RWMutex
	logger   *slog.Logger
	watcher  *fsnotify.Watcher
	watchErr error

	// Path to watch
	configPath string
	absPath    string
	dir        string

	// Current valid config
	currentConfig *config.DaemonConfig

	// Callbacks
	onReloadCallback func(newConfig *config.DaemonConfig)
	onErrorCallback  func(err error)

	// Control channels
	stopCh chan struct{}
	doneCh chan struct{}

	running bool
}

// NewConfigWatcher creates a new ConfigWatcher for the daemon config file.
func NewConfigWatcher(logger *slog.Logger) (*ConfigWatcher, error) {
	configPath, err := config.DaemonConfigPath()
	if err != nil {
		return nil, err
	}

	w := &ConfigWatcher{
		logger:     logger,
		configPath: configPath,
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
	}

	// Resolve absolute path
	if absPath, err := filepath.Abs(configPath); err == nil {
		w.absPath = absPath
		w.dir = filepath.Dir(absPath)
	}

	// Create fsnotify watcher
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		w.watchErr = err
		logger.Warn("failed to create inotify watcher for config", "error", err)
	} else {
		w.watcher = fsw
	}

	return w, nil
}

// SetReloadCallback sets the callback to invoke when config is successfully reloaded.
func (w *ConfigWatcher) SetReloadCallback(callback func(newConfig *config.DaemonConfig)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onReloadCallback = callback
}

// SetErrorCallback sets the callback to invoke when config reload fails validation.
func (w *ConfigWatcher) SetErrorCallback(callback func(err error)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onErrorCallback = callback
}

// Start begins watching the config file for changes.
func (w *ConfigWatcher) Start(ctx context.Context, initialConfig *config.DaemonConfig) error {
	if w.watcher == nil {
		w.logger.Warn("inotify watcher not available, config file watching disabled")
		return nil
	}

	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return nil
	}
	w.running = true
	w.currentConfig = initialConfig
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
		w.logger.Warn("failed to watch config directory", "dir", w.dir, "error", err)
		return err
	}

	go w.eventLoop(ctx)

	w.logger.Debug("config inotify watcher started", "path", w.configPath)
	return nil
}

// Stop stops watching the config file.
func (w *ConfigWatcher) Stop() {
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

	w.logger.Debug("config inotify watcher stopped")
}

// GetCurrentConfig returns the current valid configuration.
func (w *ConfigWatcher) GetCurrentConfig() *config.DaemonConfig {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.currentConfig
}

// eventLoop processes inotify events.
func (w *ConfigWatcher) eventLoop(ctx context.Context) {
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
			w.logger.Warn("config inotify watcher error", "error", err)
		}
	}
}

// handleEvent processes a single file system event.
func (w *ConfigWatcher) handleEvent(event fsnotify.Event) {
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
		w.logger.Debug("config file changed", "path", w.configPath, "op", event.Op.String())

		w.mu.RLock()
		reloadCallback := w.onReloadCallback
		errorCallback := w.onErrorCallback
		w.mu.RUnlock()

		// Try to load and validate the new config
		newConfig, err := config.LoadDaemonConfig()
		if err != nil {
			w.logger.Warn("config file changed but validation failed", "error", err)
			if errorCallback != nil {
				errorCallback(err)
			}
			return
		}

		// Config is valid - update current and notify
		w.mu.Lock()
		w.currentConfig = newConfig
		w.mu.Unlock()

		w.logger.Info("config reloaded successfully")
		if reloadCallback != nil {
			reloadCallback(newConfig)
		}
	}
}

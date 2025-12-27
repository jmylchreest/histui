// Package daemon provides the main orchestration for histuid.
package daemon

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/jmylchreest/histui/internal/config"
)

// StoreWatcher watches the notification history file for external changes.
// This allows histuid to detect when notifications are dismissed via histui CLI.
type StoreWatcher struct {
	mu     sync.RWMutex
	logger *slog.Logger

	// Path to watch
	historyPath string

	// Last known modification time
	lastModTime time.Time

	// Polling interval
	pollInterval time.Duration

	// Callback for changes
	onChangeCallback func()

	// Control channels
	stopCh chan struct{}
	doneCh chan struct{}

	running bool
}

// NewStoreWatcher creates a new StoreWatcher for the given history file path.
func NewStoreWatcher(historyPath string, logger *slog.Logger) *StoreWatcher {
	return &StoreWatcher{
		logger:       logger,
		historyPath:  historyPath,
		pollInterval: 500 * time.Millisecond, // Poll every 500ms
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}
}

// SetPollInterval sets the polling interval for file changes.
func (w *StoreWatcher) SetPollInterval(interval time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pollInterval = interval
}

// SetChangeCallback sets the callback to invoke when the store file changes.
func (w *StoreWatcher) SetChangeCallback(callback func()) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onChangeCallback = callback
}

// Start begins watching the store file for changes.
func (w *StoreWatcher) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return nil
	}
	w.running = true

	// Get initial modification time
	if info, err := os.Stat(w.historyPath); err == nil {
		w.lastModTime = info.ModTime()
	}

	w.stopCh = make(chan struct{})
	w.doneCh = make(chan struct{})
	w.mu.Unlock()

	go w.watchLoop(ctx)

	w.logger.Debug("store watcher started", "path", w.historyPath, "interval", w.pollInterval)
	return nil
}

// Stop stops watching the store file.
func (w *StoreWatcher) Stop() {
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
	w.logger.Debug("store watcher stopped")
}

// watchLoop is the main polling loop.
func (w *StoreWatcher) watchLoop(ctx context.Context) {
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

// checkForChanges checks if the store file has been modified.
func (w *StoreWatcher) checkForChanges() {
	w.mu.RLock()
	callback := w.onChangeCallback
	lastModTime := w.lastModTime
	w.mu.RUnlock()

	info, err := os.Stat(w.historyPath)
	if err != nil {
		// File might not exist yet or was deleted
		if !os.IsNotExist(err) {
			w.logger.Debug("failed to stat store file", "path", w.historyPath, "error", err)
		}
		return
	}

	modTime := info.ModTime()
	if modTime.After(lastModTime) {
		w.mu.Lock()
		w.lastModTime = modTime
		w.mu.Unlock()

		w.logger.Debug("store file changed", "path", w.historyPath, "modTime", modTime)

		if callback != nil {
			callback()
		}
	}
}

// StateWatcher watches the shared state file for DnD changes.
type StateWatcher struct {
	mu     sync.RWMutex
	logger *slog.Logger

	// Path to watch
	statePath string

	// Last known modification time
	lastModTime time.Time

	// Polling interval
	pollInterval time.Duration

	// Callback for changes
	onChangeCallback func()

	// Control channels
	stopCh chan struct{}
	doneCh chan struct{}

	running bool
}

// NewStateWatcher creates a new StateWatcher for the given state file path.
func NewStateWatcher(statePath string, logger *slog.Logger) *StateWatcher {
	return &StateWatcher{
		logger:       logger,
		statePath:    statePath,
		pollInterval: 500 * time.Millisecond, // Poll every 500ms
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}
}

// SetPollInterval sets the polling interval for file changes.
func (w *StateWatcher) SetPollInterval(interval time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pollInterval = interval
}

// SetChangeCallback sets the callback to invoke when the state file changes.
func (w *StateWatcher) SetChangeCallback(callback func()) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onChangeCallback = callback
}

// Start begins watching the state file for changes.
func (w *StateWatcher) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return nil
	}
	w.running = true

	// Get initial modification time
	if info, err := os.Stat(w.statePath); err == nil {
		w.lastModTime = info.ModTime()
	}

	w.stopCh = make(chan struct{})
	w.doneCh = make(chan struct{})
	w.mu.Unlock()

	go w.watchLoop(ctx)

	w.logger.Debug("state watcher started", "path", w.statePath, "interval", w.pollInterval)
	return nil
}

// Stop stops watching the state file.
func (w *StateWatcher) Stop() {
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
	w.logger.Debug("state watcher stopped")
}

// watchLoop is the main polling loop.
func (w *StateWatcher) watchLoop(ctx context.Context) {
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

// checkForChanges checks if the state file has been modified.
func (w *StateWatcher) checkForChanges() {
	w.mu.RLock()
	callback := w.onChangeCallback
	lastModTime := w.lastModTime
	w.mu.RUnlock()

	info, err := os.Stat(w.statePath)
	if err != nil {
		// File might not exist yet or was deleted
		if !os.IsNotExist(err) {
			w.logger.Debug("failed to stat state file", "path", w.statePath, "error", err)
		}
		return
	}

	modTime := info.ModTime()
	if modTime.After(lastModTime) {
		w.mu.Lock()
		w.lastModTime = modTime
		w.mu.Unlock()

		w.logger.Debug("state file changed", "path", w.statePath, "modTime", modTime)

		if callback != nil {
			callback()
		}
	}
}

// ConfigWatcher watches the daemon config file for changes and validates new configs.
type ConfigWatcher struct {
	mu     sync.RWMutex
	logger *slog.Logger

	// Path to watch
	configPath string

	// Last known modification time
	lastModTime time.Time

	// Current valid config
	currentConfig *config.DaemonConfig

	// Polling interval
	pollInterval time.Duration

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

	return &ConfigWatcher{
		logger:       logger,
		configPath:   configPath,
		pollInterval: 1 * time.Second, // Poll every second
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}, nil
}

// SetPollInterval sets the polling interval for file changes.
func (w *ConfigWatcher) SetPollInterval(interval time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pollInterval = interval
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
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return nil
	}
	w.running = true
	w.currentConfig = initialConfig

	// Get initial modification time
	if info, err := os.Stat(w.configPath); err == nil {
		w.lastModTime = info.ModTime()
	}

	w.stopCh = make(chan struct{})
	w.doneCh = make(chan struct{})
	w.mu.Unlock()

	go w.watchLoop(ctx)

	w.logger.Debug("config watcher started", "path", w.configPath, "interval", w.pollInterval)
	return nil
}

// Stop stops watching the config file.
func (w *ConfigWatcher) Stop() {
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
	w.logger.Debug("config watcher stopped")
}

// GetCurrentConfig returns the current valid configuration.
func (w *ConfigWatcher) GetCurrentConfig() *config.DaemonConfig {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.currentConfig
}

// watchLoop is the main polling loop.
func (w *ConfigWatcher) watchLoop(ctx context.Context) {
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

// checkForChanges checks if the config file has been modified.
func (w *ConfigWatcher) checkForChanges() {
	w.mu.RLock()
	reloadCallback := w.onReloadCallback
	errorCallback := w.onErrorCallback
	lastModTime := w.lastModTime
	w.mu.RUnlock()

	info, err := os.Stat(w.configPath)
	if err != nil {
		// File might not exist yet or was deleted
		if !os.IsNotExist(err) {
			w.logger.Debug("failed to stat config file", "path", w.configPath, "error", err)
		}
		return
	}

	modTime := info.ModTime()
	if modTime.After(lastModTime) {
		w.mu.Lock()
		w.lastModTime = modTime
		w.mu.Unlock()

		w.logger.Debug("config file changed", "path", w.configPath, "modTime", modTime)

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

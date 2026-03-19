// Package daemon provides the main orchestration for histuid.
package daemon

import (
	"context"
	"log/slog"
	"sync"

	"github.com/fsnotify/fsnotify"

	"github.com/jmylchreest/histui/internal/config"
	"github.com/jmylchreest/histui/internal/fswatcher"
)

// ConfigWatcher watches the daemon config file for changes and validates new configs.
// Delegates to fswatcher.FileWatcher for the underlying file system monitoring.
type ConfigWatcher struct {
	mu     sync.RWMutex
	logger *slog.Logger
	fw     *fswatcher.FileWatcher

	// Path to watch
	configPath string

	// Current valid config
	currentConfig *config.DaemonConfig

	// Callbacks
	onReloadCallback func(newConfig *config.DaemonConfig)
	onErrorCallback  func(err error)
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
	}

	w.fw = fswatcher.New("config", logger, w.handleEvent,
		fswatcher.WithEnsureDirs(true),
	)
	w.fw.Watch(configPath)

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
	w.mu.Lock()
	w.currentConfig = initialConfig
	w.mu.Unlock()

	return w.fw.Start(ctx)
}

// Stop stops watching the config file.
func (w *ConfigWatcher) Stop() {
	w.fw.Stop()
}

// handleEvent processes a file change event from the underlying FileWatcher.
func (w *ConfigWatcher) handleEvent(_ string, _ fsnotify.Op) {
	w.logger.Debug("config file changed", "path", w.configPath)

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

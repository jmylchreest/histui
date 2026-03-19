package icon

import (
	"context"
	"log/slog"
	"sync"

	"github.com/fsnotify/fsnotify"

	"github.com/jmylchreest/histui/internal/fswatcher"
)

// AliasesWatcher watches the user's icon-aliases.toml file for changes.
// Delegates to fswatcher.FileWatcher for the underlying file system monitoring.
type AliasesWatcher struct {
	mu     sync.RWMutex
	logger *slog.Logger
	fw     *fswatcher.FileWatcher

	// Path to watch
	aliasesPath string

	// Callbacks
	onReloadCallback func(aliases map[string]string)
	onErrorCallback  func(err error)
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
	}

	w.fw = fswatcher.New("icon-aliases", logger, w.handleEvent,
		fswatcher.WithEnsureDirs(true),
	)
	w.fw.Watch(aliasesPath)

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
	return w.fw.Start(ctx)
}

// Stop stops watching the aliases file.
func (w *AliasesWatcher) Stop() {
	w.fw.Stop()
}

// handleEvent processes a file change event from the underlying FileWatcher.
func (w *AliasesWatcher) handleEvent(_ string, _ fsnotify.Op) {
	w.logger.Debug("icon aliases file changed", "path", w.aliasesPath)

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

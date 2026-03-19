package audio

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"

	"github.com/jmylchreest/histui/internal/fswatcher"
)

// Watcher watches audio files for changes and invalidates the cache.
// Delegates to fswatcher.FileWatcher for the underlying file system monitoring.
type Watcher struct {
	logger *slog.Logger
	player *Player
	fw     *fswatcher.FileWatcher
}

// NewWatcher creates a new audio file watcher.
func NewWatcher(player *Player, logger *slog.Logger) *Watcher {
	if logger == nil {
		logger = slog.Default()
	}

	w := &Watcher{
		logger: logger,
		player: player,
	}

	w.fw = fswatcher.New("audio", logger, w.handleEvent)

	return w
}

// Watch adds a path to the watch list.
// The file must exist on disk; non-existent files are silently skipped.
func (w *Watcher) Watch(path string) {
	if path == "" {
		return
	}

	// Resolve to absolute path for the existence check
	absPath, err := filepath.Abs(path)
	if err != nil {
		w.logger.Warn("failed to resolve path", "path", path, "error", err)
		return
	}

	// Check if file exists (audio files should already be on disk)
	if _, err := os.Stat(absPath); err != nil {
		w.logger.Warn("cannot watch non-existent file", "path", absPath)
		return
	}

	w.fw.Watch(path)
}

// Start begins watching audio files for changes.
func (w *Watcher) Start(ctx context.Context) error {
	return w.fw.Start(ctx)
}

// Stop stops watching audio files.
func (w *Watcher) Stop() {
	w.fw.Stop()
}

// handleEvent processes a file change event from the underlying FileWatcher.
func (w *Watcher) handleEvent(absPath string, _ fsnotify.Op) {
	w.logger.Debug("audio file changed, invalidating cache", "path", absPath)

	if w.player != nil {
		w.player.InvalidateCache(absPath)
	}
}

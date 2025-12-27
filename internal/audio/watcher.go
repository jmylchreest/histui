package audio

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// Watcher watches audio files for changes and invalidates the cache.
// Uses inotify (via fsnotify) for efficient file system event notification.
type Watcher struct {
	mu       sync.RWMutex
	logger   *slog.Logger
	player   *Player
	watcher  *fsnotify.Watcher
	watchErr error

	// Paths being watched (file path -> true)
	watchedPaths map[string]bool

	// Directories being watched (for files in the same directory)
	watchedDirs map[string]int // dir -> refcount

	// Control channels
	stopCh chan struct{}
	doneCh chan struct{}

	running bool
}

// NewWatcher creates a new audio file watcher.
func NewWatcher(player *Player, logger *slog.Logger) *Watcher {
	if logger == nil {
		logger = slog.Default()
	}

	w := &Watcher{
		logger:       logger,
		player:       player,
		watchedPaths: make(map[string]bool),
		watchedDirs:  make(map[string]int),
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}

	// Create the fsnotify watcher
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		w.watchErr = err
		w.logger.Warn("failed to create inotify watcher, file watching disabled", "error", err)
	} else {
		w.watcher = fsw
	}

	return w
}

// Watch adds a path to the watch list.
// We watch the directory containing the file since watching individual files
// can fail when the file is replaced (common with editors using atomic saves).
func (w *Watcher) Watch(path string) {
	if path == "" || w.watcher == nil {
		return
	}

	// Resolve to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		w.logger.Warn("failed to resolve path", "path", path, "error", err)
		return
	}

	// Check if file exists
	if _, err := os.Stat(absPath); err != nil {
		w.logger.Warn("cannot watch non-existent file", "path", absPath)
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Already watching this file
	if w.watchedPaths[absPath] {
		return
	}
	w.watchedPaths[absPath] = true

	// Watch the directory containing the file
	dir := filepath.Dir(absPath)
	w.watchedDirs[dir]++

	// Only add to watcher if this is the first file in this directory
	if w.watchedDirs[dir] == 1 {
		if err := w.watcher.Add(dir); err != nil {
			w.logger.Warn("failed to watch directory", "dir", dir, "error", err)
			w.watchedDirs[dir]--
			delete(w.watchedPaths, absPath)
		} else {
			w.logger.Debug("watching directory for audio file changes", "dir", dir)
		}
	}
}

// Unwatch removes a path from the watch list.
func (w *Watcher) Unwatch(path string) {
	if path == "" || w.watcher == nil {
		return
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.watchedPaths[absPath] {
		return
	}
	delete(w.watchedPaths, absPath)

	// Decrement directory refcount
	dir := filepath.Dir(absPath)
	w.watchedDirs[dir]--

	// If no more files in this directory, stop watching it
	if w.watchedDirs[dir] <= 0 {
		delete(w.watchedDirs, dir)
		if err := w.watcher.Remove(dir); err != nil {
			w.logger.Debug("failed to remove directory watch", "dir", dir, "error", err)
		}
	}
}

// Start begins watching audio files for changes.
func (w *Watcher) Start(ctx context.Context) error {
	if w.watcher == nil {
		// fsnotify failed to initialize, log and continue without watching
		w.logger.Warn("inotify watcher not available, audio file hot-reload disabled")
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
	w.mu.Unlock()

	go w.eventLoop(ctx)

	w.logger.Debug("audio inotify watcher started")
	return nil
}

// Stop stops watching audio files.
func (w *Watcher) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		// Close the fsnotify watcher even if not running
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

	w.logger.Debug("audio inotify watcher stopped")
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
			w.logger.Warn("inotify watcher error", "error", err)
		}
	}
}

// handleEvent processes a single file system event.
func (w *Watcher) handleEvent(event fsnotify.Event) {
	// Get absolute path of the affected file
	absPath, err := filepath.Abs(event.Name)
	if err != nil {
		return
	}

	// Check if this is a file we're watching
	w.mu.RLock()
	isWatched := w.watchedPaths[absPath]
	w.mu.RUnlock()

	if !isWatched {
		return
	}

	// Only care about write and create events (handles both edit-in-place and atomic save)
	if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
		w.logger.Debug("audio file changed, invalidating cache", "path", absPath, "op", event.Op.String())

		if w.player != nil {
			w.player.InvalidateCache(absPath)
		}
	}
}

// IsRunning returns whether the watcher is currently running.
func (w *Watcher) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

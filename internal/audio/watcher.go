package audio

import (
	"context"
	"log/slog"
	"maps"
	"os"
	"sync"
	"time"
)

// Watcher watches audio files for changes and invalidates the cache.
type Watcher struct {
	mu     sync.RWMutex
	logger *slog.Logger
	player *Player

	// Paths to watch with their last modification times
	watchedPaths map[string]time.Time

	// Polling interval
	pollInterval time.Duration

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

	return &Watcher{
		logger:       logger,
		player:       player,
		watchedPaths: make(map[string]time.Time),
		pollInterval: 2 * time.Second,
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

// Watch adds a path to the watch list.
func (w *Watcher) Watch(path string) {
	if path == "" {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Get initial modification time
	if info, err := os.Stat(path); err == nil {
		w.watchedPaths[path] = info.ModTime()
	} else {
		w.watchedPaths[path] = time.Time{}
	}
}

// Unwatch removes a path from the watch list.
func (w *Watcher) Unwatch(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.watchedPaths, path)
}

// Start begins watching audio files for changes.
func (w *Watcher) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return nil
	}
	w.running = true
	w.stopCh = make(chan struct{})
	w.doneCh = make(chan struct{})
	w.mu.Unlock()

	go w.watchLoop(ctx)

	w.logger.Debug("audio watcher started", "interval", w.pollInterval)
	return nil
}

// Stop stops watching audio files.
func (w *Watcher) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.running = false
	close(w.stopCh)
	w.mu.Unlock()

	<-w.doneCh
	w.logger.Debug("audio watcher stopped")
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

// checkForChanges checks if any watched files have been modified.
func (w *Watcher) checkForChanges() {
	w.mu.RLock()
	paths := make(map[string]time.Time, len(w.watchedPaths))
	maps.Copy(paths, w.watchedPaths)
	w.mu.RUnlock()

	for path, lastModTime := range paths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		modTime := info.ModTime()
		if modTime.After(lastModTime) {
			w.logger.Debug("audio file changed, invalidating cache", "path", path)

			w.mu.Lock()
			w.watchedPaths[path] = modTime
			w.mu.Unlock()

			if w.player != nil {
				w.player.InvalidateCache(path)
			}
		}
	}
}

// IsRunning returns whether the watcher is currently running.
func (w *Watcher) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

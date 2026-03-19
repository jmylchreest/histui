// Package fswatcher provides a generic file watcher built on fsnotify.
//
// It encapsulates the common lifecycle pattern (Start/Stop/event loop) used
// across multiple packages that need to watch files for changes. Consumers
// provide a callback that is invoked when a watched file is written or created.
//
// Directories are watched (not individual files) to handle editors that use
// atomic saves (write to temp file, rename over target).
package fswatcher

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// EventHandler is called when a watched file changes.
// absPath is the absolute path of the changed file.
// op is the fsnotify operation that triggered the event.
type EventHandler func(absPath string, op fsnotify.Op)

// Option configures a FileWatcher.
type Option func(*FileWatcher)

// WithEnsureDirs causes Start() to create watched directories if they don't exist.
// When Watch() is called after Start(), the directory is also created at that point.
func WithEnsureDirs(ensure bool) Option {
	return func(w *FileWatcher) {
		w.ensureDirs = ensure
	}
}

// WithEventFilter sets which fsnotify operations trigger the handler.
// Default is Write|Create.
func WithEventFilter(op fsnotify.Op) Option {
	return func(w *FileWatcher) {
		w.eventFilter = op
	}
}

// WithErrorHandler sets a custom handler for fsnotify watcher errors.
// Default is to log a warning.
func WithErrorHandler(handler func(err error)) Option {
	return func(w *FileWatcher) {
		w.errorHandler = handler
	}
}

// FileWatcher watches files via fsnotify and dispatches change events.
type FileWatcher struct {
	mu       sync.RWMutex
	logger   *slog.Logger
	watcher  *fsnotify.Watcher
	watchErr error

	// Configuration
	name         string
	handler      EventHandler
	errorHandler func(err error)
	eventFilter  fsnotify.Op

	// Path tracking
	watchedPaths map[string]bool
	watchedDirs  map[string]int
	ensureDirs   bool

	// Control channels
	stopCh  chan struct{}
	doneCh  chan struct{}
	running bool
}

// New creates a new FileWatcher. The name is used in log messages (e.g., "audio",
// "config", "icon-aliases"). The handler is called when a watched file changes.
//
// If fsnotify fails to initialize (e.g., no inotify support), the watcher
// degrades gracefully — Start() becomes a no-op and Watch() is silently ignored.
func New(name string, logger *slog.Logger, handler EventHandler, opts ...Option) *FileWatcher {
	if logger == nil {
		logger = slog.Default()
	}

	w := &FileWatcher{
		name:         name,
		logger:       logger,
		handler:      handler,
		eventFilter:  fsnotify.Write | fsnotify.Create,
		watchedPaths: make(map[string]bool),
		watchedDirs:  make(map[string]int),
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}

	for _, opt := range opts {
		opt(w)
	}

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		w.watchErr = err
		w.logger.Warn("failed to create inotify watcher, file watching disabled",
			"name", name, "error", err)
	} else {
		w.watcher = fsw
	}

	return w
}

// Watch adds a file path to the watch list. The containing directory is
// watched via fsnotify (to handle atomic saves). Directory refcounting
// is managed automatically.
//
// Safe to call before or after Start().
func (w *FileWatcher) Watch(path string) {
	if path == "" || w.watcher == nil {
		return
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		w.logger.Warn("failed to resolve path", "name", w.name, "path", path, "error", err)
		return
	}

	dir := filepath.Dir(absPath)

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.watchedPaths[absPath] {
		return
	}
	w.watchedPaths[absPath] = true

	w.watchedDirs[dir]++
	if w.watchedDirs[dir] == 1 {
		if w.ensureDirs {
			if err := os.MkdirAll(dir, 0755); err != nil {
				w.logger.Warn("failed to create watch directory",
					"name", w.name, "dir", dir, "error", err)
			}
		}

		if err := w.watcher.Add(dir); err != nil {
			w.logger.Warn("failed to watch directory",
				"name", w.name, "dir", dir, "error", err)
			w.watchedDirs[dir]--
			delete(w.watchedPaths, absPath)
		} else {
			w.logger.Debug("watching directory",
				"name", w.name, "dir", dir, "file", filepath.Base(absPath))
		}
	}
}

// Unwatch removes a file path from the watch list. The directory watch is
// removed when its refcount reaches zero.
func (w *FileWatcher) Unwatch(path string) {
	if path == "" || w.watcher == nil {
		return
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return
	}

	dir := filepath.Dir(absPath)

	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.watchedPaths[absPath] {
		return
	}
	delete(w.watchedPaths, absPath)

	w.watchedDirs[dir]--
	if w.watchedDirs[dir] <= 0 {
		delete(w.watchedDirs, dir)
		_ = w.watcher.Remove(dir)
	}
}

// Start begins the event loop. Idempotent (safe to call multiple times).
func (w *FileWatcher) Start(ctx context.Context) error {
	if w.watcher == nil {
		w.logger.Warn("inotify watcher not available, file watching disabled", "name", w.name)
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

	w.logger.Debug("file watcher started", "name", w.name)
	return nil
}

// Stop stops the event loop and closes the underlying fsnotify watcher.
// Blocks until the event loop goroutine exits.
func (w *FileWatcher) Stop() {
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

	w.logger.Debug("file watcher stopped", "name", w.name)
}

// eventLoop processes fsnotify events until stopped or context cancelled.
func (w *FileWatcher) eventLoop(ctx context.Context) {
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
			if w.errorHandler != nil {
				w.errorHandler(err)
			} else {
				w.logger.Warn("inotify watcher error", "name", w.name, "error", err)
			}
		}
	}
}

// handleEvent processes a single file system event.
func (w *FileWatcher) handleEvent(event fsnotify.Event) {
	absPath, err := filepath.Abs(event.Name)
	if err != nil {
		return
	}

	w.mu.RLock()
	isWatched := w.watchedPaths[absPath]
	w.mu.RUnlock()

	if !isWatched {
		return
	}

	if event.Op&w.eventFilter != 0 {
		if w.handler != nil {
			w.handler(absPath, event.Op)
		}
	}
}

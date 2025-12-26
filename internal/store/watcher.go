package store

import (
	"log/slog"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// FileWatcher watches a file for changes and triggers rehydration.
type FileWatcher struct {
	watcher  *fsnotify.Watcher
	store    *Store
	filePath string
	done     chan struct{}
	mu       sync.Mutex
	running  bool
}

// NewFileWatcher creates a new file watcher for the store's persistence file.
func NewFileWatcher(store *Store, filePath string) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	fw := &FileWatcher{
		watcher:  watcher,
		store:    store,
		filePath: filePath,
		done:     make(chan struct{}),
	}

	return fw, nil
}

// Start begins watching the file for changes.
func (fw *FileWatcher) Start() error {
	fw.mu.Lock()
	if fw.running {
		fw.mu.Unlock()
		return nil
	}
	fw.running = true
	fw.mu.Unlock()

	// Watch the directory containing the file (more reliable for writes)
	dir := filepath.Dir(fw.filePath)
	if err := fw.watcher.Add(dir); err != nil {
		return err
	}

	go fw.watch()
	return nil
}

// watch is the main watch loop.
func (fw *FileWatcher) watch() {
	filename := filepath.Base(fw.filePath)

	for {
		select {
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}

			// Only care about our file
			if filepath.Base(event.Name) != filename {
				continue
			}

			// Handle write events
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				slog.Debug("file changed, rehydrating store", "file", fw.filePath)
				if err := fw.store.Hydrate(); err != nil {
					slog.Warn("failed to rehydrate store", "error", err)
				}
			}

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			slog.Warn("file watcher error", "error", err)

		case <-fw.done:
			return
		}
	}
}

// Stop stops the file watcher.
func (fw *FileWatcher) Stop() error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if !fw.running {
		return nil
	}

	fw.running = false
	close(fw.done)
	return fw.watcher.Close()
}

// Package tui provides the BubbleTea-based terminal user interface.
package tui

import (
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"

	"github.com/jmylchreest/histui/internal/db"
)

// fileWatcherMsg signals that the .lastupdated file has changed.
type fileWatcherMsg struct{}

// startFileWatcher starts watching the .lastupdated file.
// Returns a tea.Cmd that should be run in Init().
// When a change is detected, it returns fileWatcherMsg.
func startFileWatcher() tea.Cmd {
	return func() tea.Msg {
		path, err := db.LastUpdatedPath()
		if err != nil {
			return nil
		}

		// Create watcher
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			return nil
		}
		// Note: watcher is intentionally not closed here - it runs until a change is detected
		// After returning fileWatcherMsg, the Update handler restarts the watcher

		// Watch the parent directory (file may not exist yet)
		dir := filepath.Dir(path)
		if err := watcher.Add(dir); err != nil {
			_ = watcher.Close()
			return nil
		}

		// Debounce rapid file changes
		var lastEvent time.Time
		const debounceInterval = 100 * time.Millisecond

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					_ = watcher.Close()
					return nil
				}
				// Check if this event is for our file
				if event.Name == path && (event.Op&fsnotify.Write != 0 || event.Op&fsnotify.Create != 0) {
					now := time.Now()
					if now.Sub(lastEvent) >= debounceInterval {
						_ = watcher.Close()
						return fileWatcherMsg{}
					}
				}
			case _, ok := <-watcher.Errors:
				if !ok {
					_ = watcher.Close()
					return nil
				}
				// Ignore errors, continue watching
			}
		}
	}
}

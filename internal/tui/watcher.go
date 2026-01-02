// Package tui provides the BubbleTea-based terminal user interface.
package tui

import (
	"path/filepath"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"

	"github.com/jmylchreest/histui/internal/db"
	"github.com/jmylchreest/histui/internal/dbus"
)

// fileWatcherMsg signals that the .lastupdated file has changed.
type fileWatcherMsg struct{}

// dbusSignalMsg contains D-Bus signal information.
type dbusSignalMsg struct {
	signalType string // "displayed", "dismissed", "dnd_changed", "config_changed"
	histuiID   string // for displayed/dismissed
	dndEnabled bool   // for dnd_changed
}

// dbusRetryMsg signals that the D-Bus watcher should be retried.
type dbusRetryMsg struct{}

// activeIDsMsg contains the current set of active notification IDs.
type activeIDsMsg struct {
	ids []string
}

// daemonStatusMsg contains the daemon status label.
type daemonStatusMsg struct {
	status string
}

// Shared D-Bus signal channel - stays open for the lifetime of the TUI
var (
	dbusSignalCh     chan tea.Msg
	dbusSignalOnce   sync.Once
	dbusWatcherSetup sync.Once
)

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

// signalHandler implements dbus.SignalHandler for TUI updates.
type signalHandler struct {
	msgCh chan tea.Msg
}

func (h *signalHandler) OnDnDChanged(enabled bool) {
	select {
	case h.msgCh <- dbusSignalMsg{signalType: "dnd_changed", dndEnabled: enabled}:
	default:
		// Channel full, drop the message
	}
}

func (h *signalHandler) OnNotificationDisplayed(histuiID string) {
	select {
	case h.msgCh <- dbusSignalMsg{signalType: "displayed", histuiID: histuiID}:
	default:
	}
}

func (h *signalHandler) OnNotificationDismissed(histuiID string, _ string) {
	select {
	case h.msgCh <- dbusSignalMsg{signalType: "dismissed", histuiID: histuiID}:
	default:
	}
}

func (h *signalHandler) OnConfigChanged() {
	select {
	case h.msgCh <- dbusSignalMsg{signalType: "config_changed"}:
	default:
	}
}

// initDBusSignalChannel initializes the shared D-Bus signal channel and starts
// the background watcher goroutine. This is called once and the channel stays
// open for the lifetime of the TUI.
func initDBusSignalChannel() {
	dbusSignalOnce.Do(func() {
		dbusSignalCh = make(chan tea.Msg, 64) // Larger buffer for rapid signals
	})

	dbusWatcherSetup.Do(func() {
		go func() {
			for {
				client := dbus.NewDaemonClient(nil)
				if !client.IsAvailable() {
					_ = client.Close()
					time.Sleep(5 * time.Second)
					continue
				}

				handler := &signalHandler{msgCh: dbusSignalCh}

				if err := client.SubscribeSignals(handler); err != nil {
					_ = client.Close()
					time.Sleep(2 * time.Second)
					continue
				}

				// Periodically check if connection is still alive
				// If ping fails, reconnect
				for {
					time.Sleep(10 * time.Second)
					if _, err := client.Ping(); err != nil {
						_ = client.Close()
						break
					}
				}
			}
		}()
	})
}

// startDBusWatcher starts watching D-Bus signals from histuid.
// Returns a tea.Cmd that reads from the shared signal channel.
func startDBusWatcher() tea.Cmd {
	// Ensure the channel and background watcher are initialized
	initDBusSignalChannel()

	return func() tea.Msg {
		// Read from the shared channel - this blocks until a signal arrives
		// but the channel stays open so we don't miss signals
		select {
		case msg := <-dbusSignalCh:
			return msg
		case <-time.After(30 * time.Second):
			// Periodic timeout to allow retry/reconnect checks
			return dbusRetryMsg{}
		}
	}
}

// fetchActiveIDs fetches the current active notification IDs from histuid.
func fetchActiveIDs() tea.Cmd {
	return func() tea.Msg {
		client := dbus.NewDaemonClient(nil)
		defer func() { _ = client.Close() }()

		ids, err := client.GetActiveNotifications()
		if err != nil || ids == nil {
			return activeIDsMsg{ids: []string{}}
		}
		return activeIDsMsg{ids: ids}
	}
}

// fetchDaemonStatus fetches the current daemon status.
func fetchDaemonStatus() tea.Cmd {
	return func() tea.Msg {
		status := dbus.GetDaemonStatusLabel()
		return daemonStatusMsg{status: status}
	}
}

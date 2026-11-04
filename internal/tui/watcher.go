// Package tui provides the BubbleTea-based terminal user interface.
package tui

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"

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

// fileWatcher owns one fsnotify watch for the entire TUI lifetime. Its buffered
// invalidation channel coalesces events while Update is busy, without ever
// closing/reopening the filesystem watch between refreshes.
type fileWatcher struct {
	changes chan struct{}
	cancel  context.CancelFunc
	done    chan struct{}
}

func newFileWatcher(path string) (*fileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := watcher.Add(filepath.Dir(path)); err != nil {
		_ = watcher.Close()
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	w := &fileWatcher{changes: make(chan struct{}, 1), cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(w.done)
		defer close(w.changes)
		defer func() { _ = watcher.Close() }()
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Name == path && event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) != 0 {
					w.invalidate()
				}
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
				// An overflow may mean a notification was missed. Reconcile from the DB.
				w.invalidate()
			}
		}
	}()
	return w, nil
}

func (w *fileWatcher) invalidate() {
	select {
	case w.changes <- struct{}{}:
	default:
	}
}

func (w *fileWatcher) wait() tea.Msg {
	if _, ok := <-w.changes; !ok {
		return nil
	}
	return fileWatcherMsg{}
}

func (w *fileWatcher) close() {
	w.cancel()
	<-w.done
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

// Package display implements GTK4/libadwaita notification popup display.
package display

import (
	"log/slog"
	"sync"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	layershell "github.com/diamondburned/gotk4-layer-shell/pkg/gtk4layershell"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/jmylchreest/histui/internal/config"
)

// NotificationStack manages a single layer-shell window containing all notification widgets.
// This approach eliminates gaps between notifications that occur with multiple windows,
// since the compositor can only round the outer window corners.
type NotificationStack struct {
	window    *gtk.Window
	container *gtk.Box // Vertical box holding all notification widgets
	app       *gtk.Application
	config    *config.DaemonConfig
	logger    *slog.Logger

	// Ordered list of notification widgets (oldest first = top)
	mu      sync.RWMutex
	widgets []*NotificationWidget

	// Track if window is currently visible
	visible bool
}

// NotificationWidget wraps a notification's GTK widget with its metadata.
type NotificationWidget struct {
	DBusID    uint32
	HistuiID  string
	Container *gtk.Box // The .notification-popup box
	Popup     *Popup   // Reference to the Popup for callbacks and state
}

// NewNotificationStack creates a new notification stack window.
func NewNotificationStack(app *gtk.Application, cfg *config.DaemonConfig, logger *slog.Logger) *NotificationStack {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg == nil {
		cfg = config.DefaultDaemonConfig()
	}

	s := &NotificationStack{
		app:     app,
		config:  cfg,
		logger:  logger,
		widgets: make([]*NotificationWidget, 0),
	}

	s.createWindow()

	return s
}

// createWindow initializes the layer-shell window and container.
func (s *NotificationStack) createWindow() {
	s.window = gtk.NewWindow()
	s.window.SetApplication(s.app)
	s.window.SetDecorated(false)
	s.window.SetResizable(false)

	// Initialize layer-shell
	layershell.InitForWindow(s.window)
	layershell.SetLayer(s.window, layershell.LayerShellLayerTop)
	layershell.SetExclusiveZone(s.window, 0) // Don't reserve space
	layershell.SetKeyboardMode(s.window, layershell.LayerShellKeyboardModeNone)

	// Set namespace for compositor rules
	layershell.SetNamespace(s.window, "histui-notification")

	// Create the vertical container for notifications
	// Gap is 0 for unified stack appearance - separators are CSS borders
	s.container = gtk.NewBox(gtk.OrientationVertical, 0)
	s.container.AddCSSClass("notification-stack")
	s.window.SetChild(s.container)

	// Set initial position
	s.updatePosition()

	s.logger.Debug("created notification stack window")
}

// updatePosition sets the layer-shell anchors and margins based on config.
func (s *NotificationStack) updatePosition() {
	pos := config.Position(s.config.Display.Position)
	offsetX := s.config.Display.OffsetX
	offsetY := s.config.Display.OffsetY

	// Reset all anchors first
	layershell.SetAnchor(s.window, layershell.LayerShellEdgeTop, false)
	layershell.SetAnchor(s.window, layershell.LayerShellEdgeBottom, false)
	layershell.SetAnchor(s.window, layershell.LayerShellEdgeLeft, false)
	layershell.SetAnchor(s.window, layershell.LayerShellEdgeRight, false)

	switch pos {
	case config.PositionTopRight:
		layershell.SetAnchor(s.window, layershell.LayerShellEdgeTop, true)
		layershell.SetAnchor(s.window, layershell.LayerShellEdgeRight, true)
		layershell.SetMargin(s.window, layershell.LayerShellEdgeTop, offsetY)
		layershell.SetMargin(s.window, layershell.LayerShellEdgeRight, offsetX)

	case config.PositionTopLeft:
		layershell.SetAnchor(s.window, layershell.LayerShellEdgeTop, true)
		layershell.SetAnchor(s.window, layershell.LayerShellEdgeLeft, true)
		layershell.SetMargin(s.window, layershell.LayerShellEdgeTop, offsetY)
		layershell.SetMargin(s.window, layershell.LayerShellEdgeLeft, offsetX)

	case config.PositionTopCenter:
		layershell.SetAnchor(s.window, layershell.LayerShellEdgeTop, true)
		layershell.SetMargin(s.window, layershell.LayerShellEdgeTop, offsetY)

	case config.PositionBottomRight:
		layershell.SetAnchor(s.window, layershell.LayerShellEdgeBottom, true)
		layershell.SetAnchor(s.window, layershell.LayerShellEdgeRight, true)
		layershell.SetMargin(s.window, layershell.LayerShellEdgeBottom, offsetY)
		layershell.SetMargin(s.window, layershell.LayerShellEdgeRight, offsetX)

	case config.PositionBottomLeft:
		layershell.SetAnchor(s.window, layershell.LayerShellEdgeBottom, true)
		layershell.SetAnchor(s.window, layershell.LayerShellEdgeLeft, true)
		layershell.SetMargin(s.window, layershell.LayerShellEdgeBottom, offsetY)
		layershell.SetMargin(s.window, layershell.LayerShellEdgeLeft, offsetX)

	case config.PositionBottomCenter:
		layershell.SetAnchor(s.window, layershell.LayerShellEdgeBottom, true)
		layershell.SetMargin(s.window, layershell.LayerShellEdgeBottom, offsetY)
	}
}

// Add adds a notification widget to the stack.
// Position depends on config.Display.NewOnTop:
//   - false (default): new notifications at bottom of stack
//   - true: new notifications at top of stack
func (s *NotificationStack) Add(widget *NotificationWidget) {
	s.mu.Lock()

	newOnTop := s.config.Display.NewOnTop
	if newOnTop {
		// Prepend to widgets list
		s.widgets = append([]*NotificationWidget{widget}, s.widgets...)
	} else {
		// Append to widgets list (default)
		s.widgets = append(s.widgets, widget)
	}

	// Capture values for idle callback
	total := len(s.widgets)
	needsPresent := !s.visible
	s.visible = true

	// Capture stack position updates
	positionUpdates := s.captureStackPositionsLocked()

	s.mu.Unlock()

	// Perform GTK operations on main thread to avoid layout race conditions
	container := widget.Container
	glib.IdleAdd(func() {
		// Add widget to container
		if newOnTop {
			s.container.Prepend(container)
		} else {
			s.container.Append(container)
		}

		// Apply stack position CSS classes
		for _, update := range positionUpdates {
			updateStackPositionClass(update.box, update.position)
		}

		// Show window if not already visible
		if needsPresent {
			s.window.Present()
		}
	})

	s.logger.Debug("added notification to stack",
		"dbus_id", widget.DBusID,
		"total", total,
		"new_on_top", newOnTop,
	)
}

// Remove removes a notification widget from the stack by D-Bus ID.
func (s *NotificationStack) Remove(dbusID uint32) *NotificationWidget {
	s.mu.Lock()
	defer s.mu.Unlock()

	var removed *NotificationWidget
	newWidgets := make([]*NotificationWidget, 0, len(s.widgets))

	for _, w := range s.widgets {
		if w.DBusID == dbusID {
			removed = w
		} else {
			newWidgets = append(newWidgets, w)
		}
	}

	s.widgets = newWidgets

	// Update stack position classes first (before removal)
	s.updateStackPositionsLocked()

	// Remove widget from container in idle callback to avoid GTK layout race
	if removed != nil {
		container := removed.Container
		glib.IdleAdd(func() {
			s.container.Remove(container)
		})

		s.logger.Debug("removed notification from stack",
			"dbus_id", dbusID,
			"remaining", len(s.widgets),
		)
	}

	// Hide window if empty (defer to idle to avoid race)
	if len(s.widgets) == 0 && s.visible {
		glib.IdleAdd(func() {
			s.window.SetVisible(false)
		})
		s.visible = false
	}

	return removed
}

// RemoveByHistuiID removes a notification widget from the stack by histui ID.
func (s *NotificationStack) RemoveByHistuiID(histuiID string) *NotificationWidget {
	s.mu.Lock()
	defer s.mu.Unlock()

	var removed *NotificationWidget
	newWidgets := make([]*NotificationWidget, 0, len(s.widgets))

	for _, w := range s.widgets {
		if w.HistuiID == histuiID {
			removed = w
		} else {
			newWidgets = append(newWidgets, w)
		}
	}

	s.widgets = newWidgets

	// Update stack position classes first (before removal)
	s.updateStackPositionsLocked()

	// Remove widget from container in idle callback to avoid GTK layout race
	if removed != nil {
		container := removed.Container
		glib.IdleAdd(func() {
			s.container.Remove(container)
		})
	}

	// Hide window if empty (defer to idle to avoid race)
	if len(s.widgets) == 0 && s.visible {
		glib.IdleAdd(func() {
			s.window.SetVisible(false)
		})
		s.visible = false
	}

	return removed
}

// Get returns a notification widget by D-Bus ID.
func (s *NotificationStack) Get(dbusID uint32) *NotificationWidget {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, w := range s.widgets {
		if w.DBusID == dbusID {
			return w
		}
	}
	return nil
}

// GetByHistuiID returns a notification widget by histui ID.
func (s *NotificationStack) GetByHistuiID(histuiID string) *NotificationWidget {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, w := range s.widgets {
		if w.HistuiID == histuiID {
			return w
		}
	}
	return nil
}

// Clear removes all notifications from the stack.
func (s *NotificationStack) Clear() []*NotificationWidget {
	s.mu.Lock()
	defer s.mu.Unlock()

	removed := s.widgets
	s.widgets = make([]*NotificationWidget, 0)

	// Remove all from container in idle callback to avoid GTK layout race
	// Capture containers before they could be modified
	containers := make([]*gtk.Box, len(removed))
	for i, w := range removed {
		containers[i] = w.Container
	}
	glib.IdleAdd(func() {
		for _, container := range containers {
			s.container.Remove(container)
		}
	})

	// Hide window (defer to idle to avoid race)
	if s.visible {
		glib.IdleAdd(func() {
			s.window.SetVisible(false)
		})
		s.visible = false
	}

	s.logger.Debug("cleared notification stack", "removed", len(removed))

	return removed
}

// Count returns the number of notifications in the stack.
func (s *NotificationStack) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.widgets)
}

// All returns all notification widgets in order.
func (s *NotificationStack) All() []*NotificationWidget {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*NotificationWidget, len(s.widgets))
	copy(result, s.widgets)
	return result
}

// stackPositionUpdate captures a position update for deferred GTK execution.
type stackPositionUpdate struct {
	box      *gtk.Box
	position string
}

// captureStackPositionsLocked captures stack position updates for deferred application.
// Caller must hold the lock.
func (s *NotificationStack) captureStackPositionsLocked() []stackPositionUpdate {
	count := len(s.widgets)
	updates := make([]stackPositionUpdate, 0, count)

	for i, w := range s.widgets {
		var position string
		switch {
		case count == 1:
			position = "single"
		case i == 0:
			position = "first"
		case i == count-1:
			position = "last"
		default:
			position = "middle"
		}

		updates = append(updates, stackPositionUpdate{
			box:      w.Container,
			position: position,
		})
	}

	return updates
}

// updateStackPositionsLocked updates CSS classes for stack position styling.
// Caller must hold the lock. Uses IdleAdd for thread-safe GTK operations.
func (s *NotificationStack) updateStackPositionsLocked() {
	updates := s.captureStackPositionsLocked()

	// Apply updates on GTK main thread
	glib.IdleAdd(func() {
		for _, update := range updates {
			updateStackPositionClass(update.box, update.position)
		}
	})
}

// updateStackPositionClass updates the stack position CSS class on a widget.
func updateStackPositionClass(box *gtk.Box, position string) {
	// Remove old stack position classes
	box.RemoveCSSClass("stack-single")
	box.RemoveCSSClass("stack-first")
	box.RemoveCSSClass("stack-middle")
	box.RemoveCSSClass("stack-last")

	// Add new class
	if position != "" {
		box.AddCSSClass("stack-" + position)
	}
}

// UpdateConfig updates the stack configuration.
func (s *NotificationStack) UpdateConfig(cfg *config.DaemonConfig) {
	s.mu.Lock()
	s.config = cfg
	s.mu.Unlock()

	// Update window position
	glib.IdleAdd(func() {
		s.updatePosition()
	})
}

// Destroy cleans up the stack window.
func (s *NotificationStack) Destroy() {
	s.Clear()
	if s.window != nil {
		s.window.Destroy()
		s.window = nil
	}
}

// Ensure adw is used (for libadwaita initialization)
var _ = adw.MAJOR_VERSION

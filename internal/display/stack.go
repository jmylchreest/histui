// Package display implements GTK4/libadwaita notification popup display.
package display

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"unsafe"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	layershell "github.com/diamondburned/gotk4-layer-shell/pkg/gtk4layershell"
	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
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

	// Monitor hotplug watch: re-resolves the configured output when the set of
	// connected monitors changes. Both are set together and only touched on the
	// GTK main thread.
	monitorsModel         *gio.ListModel
	monitorsChangedHandle coreglib.SignalHandle
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
	layershell.SetLayer(s.window, s.configuredLayer())
	layershell.SetExclusiveZone(s.window, 0) // Don't reserve space
	layershell.SetKeyboardMode(s.window, layershell.LayerShellKeyboardModeNone)

	// Set namespace for compositor rules
	layershell.SetNamespace(s.window, "histui-notification")

	// Pin the surface to the configured output (0 = let compositor decide)
	s.applyMonitor()
	s.watchMonitorOutputs()

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

// applyMonitor pins the layer-shell surface to the configured output.
//
// config.Display.Monitor selects an output by 1-based index or connector name
// ("DP-1"); its zero value means "let the compositor decide" (the layer-shell
// default). This runs on startup and on every config reload, so switching
// monitor at runtime — including back to auto — takes effect live.
func (s *NotificationStack) applyMonitor() {
	if s.window == nil {
		return
	}

	s.logDetectedOutputs()

	sel := s.config.Display.Monitor
	monitor := s.resolveMonitor(sel)
	if monitor == nil {
		// Auto (0) or no resolvable output: hand layer-shell a NULL monitor so
		// the compositor chooses. This also actively un-pins the surface on a
		// live reload from a specific monitor back to 0.
		layershell.SetMonitor(s.window, nullMonitor())
		s.logger.Debug("using output for notifications", "monitor", "auto", "source", "config")
		return
	}

	layershell.SetMonitor(s.window, monitor)
	s.logger.Debug("using output for notifications",
		"monitor", sel.String(),
		"connector", monitor.Connector(),
		"description", monitor.Description(),
		"source", "config")
}

// watchMonitorOutputs re-applies the output selection whenever the set of
// connected monitors changes (hotplug/unplug). This keeps an index- or
// name-based pin correct across monitors coming and going — e.g. when the
// named output reconnects — without a config reload or restart. The
// items-changed signal fires on the GTK main loop, so the handler can touch
// GTK/layer-shell state directly.
func (s *NotificationStack) watchMonitorOutputs() {
	display := gdk.DisplayGetDefault()
	if display == nil {
		return
	}
	monitors := display.Monitors()
	if monitors == nil {
		return
	}

	s.monitorsModel = monitors
	s.monitorsChangedHandle = monitors.ConnectItemsChanged(func(position, removed, added uint) {
		s.logger.Debug("monitor layout changed, re-applying output selection",
			"removed", removed, "added", added)
		s.applyMonitor()
	})
}

// nullMonitor returns a *gdk.Monitor whose native pointer is NULL.
//
// layershell.SetMonitor panics on a typed-nil *gdk.Monitor: the gotk4 binding
// dereferences the value to read its native pointer. A wrapper that is itself
// non-nil but embeds a nil object pointer instead resolves to a native pointer
// of 0 — a genuine NULL GdkMonitor*, which gtk_layer_set_monitor treats as
// "let the compositor choose". The layout mirrors gotk4's own object wrappers
// (an embedded *coreglib.Object); unsafe but stable for the pinned gotk4
// version, and the only way to un-pin an output at runtime.
func nullMonitor() *gdk.Monitor {
	var m struct {
		_ [0]func()
		*coreglib.Object
	}
	return (*gdk.Monitor)(unsafe.Pointer(&m))
}

// logDetectedOutputs logs every connected output at debug so users can map the
// 1-indexed `monitor` config value to a physical monitor — emitted on every
// applyMonitor, including monitor=0, so the indices are visible without having
// to pin one first.
func (s *NotificationStack) logDetectedOutputs() {
	display := gdk.DisplayGetDefault()
	if display == nil {
		return
	}
	monitors := display.Monitors()
	if monitors == nil {
		return
	}

	n := monitors.NItems()
	s.logger.Debug("detected outputs", "count", n)
	for i := uint(0); i < n; i++ {
		m := castMonitor(monitors.Item(i))
		if m == nil {
			continue
		}
		g := m.Geometry()
		s.logger.Debug("detected output",
			"index", i+1,
			"connector", m.Connector(),
			"description", m.Description(),
			"geometry", fmt.Sprintf("%dx%d+%d+%d", g.Width(), g.Height(), g.X(), g.Y()))
	}
}

// resolveMonitor maps a MonitorSelector to a *gdk.Monitor. Returns nil for auto
// or when no display/monitors are available, letting the compositor choose.
//
// By connector name: matches m.Connector() (or m.Description()) case-insensitively;
// an unknown name falls back to auto, since there is no sensible index to guess.
// By index (1-based): an out-of-range index falls back to the first output.
func (s *NotificationStack) resolveMonitor(sel config.MonitorSelector) *gdk.Monitor {
	if sel.IsAuto() {
		return nil
	}

	display := gdk.DisplayGetDefault()
	if display == nil {
		s.logger.Warn("no display available, deferring output selection to compositor")
		return nil
	}

	monitors := display.Monitors()
	if monitors == nil {
		s.logger.Warn("no monitors list available, deferring output selection to compositor")
		return nil
	}

	n := monitors.NItems()

	if sel.Name != "" {
		for i := uint(0); i < n; i++ {
			m := castMonitor(monitors.Item(i))
			if m == nil {
				continue
			}
			if strings.EqualFold(m.Connector(), sel.Name) || strings.EqualFold(m.Description(), sel.Name) {
				return m
			}
		}
		s.logger.Warn("configured monitor name not found, deferring output selection to compositor",
			"name", sel.Name, "available", n)
		return nil
	}

	index := uint(sel.Index - 1)
	if index >= n {
		s.logger.Warn("configured monitor not available, falling back to first output",
			"configured", sel.Index, "available", n)
		index = 0
	}

	return castMonitor(monitors.Item(index))
}

// castMonitor converts a list-model item to a *gdk.Monitor via gotk4's
// concrete-type marshaling.
func castMonitor(obj *coreglib.Object) *gdk.Monitor {
	if obj == nil {
		return nil
	}
	m, _ := obj.Cast().(*gdk.Monitor)
	return m
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

	// Re-apply output selection and position (monitor layout may have changed)
	glib.IdleAdd(func() {
		s.applyMonitor()
		s.updatePosition()
	})
}

// Destroy cleans up the stack window.
func (s *NotificationStack) Destroy() {
	if s.monitorsModel != nil && s.monitorsChangedHandle != 0 {
		s.monitorsModel.HandlerDisconnect(s.monitorsChangedHandle)
		s.monitorsChangedHandle = 0
		s.monitorsModel = nil
	}
	s.Clear()
	if s.window != nil {
		s.window.Destroy()
		s.window = nil
	}
}

// configuredLayer returns the layer-shell layer from config.
func (s *NotificationStack) configuredLayer() layershell.Layer {
	switch config.Layer(s.config.Display.Layer) {
	case config.LayerBackground:
		return layershell.LayerShellLayerBackground
	case config.LayerBottom:
		return layershell.LayerShellLayerBottom
	case config.LayerOverlay:
		return layershell.LayerShellLayerOverlay
	default:
		return layershell.LayerShellLayerTop
	}
}

// Ensure adw is used (for libadwaita initialization)
var _ = adw.MAJOR_VERSION

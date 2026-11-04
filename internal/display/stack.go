// Package display implements GTK4/libadwaita notification popup display.
package display

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"codeberg.org/puregotk/puregotk/v4/gdk"
	"codeberg.org/puregotk/puregotk/v4/gio"
	"codeberg.org/puregotk/puregotk/v4/gobject"
	"codeberg.org/puregotk/puregotk/v4/gtk"

	"github.com/jmylchreest/histui/internal/config"
	"github.com/jmylchreest/histui/internal/gtkruntime"
	layershell "github.com/jmylchreest/histui/internal/gtkruntime/layershell"
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
	visible     bool
	sources     gtkruntime.Sources
	ownedPopups map[*Popup]struct{}

	// Monitor hotplug watch: re-resolves the configured output when the set of
	// connected monitors changes. Both are set together and only touched on the
	// GTK main thread.
	monitorsModel         gio.ListModel
	monitorsChangedHandle uint32
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
		app:         app,
		config:      cfg,
		logger:      logger,
		widgets:     make([]*NotificationWidget, 0),
		ownedPopups: make(map[*Popup]struct{}),
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
	layershell.SetKeyboardMode(s.window, layershell.KeyboardModeNoneValue)

	// Set namespace for compositor rules
	layershell.SetNamespace(s.window, ptr("histui-notification"))

	// Pin the surface to the configured output (0 = let compositor decide)
	s.applyMonitor()
	s.watchMonitorOutputs()

	// Create the vertical container for notifications
	// Gap is 0 for unified stack appearance - separators are CSS borders
	s.container = gtk.NewBox(gtk.OrientationVerticalValue, 0)
	s.container.AddCssClass("notification-stack")
	s.window.SetChild(&s.container.Widget)

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
	layershell.SetAnchor(s.window, layershell.EdgeTopValue, false)
	layershell.SetAnchor(s.window, layershell.EdgeBottomValue, false)
	layershell.SetAnchor(s.window, layershell.EdgeLeftValue, false)
	layershell.SetAnchor(s.window, layershell.EdgeRightValue, false)

	switch pos {
	case config.PositionTopRight:
		layershell.SetAnchor(s.window, layershell.EdgeTopValue, true)
		layershell.SetAnchor(s.window, layershell.EdgeRightValue, true)
		layershell.SetMargin(s.window, layershell.EdgeTopValue, int32(offsetY))
		layershell.SetMargin(s.window, layershell.EdgeRightValue, int32(offsetX))

	case config.PositionTopLeft:
		layershell.SetAnchor(s.window, layershell.EdgeTopValue, true)
		layershell.SetAnchor(s.window, layershell.EdgeLeftValue, true)
		layershell.SetMargin(s.window, layershell.EdgeTopValue, int32(offsetY))
		layershell.SetMargin(s.window, layershell.EdgeLeftValue, int32(offsetX))

	case config.PositionTopCenter:
		layershell.SetAnchor(s.window, layershell.EdgeTopValue, true)
		layershell.SetMargin(s.window, layershell.EdgeTopValue, int32(offsetY))

	case config.PositionBottomRight:
		layershell.SetAnchor(s.window, layershell.EdgeBottomValue, true)
		layershell.SetAnchor(s.window, layershell.EdgeRightValue, true)
		layershell.SetMargin(s.window, layershell.EdgeBottomValue, int32(offsetY))
		layershell.SetMargin(s.window, layershell.EdgeRightValue, int32(offsetX))

	case config.PositionBottomLeft:
		layershell.SetAnchor(s.window, layershell.EdgeBottomValue, true)
		layershell.SetAnchor(s.window, layershell.EdgeLeftValue, true)
		layershell.SetMargin(s.window, layershell.EdgeBottomValue, int32(offsetY))
		layershell.SetMargin(s.window, layershell.EdgeLeftValue, int32(offsetX))

	case config.PositionBottomCenter:
		layershell.SetAnchor(s.window, layershell.EdgeBottomValue, true)
		layershell.SetMargin(s.window, layershell.EdgeBottomValue, int32(offsetY))
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

	defer monitor.Unref()
	layershell.SetMonitor(s.window, monitor)
	s.logger.Debug("using output for notifications",
		"monitor", sel.String(),
		"connector", monitor.GetConnector(),
		"description", monitor.GetDescription(),
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
	defer display.Unref()
	monitors := display.GetMonitors()
	if monitors == nil {
		return
	}

	s.monitorsModel = monitors
	object := gobject.ObjectNewFromInternalPtr(monitors.GoPointer())
	s.monitorsChangedHandle = object.ConnectSignal("items-changed", ptr(func() {
		s.logger.Debug("monitor layout changed, re-applying output selection")
		s.applyMonitor()
	}))
}

func nullMonitor() *gdk.Monitor { return nil }

// logDetectedOutputs logs every connected output at debug so users can map the
// 1-indexed `monitor` config value to a physical monitor — emitted on every
// applyMonitor, including monitor=0, so the indices are visible without having
// to pin one first.
func (s *NotificationStack) logDetectedOutputs() {
	display := gdk.DisplayGetDefault()
	if display == nil {
		return
	}
	defer display.Unref()
	monitors := display.GetMonitors()
	if monitors == nil {
		return
	}

	defer gobject.ObjectNewFromInternalPtr(monitors.GoPointer()).Unref()
	n := monitors.GetNItems()
	s.logger.Debug("detected outputs", "count", n)
	for i := uint32(0); i < n; i++ {
		m := castMonitor(monitors.GetItem(i))
		if m == nil {
			continue
		}
		var g gdk.Rectangle
		m.GetGeometry(&g)
		s.logger.Debug("detected output",
			"index", i+1,
			"connector", m.GetConnector(),
			"description", m.GetDescription(),
			"geometry", fmt.Sprintf("%dx%d+%d+%d", g.Width, g.Height, g.X, g.Y))
		m.Unref()
	}
}

// resolveMonitor maps a MonitorSelector to a *gdk.Monitor. Returns nil for auto
// or when no display/monitors are available, letting the compositor choose.
//
// By connector name: matches m.GetConnector() (or m.GetDescription()) case-insensitively;
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

	defer display.Unref()
	monitors := display.GetMonitors()
	if monitors == nil {
		s.logger.Warn("no monitors list available, deferring output selection to compositor")
		return nil
	}

	defer gobject.ObjectNewFromInternalPtr(monitors.GoPointer()).Unref()
	n := monitors.GetNItems()

	if sel.Name != "" {
		for i := uint32(0); i < n; i++ {
			m := castMonitor(monitors.GetItem(i))
			if m == nil {
				continue
			}
			if strings.EqualFold(m.GetConnector(), sel.Name) || strings.EqualFold(m.GetDescription(), sel.Name) {
				return m
			}
			m.Unref()
		}
		s.logger.Warn("configured monitor name not found, deferring output selection to compositor",
			"name", sel.Name, "available", n)
		return nil
	}

	index := uint32(sel.Index - 1)
	if index >= n {
		s.logger.Warn("configured monitor not available, falling back to first output",
			"configured", sel.Index, "available", n)
		index = 0
	}

	return castMonitor(monitors.GetItem(index))
}

// castMonitor consumes the owned list-model item reference.
func castMonitor(ptr uintptr) *gdk.Monitor {
	if ptr == 0 {
		return nil
	}
	return gdk.MonitorNewFromInternalPtr(ptr)
}

// Add adds a notification widget to the stack.
// Position depends on config.Display.NewOnTop:
//   - false (default): new notifications at bottom of stack
//   - true: new notifications at top of stack
func (s *NotificationStack) Add(widget *NotificationWidget) {
	s.mu.Lock()
	s.ownedPopups[widget.Popup] = struct{}{}

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
	s.sources.Idle(func() {
		// Add widget to container
		if newOnTop {
			s.container.Prepend(&container.Widget)
		} else {
			s.container.Append(&container.Widget)
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
		s.sources.Idle(func() {
			s.container.Remove(&container.Widget)
			removed.Popup.dispose()
			delete(s.ownedPopups, removed.Popup)
		})

		s.logger.Debug("removed notification from stack",
			"dbus_id", dbusID,
			"remaining", len(s.widgets),
		)
	}

	// Hide window if empty (defer to idle to avoid race)
	if len(s.widgets) == 0 && s.visible {
		s.sources.Idle(func() {
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
		s.sources.Idle(func() {
			s.container.Remove(&container.Widget)
			removed.Popup.dispose()
			delete(s.ownedPopups, removed.Popup)
		})
	}

	// Hide window if empty (defer to idle to avoid race)
	if len(s.widgets) == 0 && s.visible {
		s.sources.Idle(func() {
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
	s.sources.Idle(func() {
		for i, container := range containers {
			s.container.Remove(&container.Widget)
			removed[i].Popup.dispose()
			delete(s.ownedPopups, removed[i].Popup)
		}
	})

	// Hide window (defer to idle to avoid race)
	if s.visible {
		s.sources.Idle(func() {
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
	s.sources.Idle(func() {
		for _, update := range updates {
			updateStackPositionClass(update.box, update.position)
		}
	})
}

// updateStackPositionClass updates the stack position CSS class on a widget.
func updateStackPositionClass(box *gtk.Box, position string) {
	// Remove old stack position classes
	box.RemoveCssClass("stack-single")
	box.RemoveCssClass("stack-first")
	box.RemoveCssClass("stack-middle")
	box.RemoveCssClass("stack-last")

	// Add new class
	if position != "" {
		box.AddCssClass("stack-" + position)
	}
}

// UpdateConfig updates the stack configuration.
func (s *NotificationStack) UpdateConfig(cfg *config.DaemonConfig) {
	s.mu.Lock()
	s.config = cfg
	s.mu.Unlock()

	// Re-apply output selection and position (monitor layout may have changed)
	s.sources.Idle(func() {
		s.applyMonitor()
		s.updatePosition()
	})
}

// Destroy cleans up the stack window.
func (s *NotificationStack) Destroy() {
	if s.monitorsModel != nil && s.monitorsChangedHandle != 0 {
		gobject.ObjectNewFromInternalPtr(s.monitorsModel.GoPointer()).DisconnectSignal(s.monitorsChangedHandle)
		s.monitorsChangedHandle = 0
		gobject.ObjectNewFromInternalPtr(s.monitorsModel.GoPointer()).Unref()
		s.monitorsModel = nil
	}
	s.sources.Close()
	if s.window != nil {
		s.window.Destroy()
		s.window.Unref()
		s.window = nil
		s.container.Unref()
		s.container = nil
	}
	for popup := range s.ownedPopups {
		popup.dispose()
	}
	s.ownedPopups = nil
	s.widgets = nil
}

// configuredLayer returns the layer-shell layer from config.
func (s *NotificationStack) configuredLayer() layershell.Layer {
	switch config.Layer(s.config.Display.Layer) {
	case config.LayerBackground:
		return layershell.LayerBackgroundValue
	case config.LayerBottom:
		return layershell.LayerBottomValue
	case config.LayerOverlay:
		return layershell.LayerOverlayValue
	default:
		return layershell.LayerTopValue
	}
}

// Ensure adw is used (for libadwaita initialization)

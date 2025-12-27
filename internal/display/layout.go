package display

import (
	"log/slog"
	"unsafe"

	layershell "github.com/diamondburned/gotk4-layer-shell/pkg/gtk4layershell"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/jmylchreest/histui/internal/config"
)

// LayoutManager handles popup positioning and multi-monitor support.
type LayoutManager struct {
	config  *config.DaemonConfig
	display *gdk.Display
	logger  *slog.Logger
}

// NewLayoutManager creates a new layout manager.
func NewLayoutManager(cfg *config.DaemonConfig, logger *slog.Logger) *LayoutManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &LayoutManager{
		config:  cfg,
		display: gdk.DisplayGetDefault(),
		logger:  logger,
	}
}

// CalculatePosition returns the offset for a popup at the given stack position.
// The returned values are (offsetX, offsetY) from the configured anchor point.
func (l *LayoutManager) CalculatePosition(position int) (int, int) {
	offsetX := l.config.Display.OffsetX
	offsetY := l.config.Display.OffsetY

	// Add stacking offset
	stackOffset := position * (l.config.Display.MaxHeight + l.config.Display.Gap)
	offsetY += stackOffset

	return offsetX, offsetY
}

// GetMonitor returns the monitor to display popups on based on config.
// Config values:
// - 0: All monitors (returns nil, use default)
// - 1+: Specific monitor (1-indexed)
//
// Returns nil if the monitor is not available (fallback to primary).
func (l *LayoutManager) GetMonitor() *gdk.Monitor {
	if l.display == nil {
		return nil
	}

	monitorNum := l.config.Display.Monitor
	if monitorNum == 0 {
		// 0 means all monitors - use default behavior
		return nil
	}

	monitors := l.display.Monitors()
	if monitors == nil {
		l.logger.Warn("no monitors list available")
		return nil
	}

	// Convert to 0-indexed
	index := uint(monitorNum - 1)

	if index >= monitors.NItems() {
		l.logger.Warn("configured monitor not available, using primary",
			"configured", monitorNum,
			"available", monitors.NItems(),
		)
		// Fallback to primary monitor
		return getPrimaryMonitor(l.display)
	}

	obj := monitors.Item(index)
	if obj == nil {
		return nil
	}

	// Cast coreglib.Object to gdk.Monitor
	// The gotk4 bindings use pointer embedding, so we can wrap it
	return wrapMonitor(obj)
}

// getPrimaryMonitor returns the primary monitor or first available.
func getPrimaryMonitor(display *gdk.Display) *gdk.Monitor {
	monitors := display.Monitors()
	if monitors == nil || monitors.NItems() == 0 {
		return nil
	}

	// GTK4 doesn't have a "primary" concept in the same way
	// Return the first monitor as fallback
	obj := monitors.Item(0)
	if obj == nil {
		return nil
	}

	return wrapMonitor(obj)
}

// wrapMonitor wraps a coreglib.Object as a gdk.Monitor.
// This is necessary because gotk4 doesn't expose the wrapMonitor function.
func wrapMonitor(obj *glib.Object) *gdk.Monitor {
	if obj == nil {
		return nil
	}
	// The gdk.Monitor struct embeds a *coreglib.Object, so we can create
	// one by casting the native pointer. This is how gotk4 does it internally.
	// We use unsafe to access the internal structure.
	type monitor struct {
		_ [0]func()
		*glib.Object
	}
	m := &monitor{Object: obj}
	return (*gdk.Monitor)(unsafe.Pointer(m))
}

// SetMonitor configures a window to appear on the specified monitor.
func (l *LayoutManager) SetMonitor(window *gtk.Window, monitor *gdk.Monitor) {
	if monitor == nil {
		return
	}
	layershell.SetMonitor(window, monitor)
}

// UpdateStack recalculates positions for all popups in a stack.
// Returns a slice of (position, offsetX, offsetY) tuples.
func (l *LayoutManager) UpdateStack(count int) [][3]int {
	positions := make([][3]int, count)
	for i := range count {
		offsetX, offsetY := l.CalculatePosition(i)
		positions[i] = [3]int{i, offsetX, offsetY}
	}
	return positions
}

// IsBottom returns true if the configured position is at the bottom of the screen.
func (l *LayoutManager) IsBottom() bool {
	pos := config.Position(l.config.Display.Position)
	switch pos {
	case config.PositionBottomLeft, config.PositionBottomRight, config.PositionBottomCenter:
		return true
	default:
		return false
	}
}

// HandleMonitorChange should be called when monitors change.
// It updates the display reference and logs the change.
func (l *LayoutManager) HandleMonitorChange() {
	l.display = gdk.DisplayGetDefault()
	if l.display == nil {
		l.logger.Warn("no display available after monitor change")
		return
	}

	monitors := l.display.Monitors()
	if monitors != nil {
		l.logger.Info("monitor configuration changed", "count", monitors.NItems())
	}
}

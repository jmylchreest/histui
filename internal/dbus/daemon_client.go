// Package dbus provides D-Bus interfaces for histui.
package dbus

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/godbus/dbus/v5"
)

// SignalHandler handles D-Bus signals from the daemon.
type SignalHandler interface {
	OnDnDChanged(enabled bool)
	OnNotificationDisplayed(histuiID string)
	OnNotificationDismissed(histuiID string, reason string)
	OnConfigChanged()
}

// DaemonClient provides D-Bus communication with the histuid daemon.
// Operations gracefully fall back when daemon is not running.
type DaemonClient struct {
	conn    *dbus.Conn
	logger  *slog.Logger
	obj     dbus.BusObject
	signals chan *dbus.Signal

	mu        sync.Mutex
	closed    bool
	available bool
}

// NewDaemonClient creates a new D-Bus client for the histui daemon.
// Returns a client even if the daemon is not available (graceful degradation).
func NewDaemonClient(logger *slog.Logger) *DaemonClient {
	if logger == nil {
		logger = slog.Default()
	}

	client := &DaemonClient{
		logger: logger,
	}

	// Try to connect to session bus
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		logger.Debug("D-Bus session bus not available", "error", err)
		return client
	}

	client.conn = conn
	client.obj = conn.Object(DaemonInterface, DaemonPath)
	client.available = true // Assume available, will be set false on first failed call

	return client
}

// IsAvailable returns true if the daemon is responding on D-Bus.
func (c *DaemonClient) IsAvailable() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.available && c.conn != nil
}

// Close closes the D-Bus connection.
func (c *DaemonClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	if c.signals != nil {
		c.conn.RemoveSignal(c.signals)
		close(c.signals)
	}

	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Ping checks if the daemon is alive and returns the DnD state.
// Returns (dndEnabled, error). If error is non-nil, daemon is not available.
func (c *DaemonClient) Ping() (bool, error) {
	if c.conn == nil {
		return false, fmt.Errorf("not connected to D-Bus")
	}

	var dndEnabled bool
	err := c.obj.Call(DaemonInterface+".Ping", 0).Store(&dndEnabled)
	if err != nil {
		c.mu.Lock()
		c.available = false
		c.mu.Unlock()
		return false, fmt.Errorf("ping failed: %w", err)
	}

	c.mu.Lock()
	c.available = true
	c.mu.Unlock()
	return dndEnabled, nil
}

// SetDnD sets the Do Not Disturb state.
// No-op if daemon is not running.
func (c *DaemonClient) SetDnD(enabled bool) error {
	if c.conn == nil {
		return nil
	}

	err := c.obj.Call(DaemonInterface+".SetDnD", 0, enabled).Err
	if err != nil {
		c.mu.Lock()
		c.available = false
		c.mu.Unlock()
		return nil
	}
	return nil
}

// ToggleDnD toggles the DnD state and returns the new state.
func (c *DaemonClient) ToggleDnD() (bool, error) {
	if c.conn == nil {
		return false, nil
	}

	var newState bool
	err := c.obj.Call(DaemonInterface+".ToggleDnD", 0).Store(&newState)
	if err != nil {
		// Fall back to manual toggle using Ping to get current state
		current, _ := c.Ping()
		newState = !current
		_ = c.SetDnD(newState)
		return newState, nil
	}
	return newState, nil
}

// StopAudio stops any currently playing notification sound.
// No-op if daemon is not running.
func (c *DaemonClient) StopAudio() error {
	if c.conn == nil {
		return nil
	}

	err := c.obj.Call(DaemonInterface+".StopAudio", 0).Err
	if err != nil {
		// Daemon not running - nothing to stop
		return nil
	}
	return nil
}

// CloseNotification closes a specific notification popup by histui ID.
// No-op if daemon is not running. Returns true if closed.
func (c *DaemonClient) CloseNotification(histuiID string) (bool, error) {
	if c.conn == nil {
		return false, nil
	}

	var closed bool
	err := c.obj.Call(DaemonInterface+".CloseNotification", 0, histuiID).Store(&closed)
	if err != nil {
		// Daemon not running - no popup to close
		return false, nil
	}
	return closed, nil
}

// CloseNotifications closes multiple notification popups by histui IDs.
// No-op if daemon is not running.
func (c *DaemonClient) CloseNotifications(histuiIDs []string) error {
	for _, id := range histuiIDs {
		if _, err := c.CloseNotification(id); err != nil {
			return err
		}
	}
	return nil
}

// CloseAllNotifications closes all active notification popups.
// No-op if daemon is not running. Returns number closed.
func (c *DaemonClient) CloseAllNotifications() (int, error) {
	if c.conn == nil {
		return 0, nil
	}

	var count int32
	err := c.obj.Call(DaemonInterface+".CloseAllNotifications", 0).Store(&count)
	if err != nil {
		// Daemon not running - no popups to close
		return 0, nil
	}
	return int(count), nil
}

// GetActiveNotifications returns histui IDs of all active/queued popups.
// Returns empty slice if daemon is not running.
func (c *DaemonClient) GetActiveNotifications() ([]string, error) {
	if c.conn == nil {
		return []string{}, nil
	}

	var ids []string
	err := c.obj.Call(DaemonInterface+".GetActiveNotifications", 0).Store(&ids)
	if err != nil {
		return []string{}, nil
	}
	if ids == nil {
		ids = []string{}
	}
	return ids, nil
}

// IsNotificationActive checks if a notification has an active popup.
// Returns false if daemon is not running.
func (c *DaemonClient) IsNotificationActive(histuiID string) (bool, error) {
	if c.conn == nil {
		return false, nil
	}

	var active bool
	err := c.obj.Call(DaemonInterface+".IsNotificationActive", 0, histuiID).Store(&active)
	if err != nil {
		return false, nil
	}
	return active, nil
}

// GetStatus returns daemon status information.
// Returns nil if daemon is not running.
func (c *DaemonClient) GetStatus() (*DaemonStatus, error) {
	if c.conn == nil {
		return nil, nil
	}

	var statusJSON string
	err := c.obj.Call(DaemonInterface+".GetStatus", 0).Store(&statusJSON)
	if err != nil {
		return nil, nil
	}

	var status DaemonStatus
	if err := json.Unmarshal([]byte(statusJSON), &status); err != nil {
		return nil, fmt.Errorf("parse status: %w", err)
	}
	return &status, nil
}

// SubscribeSignals subscribes to daemon signals.
// The handler will be called on a separate goroutine.
func (c *DaemonClient) SubscribeSignals(handler SignalHandler) error {
	if c.conn == nil {
		return fmt.Errorf("not connected to D-Bus")
	}

	c.mu.Lock()
	if c.signals != nil {
		c.mu.Unlock()
		return fmt.Errorf("already subscribed to signals")
	}

	// Create signal channel
	c.signals = make(chan *dbus.Signal, 16)
	c.conn.Signal(c.signals)
	c.mu.Unlock()

	// Add match rules for daemon signals
	matchRules := []string{
		fmt.Sprintf("type='signal',interface='%s',member='DnDChanged'", DaemonInterface),
		fmt.Sprintf("type='signal',interface='%s',member='NotificationDisplayed'", DaemonInterface),
		fmt.Sprintf("type='signal',interface='%s',member='NotificationDismissed'", DaemonInterface),
		fmt.Sprintf("type='signal',interface='%s',member='ConfigChanged'", DaemonInterface),
	}

	for _, rule := range matchRules {
		if err := c.conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, rule).Err; err != nil {
			c.logger.Warn("failed to add signal match rule", "rule", rule, "error", err)
		}
	}

	// Start signal handler goroutine
	go c.handleSignals(handler)

	return nil
}

// handleSignals processes incoming D-Bus signals.
func (c *DaemonClient) handleSignals(handler SignalHandler) {
	for sig := range c.signals {
		if sig.Path != DaemonPath {
			continue
		}

		switch sig.Name {
		case DaemonInterface + ".DnDChanged":
			if len(sig.Body) >= 1 {
				if enabled, ok := sig.Body[0].(bool); ok {
					handler.OnDnDChanged(enabled)
				}
			}

		case DaemonInterface + ".NotificationDisplayed":
			if len(sig.Body) >= 1 {
				if histuiID, ok := sig.Body[0].(string); ok {
					handler.OnNotificationDisplayed(histuiID)
				}
			}

		case DaemonInterface + ".NotificationDismissed":
			if len(sig.Body) >= 2 {
				histuiID, ok1 := sig.Body[0].(string)
				reason, ok2 := sig.Body[1].(string)
				if ok1 && ok2 {
					handler.OnNotificationDismissed(histuiID, reason)
				}
			}

		case DaemonInterface + ".ConfigChanged":
			handler.OnConfigChanged()
		}
	}
}

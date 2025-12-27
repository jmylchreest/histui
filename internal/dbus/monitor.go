// Package dbus implements the org.freedesktop.Notifications D-Bus interface.
package dbus

import (
	"encoding/binary"
	"fmt"
	"log/slog"

	"github.com/godbus/dbus/v5"
)

// Monitor passively observes D-Bus notification traffic without claiming ownership.
// This allows running alongside another notification daemon (like dunst).
type Monitor struct {
	conn   *dbus.Conn
	logger *slog.Logger

	onNotify NotificationHandler
}

// NewMonitor creates a new notification monitor.
func NewMonitor(logger *slog.Logger) *Monitor {
	if logger == nil {
		logger = slog.Default()
	}
	return &Monitor{
		logger: logger,
	}
}

// SetNotifyHandler sets the callback for received notifications.
func (m *Monitor) SetNotifyHandler(handler NotificationHandler) {
	m.onNotify = handler
}

// Start begins monitoring D-Bus for notification traffic.
func (m *Monitor) Start() error {
	conn, err := dbus.SessionBus()
	if err != nil {
		return fmt.Errorf("failed to connect to session bus: %w", err)
	}
	m.conn = conn

	// Become a monitor - this allows us to see all bus traffic
	// We specifically want to see Notify method calls to org.freedesktop.Notifications
	rules := []string{
		"type='method_call',interface='org.freedesktop.Notifications',member='Notify'",
	}

	// BecomeMonitor has no return value - just check for error
	err = conn.BusObject().Call(
		"org.freedesktop.DBus.Monitoring.BecomeMonitor",
		0,
		rules,
		uint32(0),
	).Err

	if err != nil {
		// BecomeMonitor might not be available (older D-Bus versions)
		// Fall back to eavesdropping via match rules
		m.logger.Warn("BecomeMonitor not available, trying AddMatch", "error", err)
		return m.startWithAddMatch()
	}

	m.logger.Info("started D-Bus monitor using BecomeMonitor")

	// Start processing messages
	go m.processMessages()

	return nil
}

// startWithAddMatch uses the older AddMatch API for eavesdropping.
func (m *Monitor) startWithAddMatch() error {
	// Add match rule to receive Notify calls
	matchRule := "type='method_call',interface='org.freedesktop.Notifications',member='Notify',eavesdrop='true'"

	err := m.conn.BusObject().Call(
		"org.freedesktop.DBus.AddMatch",
		0,
		matchRule,
	).Err

	if err != nil {
		return fmt.Errorf("failed to add match rule (eavesdrop may require permissions): %w", err)
	}

	m.logger.Info("started D-Bus monitor using AddMatch with eavesdrop")

	// Start processing messages
	go m.processMessages()

	return nil
}

// processMessages reads and processes D-Bus messages.
func (m *Monitor) processMessages() {
	ch := make(chan *dbus.Message, 100)
	m.conn.Eavesdrop(ch)

	for msg := range ch {
		if msg.Type != dbus.TypeMethodCall {
			continue
		}

		// Check if this is a Notify call
		if msg.Headers[dbus.FieldInterface].Value() != "org.freedesktop.Notifications" {
			continue
		}
		if msg.Headers[dbus.FieldMember].Value() != "Notify" {
			continue
		}

		m.handleNotify(msg)
	}
}

// handleNotify parses a Notify method call and invokes the handler.
func (m *Monitor) handleNotify(msg *dbus.Message) {
	// Notify(app_name, replaces_id, app_icon, summary, body, actions, hints, expire_timeout)
	if len(msg.Body) < 8 {
		m.logger.Warn("malformed Notify call", "body_len", len(msg.Body))
		return
	}

	notification := &DBusNotification{}

	// Parse arguments
	var ok bool
	if notification.AppName, ok = msg.Body[0].(string); !ok {
		m.logger.Warn("invalid app_name type")
		return
	}
	if notification.ReplacesID, ok = msg.Body[1].(uint32); !ok {
		m.logger.Warn("invalid replaces_id type")
		return
	}
	if notification.AppIcon, ok = msg.Body[2].(string); !ok {
		m.logger.Warn("invalid app_icon type")
		return
	}
	if notification.Summary, ok = msg.Body[3].(string); !ok {
		m.logger.Warn("invalid summary type")
		return
	}
	if notification.Body, ok = msg.Body[4].(string); !ok {
		m.logger.Warn("invalid body type")
		return
	}

	// Actions is []string
	if actions, ok := msg.Body[5].([]string); ok {
		notification.Actions = actions
	}

	// Hints is map[string]dbus.Variant
	if hints, ok := msg.Body[6].(map[string]dbus.Variant); ok {
		notification.Hints = hints
	}

	// ExpireTimeout is int32
	if timeout, ok := msg.Body[7].(int32); ok {
		notification.ExpireTimeout = timeout
	}

	// Generate a pseudo-ID for the notification
	// In monitor mode, we don't get the real ID from the server response
	id := generateMonitorID(notification)

	m.logger.Debug("captured notification",
		"app", notification.AppName,
		"summary", notification.Summary,
		"id", id)

	if m.onNotify != nil {
		m.onNotify(notification, id)
	}
}

// generateMonitorID creates a pseudo-ID for monitored notifications.
// Since we're eavesdropping, we don't see the server's response with the real ID.
// We generate a hash-based ID from the notification content.
func generateMonitorID(n *DBusNotification) uint32 {
	// Create a simple hash from app+summary+timestamp
	data := []byte(n.AppName + n.Summary)
	var hash uint32
	for _, b := range data {
		hash = hash*31 + uint32(b)
	}
	// Add some randomness from notification properties
	hash ^= binary.LittleEndian.Uint32([]byte{
		byte(len(n.Body)),
		byte(len(n.Actions)),
		byte(n.ExpireTimeout),
		byte(n.ExpireTimeout >> 8),
	})
	return hash
}

// Stop stops the monitor.
func (m *Monitor) Stop() error {
	if m.conn != nil {
		return m.conn.Close()
	}
	return nil
}

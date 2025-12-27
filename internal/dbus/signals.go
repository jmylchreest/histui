package dbus

import (
	"fmt"

	"github.com/godbus/dbus/v5"
)

// EmitNotificationClosed emits the NotificationClosed signal.
// This signal is emitted when a notification is closed, either by timeout,
// user dismissal, or explicit close request.
func (s *NotificationServer) EmitNotificationClosed(id uint32, reason CloseReason) error {
	if s.conn == nil {
		return fmt.Errorf("not connected to D-Bus")
	}

	err := s.conn.Emit(DBusPath, DBusInterface+".NotificationClosed", id, uint32(reason))
	if err != nil {
		return fmt.Errorf("failed to emit NotificationClosed signal: %w", err)
	}

	s.logger.Debug("emitted NotificationClosed signal", "id", id, "reason", reason.String())
	return nil
}

// EmitActionInvoked emits the ActionInvoked signal.
// This signal is emitted when the user invokes an action on a notification.
func (s *NotificationServer) EmitActionInvoked(id uint32, actionKey string) error {
	if s.conn == nil {
		return fmt.Errorf("not connected to D-Bus")
	}

	err := s.conn.Emit(DBusPath, DBusInterface+".ActionInvoked", id, actionKey)
	if err != nil {
		return fmt.Errorf("failed to emit ActionInvoked signal: %w", err)
	}

	s.logger.Debug("emitted ActionInvoked signal", "id", id, "action_key", actionKey)
	return nil
}

// EmitActivationToken emits the ActivationToken signal (optional, spec 1.2+).
// This is emitted before ActionInvoked when the compositor provides an activation token.
func (s *NotificationServer) EmitActivationToken(id uint32, activationToken string) error {
	if s.conn == nil {
		return fmt.Errorf("not connected to D-Bus")
	}

	err := s.conn.Emit(DBusPath, DBusInterface+".ActivationToken", id, activationToken)
	if err != nil {
		return fmt.Errorf("failed to emit ActivationToken signal: %w", err)
	}

	s.logger.Debug("emitted ActivationToken signal", "id", id)
	return nil
}

// CloseWithReason closes a notification and emits the appropriate signal.
// This is a convenience method that combines MarkClosed and EmitNotificationClosed.
func (s *NotificationServer) CloseWithReason(id uint32, reason CloseReason) error {
	s.MarkClosed(id)
	return s.EmitNotificationClosed(id, reason)
}

// InvokeAction invokes an action on a notification and emits the signal.
// If the notification is not resident, it is also closed after the action.
func (s *NotificationServer) InvokeAction(id uint32, actionKey string, resident bool) error {
	if err := s.EmitActionInvoked(id, actionKey); err != nil {
		return err
	}

	// Non-resident notifications are closed after action invocation
	if !resident {
		return s.CloseWithReason(id, CloseReasonDismissed)
	}

	return nil
}

// Connection returns the underlying D-Bus connection.
// This can be used for advanced operations like calling methods on other services.
func (s *NotificationServer) Connection() *dbus.Conn {
	return s.conn
}

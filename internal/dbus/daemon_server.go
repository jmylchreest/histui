// Package dbus provides D-Bus interfaces for histuid daemon.
package dbus

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
)

const (
	// DaemonInterface is the histui daemon control interface name.
	DaemonInterface = "org.freedesktop.histui.Daemon"
	// DaemonPath is the daemon object path.
	DaemonPath = "/org/freedesktop/histui/Daemon"
)

// DaemonHandler defines the interface for daemon control operations.
type DaemonHandler interface {
	// DnD control
	SetDnD(enabled bool) error
	GetDnD() bool

	// Audio control
	StopAudio()

	// Popup control
	CloseNotification(histuiID string) bool
	CloseAllNotifications() int

	// Active popup tracking
	GetActiveNotifications() []string
	IsNotificationActive(histuiID string) bool
}

// DaemonStatus contains daemon status information.
type DaemonStatus struct {
	Version     string `json:"version"`
	DnDEnabled  bool   `json:"dnd_enabled"`
	ActiveCount int    `json:"active_count"`
}

// DaemonServer implements the org.freedesktop.histui.Daemon D-Bus interface.
type DaemonServer struct {
	conn    *dbus.Conn
	handler DaemonHandler
	logger  *slog.Logger
	version string

	mu      sync.Mutex
	running bool
}

// NewDaemonServer creates a new DaemonServer.
// The connection should be shared from NotificationServer via Connection().
func NewDaemonServer(conn *dbus.Conn, handler DaemonHandler, version string, logger *slog.Logger) *DaemonServer {
	if logger == nil {
		logger = slog.Default()
	}
	return &DaemonServer{
		conn:    conn,
		handler: handler,
		version: version,
		logger:  logger,
	}
}

// Start exports the daemon interface on the existing D-Bus connection.
func (s *DaemonServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("daemon server already running")
	}

	if s.conn == nil {
		return fmt.Errorf("no D-Bus connection available")
	}

	// Request the well-known name for the daemon interface
	// This allows clients to connect to org.freedesktop.histui.Daemon
	reply, err := s.conn.RequestName(DaemonInterface, dbus.NameFlagDoNotQueue)
	if err != nil {
		return fmt.Errorf("failed to request D-Bus name %s: %w", DaemonInterface, err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return fmt.Errorf("failed to become primary owner of D-Bus name %s", DaemonInterface)
	}

	// Export the daemon server object
	if err := s.conn.Export(s, DaemonPath, DaemonInterface); err != nil {
		return fmt.Errorf("failed to export daemon object: %w", err)
	}

	// Export introspection data
	node := &introspect.Node{
		Name: DaemonPath,
		Interfaces: []introspect.Interface{
			introspect.IntrospectData,
			{
				Name:    DaemonInterface,
				Methods: daemonMethods(),
				Signals: daemonSignals(),
			},
		},
	}
	if err := s.conn.Export(introspect.NewIntrospectable(node), DaemonPath,
		"org.freedesktop.DBus.Introspectable"); err != nil {
		return fmt.Errorf("failed to export daemon introspectable: %w", err)
	}

	s.running = true
	s.logger.Info("D-Bus daemon interface started", "interface", DaemonInterface, "path", DaemonPath)
	return nil
}

// Stop unexports the daemon interface.
func (s *DaemonServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	// Unexport the object (connection is shared, don't close it)
	_ = s.conn.Export(nil, DaemonPath, DaemonInterface)

	// Release the well-known name
	_, _ = s.conn.ReleaseName(DaemonInterface)

	s.running = false
	s.logger.Info("D-Bus daemon interface stopped")
	return nil
}

// Ping checks if the daemon is responsive.
// D-Bus method: Ping() -> b
func (s *DaemonServer) Ping() (bool, *dbus.Error) {
	s.logger.Debug("Ping called")
	return true, nil
}

// GetDnD returns the current Do Not Disturb state.
// D-Bus method: GetDnD() -> b
func (s *DaemonServer) GetDnD() (bool, *dbus.Error) {
	if s.handler == nil {
		return false, nil
	}
	enabled := s.handler.GetDnD()
	s.logger.Debug("GetDnD called", "enabled", enabled)
	return enabled, nil
}

// SetDnD sets the Do Not Disturb state.
// D-Bus method: SetDnD(b) -> nothing
func (s *DaemonServer) SetDnD(enabled bool) *dbus.Error {
	s.logger.Debug("SetDnD called", "enabled", enabled)
	if s.handler == nil {
		return nil
	}
	if err := s.handler.SetDnD(enabled); err != nil {
		return dbus.MakeFailedError(err)
	}
	return nil
}

// ToggleDnD toggles the Do Not Disturb state and returns the new state.
// D-Bus method: ToggleDnD() -> b
func (s *DaemonServer) ToggleDnD() (bool, *dbus.Error) {
	if s.handler == nil {
		return false, nil
	}
	current := s.handler.GetDnD()
	newState := !current
	if err := s.handler.SetDnD(newState); err != nil {
		return current, dbus.MakeFailedError(err)
	}
	s.logger.Debug("ToggleDnD called", "new_state", newState)
	return newState, nil
}

// StopAudio stops any currently playing notification sound.
// D-Bus method: StopAudio() -> nothing
func (s *DaemonServer) StopAudio() *dbus.Error {
	s.logger.Debug("StopAudio called")
	if s.handler != nil {
		s.handler.StopAudio()
	}
	return nil
}

// CloseNotification closes a notification popup by histui ID.
// D-Bus method: CloseNotification(s) -> b
func (s *DaemonServer) CloseNotification(histuiID string) (bool, *dbus.Error) {
	s.logger.Debug("CloseNotification called", "histui_id", histuiID)
	if s.handler == nil {
		return false, nil
	}
	closed := s.handler.CloseNotification(histuiID)
	return closed, nil
}

// CloseAllNotifications closes all active notification popups.
// D-Bus method: CloseAllNotifications() -> i
func (s *DaemonServer) CloseAllNotifications() (int32, *dbus.Error) {
	s.logger.Debug("CloseAllNotifications called")
	if s.handler == nil {
		return 0, nil
	}
	count := s.handler.CloseAllNotifications()
	return int32(count), nil
}

// GetActiveNotifications returns histui IDs of all active/queued popups.
// D-Bus method: GetActiveNotifications() -> as
func (s *DaemonServer) GetActiveNotifications() ([]string, *dbus.Error) {
	s.logger.Debug("GetActiveNotifications called")
	if s.handler == nil {
		return []string{}, nil
	}
	ids := s.handler.GetActiveNotifications()
	if ids == nil {
		ids = []string{}
	}
	return ids, nil
}

// IsNotificationActive checks if a notification has an active popup.
// D-Bus method: IsNotificationActive(s) -> b
func (s *DaemonServer) IsNotificationActive(histuiID string) (bool, *dbus.Error) {
	if s.handler == nil {
		return false, nil
	}
	active := s.handler.IsNotificationActive(histuiID)
	return active, nil
}

// GetStatus returns daemon status information as JSON.
// D-Bus method: GetStatus() -> s
func (s *DaemonServer) GetStatus() (string, *dbus.Error) {
	s.logger.Debug("GetStatus called")
	status := DaemonStatus{
		Version:    s.version,
		DnDEnabled: false,
	}
	if s.handler != nil {
		status.DnDEnabled = s.handler.GetDnD()
		status.ActiveCount = len(s.handler.GetActiveNotifications())
	}
	data, err := json.Marshal(status)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	return string(data), nil
}

// EmitDnDChanged emits a signal when DnD state changes.
func (s *DaemonServer) EmitDnDChanged(enabled bool) error {
	if s.conn == nil {
		return fmt.Errorf("not connected to D-Bus")
	}
	err := s.conn.Emit(DaemonPath, DaemonInterface+".DnDChanged", enabled)
	if err != nil {
		return fmt.Errorf("failed to emit DnDChanged signal: %w", err)
	}
	s.logger.Debug("emitted DnDChanged signal", "enabled", enabled)
	return nil
}

// EmitNotificationDisplayed emits a signal when a popup is shown.
func (s *DaemonServer) EmitNotificationDisplayed(histuiID string) error {
	if s.conn == nil {
		return fmt.Errorf("not connected to D-Bus")
	}
	err := s.conn.Emit(DaemonPath, DaemonInterface+".NotificationDisplayed", histuiID)
	if err != nil {
		return fmt.Errorf("failed to emit NotificationDisplayed signal: %w", err)
	}
	s.logger.Debug("emitted NotificationDisplayed signal", "histui_id", histuiID)
	return nil
}

// EmitNotificationDismissed emits a signal when a popup is closed.
func (s *DaemonServer) EmitNotificationDismissed(histuiID string, reason string) error {
	if s.conn == nil {
		return fmt.Errorf("not connected to D-Bus")
	}
	err := s.conn.Emit(DaemonPath, DaemonInterface+".NotificationDismissed", histuiID, reason)
	if err != nil {
		return fmt.Errorf("failed to emit NotificationDismissed signal: %w", err)
	}
	s.logger.Debug("emitted NotificationDismissed signal", "histui_id", histuiID, "reason", reason)
	return nil
}

// EmitConfigChanged emits a signal when configuration is reloaded.
func (s *DaemonServer) EmitConfigChanged() error {
	if s.conn == nil {
		return fmt.Errorf("not connected to D-Bus")
	}
	err := s.conn.Emit(DaemonPath, DaemonInterface+".ConfigChanged")
	if err != nil {
		return fmt.Errorf("failed to emit ConfigChanged signal: %w", err)
	}
	s.logger.Debug("emitted ConfigChanged signal")
	return nil
}

// daemonMethods returns the D-Bus method introspection data.
func daemonMethods() []introspect.Method {
	return []introspect.Method{
		{
			Name: "Ping",
			Args: []introspect.Arg{
				{Name: "alive", Type: "b", Direction: "out"},
			},
		},
		{
			Name: "GetDnD",
			Args: []introspect.Arg{
				{Name: "enabled", Type: "b", Direction: "out"},
			},
		},
		{
			Name: "SetDnD",
			Args: []introspect.Arg{
				{Name: "enabled", Type: "b", Direction: "in"},
			},
		},
		{
			Name: "ToggleDnD",
			Args: []introspect.Arg{
				{Name: "new_state", Type: "b", Direction: "out"},
			},
		},
		{
			Name: "StopAudio",
		},
		{
			Name: "CloseNotification",
			Args: []introspect.Arg{
				{Name: "histui_id", Type: "s", Direction: "in"},
				{Name: "closed", Type: "b", Direction: "out"},
			},
		},
		{
			Name: "CloseAllNotifications",
			Args: []introspect.Arg{
				{Name: "count", Type: "i", Direction: "out"},
			},
		},
		{
			Name: "GetActiveNotifications",
			Args: []introspect.Arg{
				{Name: "histui_ids", Type: "as", Direction: "out"},
			},
		},
		{
			Name: "IsNotificationActive",
			Args: []introspect.Arg{
				{Name: "histui_id", Type: "s", Direction: "in"},
				{Name: "active", Type: "b", Direction: "out"},
			},
		},
		{
			Name: "GetStatus",
			Args: []introspect.Arg{
				{Name: "status_json", Type: "s", Direction: "out"},
			},
		},
	}
}

// daemonSignals returns the D-Bus signal introspection data.
func daemonSignals() []introspect.Signal {
	return []introspect.Signal{
		{
			Name: "DnDChanged",
			Args: []introspect.Arg{
				{Name: "enabled", Type: "b"},
			},
		},
		{
			Name: "NotificationDisplayed",
			Args: []introspect.Arg{
				{Name: "histui_id", Type: "s"},
			},
		},
		{
			Name: "NotificationDismissed",
			Args: []introspect.Arg{
				{Name: "histui_id", Type: "s"},
				{Name: "reason", Type: "s"},
			},
		},
		{
			Name: "ConfigChanged",
		},
	}
}

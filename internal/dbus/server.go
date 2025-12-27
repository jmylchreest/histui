// Package dbus implements the org.freedesktop.Notifications D-Bus interface.
package dbus

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
)

const (
	// DBusInterface is the notification interface name.
	DBusInterface = "org.freedesktop.Notifications"
	// DBusPath is the notification object path.
	DBusPath = "/org/freedesktop/Notifications"
	// DBusBusName is the bus name to claim.
	DBusBusName = "org.freedesktop.Notifications"
)

// NotificationHandler is called when a new notification is received.
type NotificationHandler func(notification *DBusNotification, id uint32)

// CloseHandler is called when CloseNotification is requested.
type CloseHandler func(id uint32)

// NotificationServer implements the org.freedesktop.Notifications D-Bus interface.
type NotificationServer struct {
	conn   *dbus.Conn
	logger *slog.Logger

	// ID generation
	nextID atomic.Uint32

	// Handlers
	notifyHandler NotificationHandler
	closeHandler  CloseHandler

	// Tracking active notifications for signal emission
	mu         sync.RWMutex
	activeIDs  map[uint32]bool // D-Bus IDs currently active
	serverInfo ServerInfo
	running    bool
	stopCh     chan struct{}
}

// NewNotificationServer creates a new NotificationServer.
func NewNotificationServer(logger *slog.Logger) *NotificationServer {
	if logger == nil {
		logger = slog.Default()
	}
	return &NotificationServer{
		logger:     logger,
		activeIDs:  make(map[uint32]bool),
		serverInfo: DefaultServerInfo(),
		stopCh:     make(chan struct{}),
	}
}

// SetNotifyHandler sets the handler called when a notification is received.
func (s *NotificationServer) SetNotifyHandler(handler NotificationHandler) {
	s.notifyHandler = handler
}

// SetCloseHandler sets the handler called when CloseNotification is requested.
func (s *NotificationServer) SetCloseHandler(handler CloseHandler) {
	s.closeHandler = handler
}

// SetServerInfo sets the server information returned by GetServerInformation.
func (s *NotificationServer) SetServerInfo(info ServerInfo) {
	s.serverInfo = info
}

// Start connects to the session bus and exports the notification service.
func (s *NotificationServer) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("server already running")
	}
	s.mu.Unlock()

	conn, err := dbus.SessionBus()
	if err != nil {
		return fmt.Errorf("failed to connect to session bus: %w", err)
	}
	s.conn = conn

	// Export the notification server object
	if err := conn.Export(s, DBusPath, DBusInterface); err != nil {
		return fmt.Errorf("failed to export object: %w", err)
	}

	// Export introspection data
	node := &introspect.Node{
		Name: DBusPath,
		Interfaces: []introspect.Interface{
			introspect.IntrospectData,
			{
				Name:    DBusInterface,
				Methods: notificationMethods(),
				Signals: notificationSignals(),
			},
		},
	}
	if err := conn.Export(introspect.NewIntrospectable(node), DBusPath,
		"org.freedesktop.DBus.Introspectable"); err != nil {
		return fmt.Errorf("failed to export introspectable: %w", err)
	}

	// Request the bus name
	reply, err := conn.RequestName(DBusBusName, dbus.NameFlagDoNotQueue|dbus.NameFlagReplaceExisting)
	if err != nil {
		return fmt.Errorf("failed to request bus name: %w", err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return fmt.Errorf("bus name %s already taken", DBusBusName)
	}

	s.mu.Lock()
	s.running = true
	s.stopCh = make(chan struct{})
	s.mu.Unlock()

	s.logger.Info("D-Bus notification server started", "interface", DBusInterface, "path", DBusPath)
	return nil
}

// Stop releases the bus name and closes the connection.
func (s *NotificationServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	close(s.stopCh)
	s.running = false

	if s.conn != nil {
		_, err := s.conn.ReleaseName(DBusBusName)
		if err != nil {
			s.logger.Warn("failed to release bus name", "error", err)
		}
		// Don't close the connection as it's shared (SessionBus)
	}

	s.logger.Info("D-Bus notification server stopped")
	return nil
}

// GetCapabilities returns the list of capabilities supported by this server.
// D-Bus method: GetCapabilities() -> as
func (s *NotificationServer) GetCapabilities() ([]string, *dbus.Error) {
	s.logger.Debug("GetCapabilities called")
	return ServerCapabilities, nil
}

// GetServerInformation returns information about the notification server.
// D-Bus method: GetServerInformation() -> (ssss)
func (s *NotificationServer) GetServerInformation() (string, string, string, string, *dbus.Error) {
	s.logger.Debug("GetServerInformation called")
	return s.serverInfo.Name, s.serverInfo.Vendor, s.serverInfo.Version, s.serverInfo.SpecVersion, nil
}

// Notify handles incoming notification requests.
// D-Bus method: Notify(susssasa{sv}i) -> u
func (s *NotificationServer) Notify(
	appName string,
	replacesID uint32,
	appIcon string,
	summary string,
	body string,
	actions []string,
	hints map[string]dbus.Variant,
	expireTimeout int32,
) (uint32, *dbus.Error) {
	// Determine the notification ID
	var id uint32
	if replacesID > 0 {
		// Use the replacement ID if provided
		id = replacesID
	} else {
		// Generate a new ID
		id = s.nextID.Add(1)
	}

	s.logger.Debug("Notify called",
		"app_name", appName,
		"replaces_id", replacesID,
		"summary", summary,
		"id", id,
	)

	// Create the notification struct
	notification := &DBusNotification{
		AppName:       appName,
		ReplacesID:    replacesID,
		AppIcon:       appIcon,
		Summary:       summary,
		Body:          body,
		Actions:       actions,
		Hints:         hints,
		ExpireTimeout: expireTimeout,
	}

	// Track the notification as active
	s.mu.Lock()
	s.activeIDs[id] = true
	s.mu.Unlock()

	// Call the handler if set
	if s.notifyHandler != nil {
		s.notifyHandler(notification, id)
	}

	return id, nil
}

// CloseNotification closes a notification by ID.
// D-Bus method: CloseNotification(u) -> nothing
func (s *NotificationServer) CloseNotification(id uint32) *dbus.Error {
	s.logger.Debug("CloseNotification called", "id", id)

	s.mu.Lock()
	_, exists := s.activeIDs[id]
	if exists {
		delete(s.activeIDs, id)
	}
	s.mu.Unlock()

	if exists && s.closeHandler != nil {
		s.closeHandler(id)
	}

	// Emit the NotificationClosed signal with reason "closed by request"
	if exists {
		if err := s.EmitNotificationClosed(id, CloseReasonClosed); err != nil {
			s.logger.Warn("failed to emit NotificationClosed signal", "id", id, "error", err)
		}
	}

	return nil
}

// MarkClosed marks a notification as closed (removes from active tracking).
// This should be called when a notification is closed by timeout or user action.
func (s *NotificationServer) MarkClosed(id uint32) {
	s.mu.Lock()
	delete(s.activeIDs, id)
	s.mu.Unlock()
}

// NotifyInternal creates a notification internally without going through D-Bus.
// This is used for internal daemon notifications (config reload, errors, etc.).
// Returns the notification ID.
func (s *NotificationServer) NotifyInternal(notification *DBusNotification) uint32 {
	// Generate a new ID
	id := s.nextID.Add(1)

	s.logger.Debug("NotifyInternal called",
		"app_name", notification.AppName,
		"summary", notification.Summary,
		"id", id,
	)

	// Track the notification as active
	s.mu.Lock()
	s.activeIDs[id] = true
	s.mu.Unlock()

	// Call the handler if set
	if s.notifyHandler != nil {
		s.notifyHandler(notification, id)
	}

	return id
}

// IsActive returns true if the notification ID is currently active.
func (s *NotificationServer) IsActive(id uint32) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeIDs[id]
}

// notificationMethods returns the D-Bus method introspection data.
func notificationMethods() []introspect.Method {
	return []introspect.Method{
		{
			Name: "GetCapabilities",
			Args: []introspect.Arg{
				{Name: "capabilities", Type: "as", Direction: "out"},
			},
		},
		{
			Name: "GetServerInformation",
			Args: []introspect.Arg{
				{Name: "name", Type: "s", Direction: "out"},
				{Name: "vendor", Type: "s", Direction: "out"},
				{Name: "version", Type: "s", Direction: "out"},
				{Name: "spec_version", Type: "s", Direction: "out"},
			},
		},
		{
			Name: "Notify",
			Args: []introspect.Arg{
				{Name: "app_name", Type: "s", Direction: "in"},
				{Name: "replaces_id", Type: "u", Direction: "in"},
				{Name: "app_icon", Type: "s", Direction: "in"},
				{Name: "summary", Type: "s", Direction: "in"},
				{Name: "body", Type: "s", Direction: "in"},
				{Name: "actions", Type: "as", Direction: "in"},
				{Name: "hints", Type: "a{sv}", Direction: "in"},
				{Name: "expire_timeout", Type: "i", Direction: "in"},
				{Name: "id", Type: "u", Direction: "out"},
			},
		},
		{
			Name: "CloseNotification",
			Args: []introspect.Arg{
				{Name: "id", Type: "u", Direction: "in"},
			},
		},
	}
}

// notificationSignals returns the D-Bus signal introspection data.
func notificationSignals() []introspect.Signal {
	return []introspect.Signal{
		{
			Name: "NotificationClosed",
			Args: []introspect.Arg{
				{Name: "id", Type: "u"},
				{Name: "reason", Type: "u"},
			},
		},
		{
			Name: "ActionInvoked",
			Args: []introspect.Arg{
				{Name: "id", Type: "u"},
				{Name: "action_key", Type: "s"},
			},
		},
	}
}

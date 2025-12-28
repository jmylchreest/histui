// Package input provides input adapters for notification sources.
package input

import (
	"context"
	"strings"

	"github.com/godbus/dbus/v5"

	"github.com/jmylchreest/histui/internal/model"
)

// InputAdapter fetches notifications from a source.
type InputAdapter interface {
	// Name returns the adapter identifier (e.g., "histuid", "stdin").
	Name() string

	// Import fetches notifications from the source.
	// Returns the notifications and any error encountered.
	Import(ctx context.Context) ([]model.Notification, error)
}

// DetectDaemon returns the name of the notification daemon currently registered
// on D-Bus at org.freedesktop.Notifications.
// Returns empty string if none found or on error.
func DetectDaemon() string {
	// Query D-Bus for the notification server
	serverName := getNotificationServerName()
	if serverName != "" {
		// Normalize known server names
		serverLower := strings.ToLower(serverName)
		switch {
		case strings.Contains(serverLower, "histuid"):
			return "histuid"
		case strings.Contains(serverLower, "mako"):
			return "mako"
		case strings.Contains(serverLower, "swaync"):
			return "swaync"
		}
		// Return as-is for unknown servers
		return serverLower
	}

	return ""
}

// getNotificationServerName queries D-Bus to get the notification server name.
// Returns empty string on error.
func getNotificationServerName() string {
	conn, err := dbus.SessionBus()
	if err != nil {
		return ""
	}

	obj := conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications")
	call := obj.Call("org.freedesktop.Notifications.GetServerInformation", 0)
	if call.Err != nil {
		return ""
	}

	// GetServerInformation returns: name, vendor, version, spec_version
	var name, vendor, version, specVersion string
	if err := call.Store(&name, &vendor, &version, &specVersion); err != nil {
		return ""
	}

	return name
}

// NewAdapter creates an InputAdapter for the specified source.
// If source is empty, attempts to auto-detect.
func NewAdapter(source string) (InputAdapter, error) {
	if source == "" {
		source = DetectDaemon()
	}

	switch source {
	case "histuid", "":
		return NewHistuidAdapter(""), nil
	case "stdin":
		return NewStdinAdapter(), nil
	default:
		return nil, &AdapterError{
			Source:  source,
			Message: "unknown or unavailable adapter",
		}
	}
}

// AdapterError represents an adapter-related error.
type AdapterError struct {
	Source  string
	Message string
	Err     error
}

func (e *AdapterError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *AdapterError) Unwrap() error {
	return e.Err
}

// Package daemon provides the main orchestration for histuid.
package daemon

import (
	"log/slog"
	"sync"
	"time"

	godbus "github.com/godbus/dbus/v5"

	"github.com/jmylchreest/histui/internal/dbus"
)

// NotificationLevel indicates the urgency/severity of an internal notification.
type NotificationLevel int

const (
	// NotificationLevelInfo is for informational messages (low urgency).
	NotificationLevelInfo NotificationLevel = iota
	// NotificationLevelWarning is for warning messages (normal urgency).
	NotificationLevelWarning
	// NotificationLevelError is for error messages (critical urgency).
	NotificationLevelError
)

// InternalNotifier handles sending notifications about internal histuid events.
// It uses a queue and rate limiting to prevent notification floods.
type InternalNotifier struct {
	mu     sync.Mutex
	logger *slog.Logger

	// Handler for creating notifications
	notifyHandler func(notification *dbus.DBusNotification) uint32

	// Rate limiting
	lastNotifyTime map[string]time.Time // key -> last notification time
	minInterval    time.Duration        // minimum time between same notifications

	// Enabled flag
	enabled bool
}

// NewInternalNotifier creates a new InternalNotifier.
func NewInternalNotifier(logger *slog.Logger) *InternalNotifier {
	return &InternalNotifier{
		logger:         logger,
		lastNotifyTime: make(map[string]time.Time),
		minInterval:    5 * time.Second, // Don't repeat same notification within 5 seconds
		enabled:        true,
	}
}

// SetNotifyHandler sets the function to call when creating a notification.
// This should be the same handler used for D-Bus notifications.
func (n *InternalNotifier) SetNotifyHandler(handler func(notification *dbus.DBusNotification) uint32) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.notifyHandler = handler
}

// SetEnabled enables or disables internal notifications.
func (n *InternalNotifier) SetEnabled(enabled bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.enabled = enabled
}

// SetMinInterval sets the minimum interval between duplicate notifications.
func (n *InternalNotifier) SetMinInterval(interval time.Duration) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.minInterval = interval
}

// Notify sends an internal notification if not rate-limited.
// The key is used for rate limiting - same key won't notify again within minInterval.
func (n *InternalNotifier) Notify(key, summary, body string, level NotificationLevel) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.enabled {
		return
	}

	if n.notifyHandler == nil {
		n.logger.Debug("internal notification skipped: no handler", "summary", summary)
		return
	}

	// Rate limiting check
	if lastTime, ok := n.lastNotifyTime[key]; ok {
		if time.Since(lastTime) < n.minInterval {
			n.logger.Debug("internal notification rate-limited", "key", key, "summary", summary)
			return
		}
	}
	n.lastNotifyTime[key] = time.Now()

	// Map level to D-Bus urgency
	urgency := byte(1) // Normal
	switch level {
	case NotificationLevelInfo:
		urgency = 0 // Low
	case NotificationLevelWarning:
		urgency = 1 // Normal
	case NotificationLevelError:
		urgency = 2 // Critical
	}

	// Create the notification
	notification := &dbus.DBusNotification{
		AppName: "histuid",
		Summary: summary,
		Body:    body,
		Hints: map[string]godbus.Variant{
			"urgency":       godbus.MakeVariant(urgency),
			"category":      godbus.MakeVariant("device"),
			"transient":     godbus.MakeVariant(true), // Internal notifications are transient
			"desktop-entry": godbus.MakeVariant("histuid"),
		},
		ExpireTimeout: 5000, // 5 seconds for internal notifications
	}

	// Set icon based on level
	switch level {
	case NotificationLevelInfo:
		notification.AppIcon = "dialog-information"
	case NotificationLevelWarning:
		notification.AppIcon = "dialog-warning"
	case NotificationLevelError:
		notification.AppIcon = "dialog-error"
	}

	n.logger.Debug("sending internal notification", "key", key, "summary", summary, "level", level)

	// Send the notification (this will be handled by the normal notify path)
	_ = n.notifyHandler(notification)
}

// NotifyConfigReloaded sends a notification about config being reloaded.
func (n *InternalNotifier) NotifyConfigReloaded() {
	n.Notify(
		"config-reload",
		"Configuration Reloaded",
		"histuid configuration has been successfully reloaded.",
		NotificationLevelInfo,
	)
}

// NotifyConfigError sends a notification about config validation error.
func (n *InternalNotifier) NotifyConfigError(err error) {
	n.Notify(
		"config-error",
		"Configuration Error",
		"Failed to reload configuration: "+err.Error(),
		NotificationLevelWarning,
	)
}

// NotifyThemeReloaded sends a notification about theme being reloaded.
func (n *InternalNotifier) NotifyThemeReloaded(themeName string) {
	n.Notify(
		"theme-reload",
		"Theme Reloaded",
		"Theme '"+themeName+"' has been reloaded.",
		NotificationLevelInfo,
	)
}

// NotifyThemeError sends a notification about theme loading error.
func (n *InternalNotifier) NotifyThemeError(err error) {
	n.Notify(
		"theme-error",
		"Theme Error",
		"Failed to load theme: "+err.Error(),
		NotificationLevelWarning,
	)
}

// NotifyDnDChanged sends a notification about DnD state change.
func (n *InternalNotifier) NotifyDnDChanged(enabled bool, reason string) {
	var summary, body string
	if enabled {
		summary = "Do Not Disturb Enabled"
		body = "Notifications will be suppressed."
	} else {
		summary = "Do Not Disturb Disabled"
		body = "Notifications will now be displayed."
	}
	if reason != "" {
		body += " (" + reason + ")"
	}
	n.Notify(
		"dnd-change",
		summary,
		body,
		NotificationLevelInfo,
	)
}

// NotifyStartup sends a notification that the daemon has started.
func (n *InternalNotifier) NotifyStartup(version string) {
	n.Notify(
		"startup",
		"histuid Started",
		"Notification daemon v"+version+" is now running.",
		NotificationLevelInfo,
	)
}

// NotifyAudioError sends a notification about audio playback error.
func (n *InternalNotifier) NotifyAudioError(err error) {
	n.Notify(
		"audio-error",
		"Audio Error",
		"Failed to play notification sound: "+err.Error(),
		NotificationLevelWarning,
	)
}

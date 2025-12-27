package dbus

import (
	"github.com/godbus/dbus/v5"

	"github.com/jmylchreest/histui/internal/model"
)

// CloseReason represents the reason for closing a notification.
// These values are defined by the freedesktop.org notification specification.
type CloseReason uint32

const (
	// CloseReasonExpired indicates the notification expired (timeout reached).
	CloseReasonExpired CloseReason = 1
	// CloseReasonDismissed indicates the user dismissed the notification.
	CloseReasonDismissed CloseReason = 2
	// CloseReasonClosed indicates the notification was closed via CloseNotification.
	CloseReasonClosed CloseReason = 3
	// CloseReasonUndefined is reserved/undefined per the spec.
	CloseReasonUndefined CloseReason = 4
)

// String returns the string representation of the close reason.
func (r CloseReason) String() string {
	switch r {
	case CloseReasonExpired:
		return "expired"
	case CloseReasonDismissed:
		return "dismissed"
	case CloseReasonClosed:
		return "closed"
	case CloseReasonUndefined:
		return "undefined"
	default:
		return "unknown"
	}
}

// DBusNotification represents an incoming D-Bus Notify call.
// It contains the raw parameters from the org.freedesktop.Notifications.Notify method.
type DBusNotification struct {
	AppName       string
	ReplacesID    uint32
	AppIcon       string
	Summary       string
	Body          string
	Actions       []string // Alternating key, label pairs
	Hints         map[string]dbus.Variant
	ExpireTimeout int32 // -1 = server default, 0 = never expire
}

// Action represents a notification action with key and label.
type Action struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// ParsedActions converts the D-Bus action array to structured form.
// D-Bus actions are passed as alternating key/label pairs.
func (n *DBusNotification) ParsedActions() []Action {
	actions := make([]Action, 0, len(n.Actions)/2)
	for i := 0; i+1 < len(n.Actions); i += 2 {
		actions = append(actions, Action{
			Key:   n.Actions[i],
			Label: n.Actions[i+1],
		})
	}
	return actions
}

// Urgency extracts the urgency hint from the notification.
// Returns model.UrgencyNormal if not specified.
func (n *DBusNotification) Urgency() int {
	if v, ok := n.Hints["urgency"]; ok {
		if b, ok := v.Value().(byte); ok {
			return int(b)
		}
	}
	return model.UrgencyNormal
}

// Category extracts the category hint from the notification.
// Returns empty string if not specified.
func (n *DBusNotification) Category() string {
	if v, ok := n.Hints["category"]; ok {
		if s, ok := v.Value().(string); ok {
			return s
		}
	}
	return ""
}

// DesktopEntry extracts the desktop-entry hint.
func (n *DBusNotification) DesktopEntry() string {
	if v, ok := n.Hints["desktop-entry"]; ok {
		if s, ok := v.Value().(string); ok {
			return s
		}
	}
	return ""
}

// SoundFile extracts the sound-file hint.
func (n *DBusNotification) SoundFile() string {
	if v, ok := n.Hints["sound-file"]; ok {
		if s, ok := v.Value().(string); ok {
			return s
		}
	}
	return ""
}

// SoundName extracts the sound-name hint.
func (n *DBusNotification) SoundName() string {
	if v, ok := n.Hints["sound-name"]; ok {
		if s, ok := v.Value().(string); ok {
			return s
		}
	}
	return ""
}

// SuppressSound returns true if the suppress-sound hint is set.
func (n *DBusNotification) SuppressSound() bool {
	if v, ok := n.Hints["suppress-sound"]; ok {
		if b, ok := v.Value().(bool); ok {
			return b
		}
	}
	return false
}

// Transient returns true if the transient hint is set.
// Transient notifications should not be persisted.
func (n *DBusNotification) Transient() bool {
	if v, ok := n.Hints["transient"]; ok {
		if b, ok := v.Value().(bool); ok {
			return b
		}
	}
	return false
}

// Resident returns true if the resident hint is set.
// Resident notifications should not be auto-removed after an action is invoked.
func (n *DBusNotification) Resident() bool {
	if v, ok := n.Hints["resident"]; ok {
		if b, ok := v.Value().(bool); ok {
			return b
		}
	}
	return false
}

// ImagePath extracts the image-path hint.
func (n *DBusNotification) ImagePath() string {
	if v, ok := n.Hints["image-path"]; ok {
		if s, ok := v.Value().(string); ok {
			return s
		}
	}
	return ""
}

// ImageData extracts the image-data hint if present.
// The image-data format is: (iiibiiay) - width, height, rowstride, has_alpha, bits_per_sample, channels, data
// Returns nil if not present or invalid.
func (n *DBusNotification) ImageData() []byte {
	if v, ok := n.Hints["image-data"]; ok {
		// image-data is a complex struct, we'll just check if it exists
		// and handle the actual decoding when displaying
		if data, ok := v.Value().([]byte); ok {
			return data
		}
	}
	return nil
}

// Progress extracts the progress value hint.
// Returns -1 if not present, 0-100 for valid progress values.
// This is used by dunstify with the -h int:value:N option.
func (n *DBusNotification) Progress() int {
	if v, ok := n.Hints["value"]; ok {
		switch val := v.Value().(type) {
		case int32:
			return int(val)
		case uint32:
			return int(val)
		case int:
			return val
		case byte:
			return int(val)
		}
	}
	return -1
}

// StackTag extracts the stack-tag hint for notification grouping.
// Notifications with the same stack-tag should replace each other.
// This is used by dunstify with the -h string:x-dunst-stack-tag:TAG option.
func (n *DBusNotification) StackTag() string {
	// Check for dunst-specific stack tag
	if v, ok := n.Hints["x-dunst-stack-tag"]; ok {
		if s, ok := v.Value().(string); ok {
			return s
		}
	}
	// Also check for generic stack tag
	if v, ok := n.Hints["stack-tag"]; ok {
		if s, ok := v.Value().(string); ok {
			return s
		}
	}
	return ""
}

// HighlightColor extracts the highlight color hint (dunstify -h string:hlcolor:#RRGGBB).
func (n *DBusNotification) HighlightColor() string {
	if v, ok := n.Hints["hlcolor"]; ok {
		if s, ok := v.Value().(string); ok {
			return s
		}
	}
	return ""
}

// ForegroundColor extracts the foreground color hint (dunstify -h string:fgcolor:#RRGGBB).
func (n *DBusNotification) ForegroundColor() string {
	if v, ok := n.Hints["fgcolor"]; ok {
		if s, ok := v.Value().(string); ok {
			return s
		}
	}
	return ""
}

// BackgroundColor extracts the background color hint (dunstify -h string:bgcolor:#RRGGBB).
func (n *DBusNotification) BackgroundColor() string {
	if v, ok := n.Hints["bgcolor"]; ok {
		if s, ok := v.Value().(string); ok {
			return s
		}
	}
	return ""
}

// FrameColor extracts the frame/border color hint (dunstify -h string:frcolor:#RRGGBB).
func (n *DBusNotification) FrameColor() string {
	if v, ok := n.Hints["frcolor"]; ok {
		if s, ok := v.Value().(string); ok {
			return s
		}
	}
	return ""
}

// ServerCapabilities lists the capabilities advertised by histuid.
var ServerCapabilities = []string{
	"actions",         // Support notification actions
	"body",            // Support body text
	"body-hyperlinks", // Support hyperlinks in body
	"body-images",     // Support <img> in body
	"body-markup",     // Support Pango markup in body
	"icon-static",     // Support static icons
	"persistence",     // Persist notifications to history
	"sound",           // Play sounds
}

// ServerInfo contains information about the notification server.
type ServerInfo struct {
	Name        string // "histuid"
	Vendor      string // "histui"
	Version     string // Build version
	SpecVersion string // "1.2"
}

// DefaultServerInfo returns the default server information.
func DefaultServerInfo() ServerInfo {
	return ServerInfo{
		Name:        "histuid",
		Vendor:      "histui",
		Version:     "0.0.1", // Will be replaced by build-time version
		SpecVersion: "1.2",
	}
}

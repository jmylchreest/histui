package dbus

import (
	"fmt"

	"github.com/godbus/dbus/v5"

	"github.com/jmylchreest/histui/internal/model"
	"github.com/jmylchreest/histui/internal/store"
)

// ReplayHints are custom hints added to replayed notifications.
const (
	// HintReplay indicates this notification is a replay.
	HintReplay = "x-histui-replay"
	// HintOriginalID is the original histui ID of the notification.
	HintOriginalID = "x-histui-original-id"
	// HintOriginalTimestamp is the original timestamp of the notification.
	HintOriginalTimestamp = "x-histui-original-timestamp"
)

// Replayer sends notifications via D-Bus to the active notification daemon.
// This works standalone - it doesn't require histuid to be running.
type Replayer struct {
	conn       *dbus.Conn
	imageStore *store.ImageStore
}

// NewReplayer creates a new Replayer.
// If imageStore is provided, images will be included in replayed notifications.
func NewReplayer(imageStore *store.ImageStore) (*Replayer, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to session bus: %w", err)
	}

	return &Replayer{
		conn:       conn,
		imageStore: imageStore,
	}, nil
}

// Replay sends a notification to the active notification daemon.
// Returns the D-Bus notification ID assigned by the daemon.
func (r *Replayer) Replay(n *model.Notification) (uint32, error) {
	// Build hints from original hints if available (faithful replay)
	// Otherwise fall back to reconstructing from Extensions (legacy notifications)
	hints := r.buildHints(n)

	// Add replay metadata hints (always added)
	hints[HintReplay] = dbus.MakeVariant(true)
	hints[HintOriginalID] = dbus.MakeVariant(n.HistuiID)
	hints[HintOriginalTimestamp] = dbus.MakeVariant(n.Timestamp)

	// Try to load images from image store
	if r.imageStore != nil && len(n.HistuiImageRefs) > 0 {
		images, err := r.imageStore.LoadAll(n.HistuiID, n.HistuiImageRefs)
		if err == nil {
			// Include as custom hints (PNG format)
			for ref, data := range images {
				if len(data) > 0 {
					hintKey := fmt.Sprintf("x-histui-image-%s-png", ref)
					hints[hintKey] = dbus.MakeVariant(data)
				}
			}
		}
	}

	// Build actions array (alternating key, label)
	var actions []string
	if n.Extensions != nil {
		for _, action := range n.Extensions.Actions {
			actions = append(actions, action.Key, action.Label)
		}
	}

	// Call Notify on the notification daemon
	obj := r.conn.Object(DBusInterface, DBusPath)
	call := obj.Call(
		DBusInterface+".Notify",
		0, // flags
		n.AppName,
		uint32(0), // replaces_id: 0 = new notification
		n.IconPath,
		n.Summary,
		n.Body,
		actions,
		hints,
		int32(-1), // expire_timeout: -1 = server decides
	)

	if call.Err != nil {
		return 0, fmt.Errorf("failed to replay notification: %w", call.Err)
	}

	var id uint32
	if err := call.Store(&id); err != nil {
		return 0, fmt.Errorf("failed to get notification ID: %w", err)
	}

	return id, nil
}

// buildHints creates the hints map for replay.
// Uses OriginalHints if available (faithful replay), otherwise reconstructs from Extensions.
func (r *Replayer) buildHints(n *model.Notification) map[string]dbus.Variant {
	hints := make(map[string]dbus.Variant)

	// If we have original hints, use them directly (faithful replay)
	if len(n.OriginalHints) > 0 {
		for key, value := range n.OriginalHints {
			hints[key] = dbus.MakeVariant(value)
		}
		return hints
	}

	// Fallback: reconstruct hints from Extensions (for older notifications)
	// Add urgency hint
	hints["urgency"] = dbus.MakeVariant(byte(n.Urgency))

	// Add category if present
	if n.Category != "" {
		hints["category"] = dbus.MakeVariant(n.Category)
	}

	// Add extension hints if present
	if n.Extensions != nil {
		if n.Extensions.DesktopEntry != "" {
			hints["desktop-entry"] = dbus.MakeVariant(n.Extensions.DesktopEntry)
		}
		if n.Extensions.SoundFile != "" {
			hints["sound-file"] = dbus.MakeVariant(n.Extensions.SoundFile)
		}
		if n.Extensions.SoundName != "" {
			hints["sound-name"] = dbus.MakeVariant(n.Extensions.SoundName)
		}
		if n.Extensions.StackTag != "" {
			hints["x-dunst-stack-tag"] = dbus.MakeVariant(n.Extensions.StackTag)
		}
		// Only include progress hint if explicitly set (> 0)
		if n.Extensions.Progress > 0 {
			hints["value"] = dbus.MakeVariant(int32(n.Extensions.Progress))
		}
		if n.Extensions.Foreground != "" {
			hints["fgcolor"] = dbus.MakeVariant(n.Extensions.Foreground)
		}
		if n.Extensions.Background != "" {
			hints["bgcolor"] = dbus.MakeVariant(n.Extensions.Background)
		}
		if n.Extensions.Transient {
			hints["transient"] = dbus.MakeVariant(true)
		}
		if n.Extensions.Resident {
			hints["resident"] = dbus.MakeVariant(true)
		}

		// Include inline image data if present in extensions
		if len(n.Extensions.ImageData) > 0 {
			hints["x-histui-image-png"] = dbus.MakeVariant(n.Extensions.ImageData)
		}
	}

	return hints
}

// ReplayWithTimeout sends a notification with a specific timeout.
func (r *Replayer) ReplayWithTimeout(n *model.Notification, timeoutMs int32) (uint32, error) {
	// Build hints from original hints if available
	hints := r.buildHints(n)

	// Add replay metadata hints (always added)
	hints[HintReplay] = dbus.MakeVariant(true)
	hints[HintOriginalID] = dbus.MakeVariant(n.HistuiID)
	hints[HintOriginalTimestamp] = dbus.MakeVariant(n.Timestamp)

	// Build actions array
	var actions []string
	if n.Extensions != nil {
		for _, action := range n.Extensions.Actions {
			actions = append(actions, action.Key, action.Label)
		}
	}

	obj := r.conn.Object(DBusInterface, DBusPath)
	call := obj.Call(
		DBusInterface+".Notify",
		0,
		n.AppName,
		uint32(0),
		n.IconPath,
		n.Summary,
		n.Body,
		actions,
		hints,
		timeoutMs,
	)

	if call.Err != nil {
		return 0, fmt.Errorf("failed to replay notification: %w", call.Err)
	}

	var id uint32
	if err := call.Store(&id); err != nil {
		return 0, fmt.Errorf("failed to get notification ID: %w", err)
	}

	return id, nil
}

// Close releases resources.
func (r *Replayer) Close() error {
	// Don't close the connection as it's the shared session bus
	return nil
}

// IsReplayHint checks if a notification has the replay hint.
func IsReplayHint(hints map[string]dbus.Variant) bool {
	if v, ok := hints[HintReplay]; ok {
		if b, ok := v.Value().(bool); ok {
			return b
		}
	}
	return false
}

// GetOriginalID extracts the original histui ID from replay hints.
func GetOriginalID(hints map[string]dbus.Variant) string {
	if v, ok := hints[HintOriginalID]; ok {
		if s, ok := v.Value().(string); ok {
			return s
		}
	}
	return ""
}

// GetOriginalTimestamp extracts the original timestamp from replay hints.
func GetOriginalTimestamp(hints map[string]dbus.Variant) int64 {
	if v, ok := hints[HintOriginalTimestamp]; ok {
		switch ts := v.Value().(type) {
		case int64:
			return ts
		case int32:
			return int64(ts)
		case int:
			return int64(ts)
		}
	}
	return 0
}

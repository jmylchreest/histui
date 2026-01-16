package dbus

import (
	"bytes"
	"image"
	"image/color"
	"image/png"

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

// ImageDataStruct holds the parsed image-data hint from D-Bus.
// The D-Bus signature is (iiibiiay).
type ImageDataStruct struct {
	Width         int32
	Height        int32
	Rowstride     int32
	HasAlpha      bool
	BitsPerSample int32
	Channels      int32
	Data          []byte
}

// ImageData extracts the image-data hint if present.
// The image-data format is: (iiibiiay) - width, height, rowstride, has_alpha, bits_per_sample, channels, data
// Returns nil if not present or invalid.
func (n *DBusNotification) ImageData() *ImageDataStruct {
	// Try image-data first (preferred), then icon_data (legacy)
	var variant dbus.Variant
	var ok bool
	if variant, ok = n.Hints["image-data"]; !ok {
		if variant, ok = n.Hints["icon_data"]; !ok {
			return nil
		}
	}

	// D-Bus structs come in as []interface{} with fields in order
	fields, ok := variant.Value().([]interface{})
	if !ok || len(fields) != 7 {
		return nil
	}

	// Parse each field with type assertions
	// godbus may send different integer types, so handle both int32 and int
	width := toInt32(fields[0])
	height := toInt32(fields[1])
	rowstride := toInt32(fields[2])
	hasAlpha, ok4 := fields[3].(bool)
	bitsPerSample := toInt32(fields[4])
	channels := toInt32(fields[5])
	data, ok7 := fields[6].([]byte)

	if width == 0 || height == 0 || !ok4 || !ok7 {
		return nil
	}

	return &ImageDataStruct{
		Width:         width,
		Height:        height,
		Rowstride:     rowstride,
		HasAlpha:      hasAlpha,
		BitsPerSample: bitsPerSample,
		Channels:      channels,
		Data:          data,
	}
}

// RawSize returns the size of the raw pixel data in bytes.
func (img *ImageDataStruct) RawSize() int64 {
	if img == nil {
		return 0
	}
	return int64(len(img.Data))
}

// ToPNG converts the raw pixel data to PNG format.
// Returns nil if conversion fails or data is invalid.
func (img *ImageDataStruct) ToPNG() []byte {
	if img == nil || len(img.Data) == 0 {
		return nil
	}

	width := int(img.Width)
	height := int(img.Height)
	rowstride := int(img.Rowstride)
	hasAlpha := img.HasAlpha
	channels := int(img.Channels)

	// Validate dimensions
	if width <= 0 || height <= 0 || width > 4096 || height > 4096 {
		return nil
	}

	// Create image
	goImg := image.NewRGBA(image.Rect(0, 0, width, height))

	// Copy pixel data
	for y := 0; y < height; y++ {
		rowStart := y * rowstride
		for x := 0; x < width; x++ {
			pixelStart := rowStart + x*channels
			if pixelStart+channels > len(img.Data) {
				break
			}

			var r, g, b, a uint8
			if channels >= 3 {
				r = img.Data[pixelStart]
				g = img.Data[pixelStart+1]
				b = img.Data[pixelStart+2]
			}
			if hasAlpha && channels >= 4 {
				a = img.Data[pixelStart+3]
			} else {
				a = 255
			}

			goImg.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: a})
		}
	}

	// Encode to PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, goImg); err != nil {
		return nil
	}

	return buf.Bytes()
}

// toInt32 converts various integer types to int32.
func toInt32(v interface{}) int32 {
	switch val := v.(type) {
	case int32:
		return val
	case int:
		return int32(val)
	case int64:
		return int32(val)
	case uint32:
		return int32(val)
	case uint:
		return int32(val)
	case uint64:
		return int32(val)
	default:
		return 0
	}
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
// Notifications with the same non-empty stack-tag should replace each other.
// Supports dunst-compatible hints:
//   - x-dunst-stack-tag (dunstify -h string:x-dunst-stack-tag:TAG)
//   - stack-tag (generic)
//   - synchronous
//   - private-synchronous
//   - x-canonical-private-synchronous (Ubuntu/Canonical apps)
func (n *DBusNotification) StackTag() string {
	// Check hints in priority order
	hintNames := []string{
		"x-dunst-stack-tag",
		"stack-tag",
		"synchronous",
		"private-synchronous",
		"x-canonical-private-synchronous",
	}
	for _, hint := range hintNames {
		if v, ok := n.Hints[hint]; ok {
			if s, ok := v.Value().(string); ok && s != "" {
				return s
			}
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

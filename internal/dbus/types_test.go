package dbus

import (
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/assert"

	"github.com/jmylchreest/histui/internal/model"
)

func TestCloseReasonString(t *testing.T) {
	tests := []struct {
		reason   CloseReason
		expected string
	}{
		{CloseReasonExpired, "expired"},
		{CloseReasonDismissed, "dismissed"},
		{CloseReasonClosed, "closed"},
		{CloseReasonUndefined, "undefined"},
		{CloseReason(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.reason.String())
		})
	}
}

func TestParsedActions(t *testing.T) {
	tests := []struct {
		name     string
		actions  []string
		expected []Action
	}{
		{
			name:     "empty",
			actions:  nil,
			expected: []Action{},
		},
		{
			name:     "single action",
			actions:  []string{"default", "Open"},
			expected: []Action{{Key: "default", Label: "Open"}},
		},
		{
			name:    "multiple actions",
			actions: []string{"default", "Open", "dismiss", "Dismiss", "reply", "Reply"},
			expected: []Action{
				{Key: "default", Label: "Open"},
				{Key: "dismiss", Label: "Dismiss"},
				{Key: "reply", Label: "Reply"},
			},
		},
		{
			name:     "odd number (incomplete pair ignored)",
			actions:  []string{"default", "Open", "orphan"},
			expected: []Action{{Key: "default", Label: "Open"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &DBusNotification{Actions: tt.actions}
			assert.Equal(t, tt.expected, n.ParsedActions())
		})
	}
}

func TestUrgency(t *testing.T) {
	tests := []struct {
		name     string
		hints    map[string]dbus.Variant
		expected int
	}{
		{
			name:     "no hint",
			hints:    nil,
			expected: model.UrgencyNormal,
		},
		{
			name:     "low urgency",
			hints:    map[string]dbus.Variant{"urgency": dbus.MakeVariant(byte(0))},
			expected: model.UrgencyLow,
		},
		{
			name:     "normal urgency",
			hints:    map[string]dbus.Variant{"urgency": dbus.MakeVariant(byte(1))},
			expected: model.UrgencyNormal,
		},
		{
			name:     "critical urgency",
			hints:    map[string]dbus.Variant{"urgency": dbus.MakeVariant(byte(2))},
			expected: model.UrgencyCritical,
		},
		{
			name:     "wrong type returns normal",
			hints:    map[string]dbus.Variant{"urgency": dbus.MakeVariant("high")},
			expected: model.UrgencyNormal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &DBusNotification{Hints: tt.hints}
			assert.Equal(t, tt.expected, n.Urgency())
		})
	}
}

func TestCategory(t *testing.T) {
	tests := []struct {
		name     string
		hints    map[string]dbus.Variant
		expected string
	}{
		{
			name:     "no hint",
			hints:    nil,
			expected: "",
		},
		{
			name:     "email category",
			hints:    map[string]dbus.Variant{"category": dbus.MakeVariant("email.arrived")},
			expected: "email.arrived",
		},
		{
			name:     "wrong type",
			hints:    map[string]dbus.Variant{"category": dbus.MakeVariant(123)},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &DBusNotification{Hints: tt.hints}
			assert.Equal(t, tt.expected, n.Category())
		})
	}
}

func TestDesktopEntry(t *testing.T) {
	n := &DBusNotification{
		Hints: map[string]dbus.Variant{
			"desktop-entry": dbus.MakeVariant("firefox"),
		},
	}
	assert.Equal(t, "firefox", n.DesktopEntry())

	n.Hints = nil
	assert.Equal(t, "", n.DesktopEntry())
}

func TestSoundFile(t *testing.T) {
	n := &DBusNotification{
		Hints: map[string]dbus.Variant{
			"sound-file": dbus.MakeVariant("/usr/share/sounds/notify.wav"),
		},
	}
	assert.Equal(t, "/usr/share/sounds/notify.wav", n.SoundFile())

	n.Hints = nil
	assert.Equal(t, "", n.SoundFile())
}

func TestSoundName(t *testing.T) {
	n := &DBusNotification{
		Hints: map[string]dbus.Variant{
			"sound-name": dbus.MakeVariant("message-new-instant"),
		},
	}
	assert.Equal(t, "message-new-instant", n.SoundName())

	n.Hints = nil
	assert.Equal(t, "", n.SoundName())
}

func TestSuppressSound(t *testing.T) {
	tests := []struct {
		name     string
		hints    map[string]dbus.Variant
		expected bool
	}{
		{
			name:     "no hint",
			hints:    nil,
			expected: false,
		},
		{
			name:     "suppress true",
			hints:    map[string]dbus.Variant{"suppress-sound": dbus.MakeVariant(true)},
			expected: true,
		},
		{
			name:     "suppress false",
			hints:    map[string]dbus.Variant{"suppress-sound": dbus.MakeVariant(false)},
			expected: false,
		},
		{
			name:     "wrong type",
			hints:    map[string]dbus.Variant{"suppress-sound": dbus.MakeVariant("yes")},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &DBusNotification{Hints: tt.hints}
			assert.Equal(t, tt.expected, n.SuppressSound())
		})
	}
}

func TestTransient(t *testing.T) {
	n := &DBusNotification{
		Hints: map[string]dbus.Variant{
			"transient": dbus.MakeVariant(true),
		},
	}
	assert.True(t, n.Transient())

	n.Hints = map[string]dbus.Variant{
		"transient": dbus.MakeVariant(false),
	}
	assert.False(t, n.Transient())

	n.Hints = nil
	assert.False(t, n.Transient())
}

func TestResident(t *testing.T) {
	n := &DBusNotification{
		Hints: map[string]dbus.Variant{
			"resident": dbus.MakeVariant(true),
		},
	}
	assert.True(t, n.Resident())

	n.Hints = nil
	assert.False(t, n.Resident())
}

func TestImagePath(t *testing.T) {
	n := &DBusNotification{
		Hints: map[string]dbus.Variant{
			"image-path": dbus.MakeVariant("/tmp/image.png"),
		},
	}
	assert.Equal(t, "/tmp/image.png", n.ImagePath())

	n.Hints = nil
	assert.Equal(t, "", n.ImagePath())
}

func TestImageData(t *testing.T) {
	data := []byte{0x89, 0x50, 0x4E, 0x47} // PNG magic
	n := &DBusNotification{
		Hints: map[string]dbus.Variant{
			"image-data": dbus.MakeVariant(data),
		},
	}
	assert.Equal(t, data, n.ImageData())

	n.Hints = nil
	assert.Nil(t, n.ImageData())
}

func TestProgress(t *testing.T) {
	tests := []struct {
		name     string
		hints    map[string]dbus.Variant
		expected int
	}{
		{
			name:     "no hint",
			hints:    nil,
			expected: -1,
		},
		{
			name:     "int32 value",
			hints:    map[string]dbus.Variant{"value": dbus.MakeVariant(int32(50))},
			expected: 50,
		},
		{
			name:     "uint32 value",
			hints:    map[string]dbus.Variant{"value": dbus.MakeVariant(uint32(75))},
			expected: 75,
		},
		{
			name:     "int value",
			hints:    map[string]dbus.Variant{"value": dbus.MakeVariant(100)},
			expected: 100,
		},
		{
			name:     "byte value",
			hints:    map[string]dbus.Variant{"value": dbus.MakeVariant(byte(25))},
			expected: 25,
		},
		{
			name:     "wrong type",
			hints:    map[string]dbus.Variant{"value": dbus.MakeVariant("50%")},
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &DBusNotification{Hints: tt.hints}
			assert.Equal(t, tt.expected, n.Progress())
		})
	}
}

func TestStackTag(t *testing.T) {
	tests := []struct {
		name     string
		hints    map[string]dbus.Variant
		expected string
	}{
		{
			name:     "no hint",
			hints:    nil,
			expected: "",
		},
		{
			name:     "x-dunst-stack-tag",
			hints:    map[string]dbus.Variant{"x-dunst-stack-tag": dbus.MakeVariant("volume")},
			expected: "volume",
		},
		{
			name:     "generic stack-tag",
			hints:    map[string]dbus.Variant{"stack-tag": dbus.MakeVariant("brightness")},
			expected: "brightness",
		},
		{
			name: "x-dunst-stack-tag takes precedence",
			hints: map[string]dbus.Variant{
				"x-dunst-stack-tag": dbus.MakeVariant("dunst"),
				"stack-tag":         dbus.MakeVariant("generic"),
			},
			expected: "dunst",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &DBusNotification{Hints: tt.hints}
			assert.Equal(t, tt.expected, n.StackTag())
		})
	}
}

func TestColorHints(t *testing.T) {
	n := &DBusNotification{
		Hints: map[string]dbus.Variant{
			"hlcolor": dbus.MakeVariant("#FF0000"),
			"fgcolor": dbus.MakeVariant("#FFFFFF"),
			"bgcolor": dbus.MakeVariant("#000000"),
			"frcolor": dbus.MakeVariant("#808080"),
		},
	}

	assert.Equal(t, "#FF0000", n.HighlightColor())
	assert.Equal(t, "#FFFFFF", n.ForegroundColor())
	assert.Equal(t, "#000000", n.BackgroundColor())
	assert.Equal(t, "#808080", n.FrameColor())

	// Empty hints
	n.Hints = nil
	assert.Equal(t, "", n.HighlightColor())
	assert.Equal(t, "", n.ForegroundColor())
	assert.Equal(t, "", n.BackgroundColor())
	assert.Equal(t, "", n.FrameColor())
}

func TestDefaultServerInfo(t *testing.T) {
	info := DefaultServerInfo()
	assert.Equal(t, "histuid", info.Name)
	assert.Equal(t, "histui", info.Vendor)
	assert.Equal(t, "1.2", info.SpecVersion)
	assert.NotEmpty(t, info.Version)
}

func TestServerCapabilities(t *testing.T) {
	assert.Contains(t, ServerCapabilities, "actions")
	assert.Contains(t, ServerCapabilities, "body")
	assert.Contains(t, ServerCapabilities, "body-markup")
	assert.Contains(t, ServerCapabilities, "persistence")
	assert.Contains(t, ServerCapabilities, "sound")
}

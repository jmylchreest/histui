package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// Duration is a time.Duration that can be unmarshaled from human-readable strings.
// Supports formats like "5s", "10s", "1m", "1h30m", or integer milliseconds for backwards compatibility.
// A value of "0" or 0 means never expire.
type Duration time.Duration

// UnmarshalText implements encoding.TextUnmarshaler for TOML parsing.
func (d *Duration) UnmarshalText(text []byte) error {
	s := string(text)

	// Try parsing as integer (milliseconds) for backwards compatibility
	if ms, err := strconv.ParseInt(s, 10, 64); err == nil {
		*d = Duration(time.Duration(ms) * time.Millisecond)
		return nil
	}

	// Parse as duration string (e.g., "5s", "1m", "1h30m")
	dur, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: must be like '5s', '1m', '1h30m' or milliseconds: %w", s, err)
	}
	*d = Duration(dur)
	return nil
}

// MarshalText implements encoding.TextMarshaler for TOML output.
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(time.Duration(d).String()), nil
}

// Milliseconds returns the duration in milliseconds.
func (d Duration) Milliseconds() int {
	return int(time.Duration(d).Milliseconds())
}

// Duration returns the underlying time.Duration.
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// DaemonConfig is the configuration for histuid.
// Loaded from ~/.config/histui/histuid.toml
type DaemonConfig struct {
	Display  DisplayConfig  `toml:"display"`
	Timeouts TimeoutConfig  `toml:"timeouts"`
	Behavior BehaviorConfig `toml:"behavior"`
	Audio    AudioConfig    `toml:"audio"`
	Theme    ThemeConfig    `toml:"theme"`
	Layout   LayoutConfig   `toml:"layout"`
	DnD      DnDConfig      `toml:"dnd"`
	Mouse    MouseConfig    `toml:"mouse"`
}

// LayoutConfig contains layout template settings.
type LayoutConfig struct {
	Template string `toml:"template"` // Template name without .xml extension
}

// DisplayConfig contains display-related settings.
type DisplayConfig struct {
	Position   string  `toml:"position"`    // "top-right", "top-left", etc.
	OffsetX    int     `toml:"offset_x"`    // Pixels from screen edge
	OffsetY    int     `toml:"offset_y"`    // Pixels from screen edge
	Width      int     `toml:"width"`       // Popup width in pixels
	MaxHeight  int     `toml:"max_height"`  // Maximum popup height
	MaxVisible int     `toml:"max_visible"` // Maximum simultaneous popups
	Gap        int     `toml:"gap"`         // Gap between stacked popups
	Monitor    int     `toml:"monitor"`     // 0 = all, 1+ = specific monitor
	Opacity    float64 `toml:"opacity"`     // 0.0-1.0, background opacity for blur effects
}

// TimeoutConfig contains timeout settings per urgency level.
// Durations can be specified as "5s", "10s", "1m", etc. or as integer milliseconds.
// A value of "0" or 0 means never expire.
type TimeoutConfig struct {
	Low      Duration `toml:"low"`      // e.g., "5s", "1m", or 5000
	Normal   Duration `toml:"normal"`   // e.g., "10s", "1m", or 10000
	Critical Duration `toml:"critical"` // e.g., "0" for never expire
}

// BehaviorConfig contains behavior settings.
type BehaviorConfig struct {
	StackDuplicates bool `toml:"stack_duplicates"` // Combine identical notifications
	ShowCount       bool `toml:"show_count"`       // Show "(2)" for stacked duplicates
	PauseOnHover    bool `toml:"pause_on_hover"`   // Pause timeout when mouse hovers
	HistoryLength   int  `toml:"history_length"`   // Max notifications in session memory
}

// AudioConfig contains audio settings.
type AudioConfig struct {
	Enabled bool        `toml:"enabled"`
	Volume  int         `toml:"volume"` // 0-100
	Sounds  SoundConfig `toml:"sounds"`
}

// SoundConfig contains per-urgency sound file paths.
type SoundConfig struct {
	Low      string `toml:"low"`
	Normal   string `toml:"normal"`
	Critical string `toml:"critical"`
}

// ThemeConfig contains theme settings.
type ThemeConfig struct {
	Name        string `toml:"name"`         // Theme name without .css extension
	ColorScheme string `toml:"color_scheme"` // "system", "light", or "dark"
}

// ColorScheme represents the color scheme preference.
type ColorScheme string

const (
	ColorSchemeSystem ColorScheme = "system"
	ColorSchemeLight  ColorScheme = "light"
	ColorSchemeDark   ColorScheme = "dark"
)

// ValidColorSchemes returns all valid color scheme values.
func ValidColorSchemes() []ColorScheme {
	return []ColorScheme{ColorSchemeSystem, ColorSchemeLight, ColorSchemeDark}
}

// DnDConfig contains Do Not Disturb settings.
type DnDConfig struct {
	Enabled        bool `toml:"enabled"`         // Initial state
	CriticalBypass bool `toml:"critical_bypass"` // Show critical even in DnD mode
}

// MouseConfig contains mouse button action mappings.
type MouseConfig struct {
	Left   string `toml:"left"`   // "dismiss", "do-action", "close-all", "context-menu", "none"
	Middle string `toml:"middle"` // "dismiss", "do-action", "close-all", "context-menu", "none"
	Right  string `toml:"right"`  // "dismiss", "do-action", "close-all", "context-menu", "none"
}

// MouseAction represents a mouse button action.
type MouseAction string

const (
	MouseActionDismiss     MouseAction = "dismiss"
	MouseActionDoAction    MouseAction = "do-action"
	MouseActionCloseAll    MouseAction = "close-all"
	MouseActionContextMenu MouseAction = "context-menu"
	MouseActionNone        MouseAction = "none"
)

// Position represents a popup position on screen.
type Position string

const (
	PositionTopLeft      Position = "top-left"
	PositionTopRight     Position = "top-right"
	PositionTopCenter    Position = "top-center"
	PositionBottomLeft   Position = "bottom-left"
	PositionBottomRight  Position = "bottom-right"
	PositionBottomCenter Position = "bottom-center"
)

// ValidPositions returns all valid position values.
func ValidPositions() []Position {
	return []Position{
		PositionTopLeft,
		PositionTopRight,
		PositionTopCenter,
		PositionBottomLeft,
		PositionBottomRight,
		PositionBottomCenter,
	}
}

// DefaultDaemonConfig returns a new DaemonConfig with default values.
func DefaultDaemonConfig() *DaemonConfig {
	return &DaemonConfig{
		Display: DisplayConfig{
			Position:   string(PositionTopRight),
			OffsetX:    10,
			OffsetY:    10,
			Width:      350,
			MaxHeight:  200,
			MaxVisible: 5,
			Gap:        5,
			Monitor:    0,
			Opacity:    1.0, // Fully opaque by default
		},
		Timeouts: TimeoutConfig{
			Low:      Duration(5 * time.Second),
			Normal:   Duration(10 * time.Second),
			Critical: Duration(0), // Never expires
		},
		Behavior: BehaviorConfig{
			StackDuplicates: true,
			ShowCount:       true,
			PauseOnHover:    true,
			HistoryLength:   100,
		},
		Audio: AudioConfig{
			Enabled: true,
			Volume:  80,
			Sounds:  SoundConfig{},
		},
		Theme: ThemeConfig{
			Name:        "default",
			ColorScheme: string(ColorSchemeSystem),
		},
		Layout: LayoutConfig{
			Template: "default",
		},
		DnD: DnDConfig{
			Enabled:        false,
			CriticalBypass: true,
		},
		Mouse: MouseConfig{
			Left:   string(MouseActionDismiss),
			Middle: string(MouseActionDoAction),
			Right:  string(MouseActionCloseAll),
		},
	}
}

// DaemonConfigPath returns the path to the daemon config file.
func DaemonConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "histui", "histuid.toml"), nil
}

// LoadDaemonConfig loads the daemon configuration from disk.
// If the file doesn't exist, returns the default configuration.
func LoadDaemonConfig() (*DaemonConfig, error) {
	path, err := DaemonConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get config path: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultDaemonConfig(), nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Start with defaults, then overlay with file contents
	config := DefaultDaemonConfig()
	if err := toml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate the configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// SaveDaemonConfig saves the daemon configuration to disk.
func SaveDaemonConfig(config *DaemonConfig) error {
	path, err := DaemonConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := toml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write atomically via temp file
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return os.Rename(tmpPath, path)
}

// Validate checks if the configuration is valid.
func (c *DaemonConfig) Validate() error {
	// Validate position
	validPos := false
	for _, p := range ValidPositions() {
		if c.Display.Position == string(p) {
			validPos = true
			break
		}
	}
	if !validPos {
		return fmt.Errorf("invalid position %q, must be one of: %v", c.Display.Position, ValidPositions())
	}

	// Validate dimensions
	if c.Display.Width < 100 || c.Display.Width > 1000 {
		return fmt.Errorf("width must be between 100 and 1000, got %d", c.Display.Width)
	}
	if c.Display.MaxVisible < 1 || c.Display.MaxVisible > 20 {
		return fmt.Errorf("max_visible must be between 1 and 20, got %d", c.Display.MaxVisible)
	}

	// Validate volume
	if c.Audio.Volume < 0 || c.Audio.Volume > 100 {
		return fmt.Errorf("volume must be between 0 and 100, got %d", c.Audio.Volume)
	}

	// Validate mouse actions
	validActions := map[string]bool{
		string(MouseActionDismiss):     true,
		string(MouseActionDoAction):    true,
		string(MouseActionCloseAll):    true,
		string(MouseActionContextMenu): true,
		string(MouseActionNone):        true,
	}
	for _, action := range []string{c.Mouse.Left, c.Mouse.Middle, c.Mouse.Right} {
		if !validActions[action] {
			return fmt.Errorf("invalid mouse action %q", action)
		}
	}

	return nil
}

// GetTimeoutForUrgency returns the timeout in milliseconds for the given urgency level.
func (c *DaemonConfig) GetTimeoutForUrgency(urgency int) int {
	switch urgency {
	case 0: // Low
		return c.Timeouts.Low.Milliseconds()
	case 2: // Critical
		return c.Timeouts.Critical.Milliseconds()
	default: // Normal (1) or unknown
		return c.Timeouts.Normal.Milliseconds()
	}
}

// GetSoundForUrgency returns the sound file path for the given urgency level.
// Expands ~ to home directory.
func (c *DaemonConfig) GetSoundForUrgency(urgency int) string {
	var path string
	switch urgency {
	case 0: // Low
		path = c.Audio.Sounds.Low
	case 2: // Critical
		path = c.Audio.Sounds.Critical
	default: // Normal (1) or unknown
		path = c.Audio.Sounds.Normal
	}
	return expandPath(path)
}

// expandPath expands ~ to the user's home directory.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/jmylchreest/histui/internal/types"
)

// ParseLogLevel converts a log level string to slog.Level.
// Supported values: debug, info, warn, error (case-insensitive).
// Returns slog.LevelInfo for unrecognized values.
func ParseLogLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Type aliases for shared types from internal/types package.
// See internal/types/types.go for documentation.
type (
	Duration = types.Duration
	ByteSize = types.ByteSize
)

// Byte size constants (re-exported from types package)
const (
	// IEC binary units (base 1024)
	KiB = types.KiB
	MiB = types.MiB
	GiB = types.GiB
	TiB = types.TiB
	// SI decimal units (base 1000)
	KB = types.KB
	MB = types.MB
	GB = types.GB
	TB = types.TB
)

// DaemonConfig is the configuration for histuid.
// Loaded from ~/.config/histui/histuid.toml
// Can be overridden via environment variables (HISTUID_*) or command-line flags.
type DaemonConfig struct {
	LogLevel string         `toml:"log_level" mapstructure:"log_level"` // debug, info, warn, error
	Display  DisplayConfig  `toml:"display" mapstructure:"display"`
	Timeouts TimeoutConfig  `toml:"timeouts" mapstructure:"timeouts"`
	Behavior BehaviorConfig `toml:"behavior" mapstructure:"behavior"`
	Audio    AudioConfig    `toml:"audio" mapstructure:"audio"`
	Theme    ThemeConfig    `toml:"theme" mapstructure:"theme"`
	DnD      DnDConfig      `toml:"dnd" mapstructure:"dnd"`
	Mouse    MouseConfig    `toml:"mouse" mapstructure:"mouse"`
	History  HistoryConfig  `toml:"history" mapstructure:"history"`
}

// HistoryConfig contains history retention settings.
type HistoryConfig struct {
	MaxNotifications int  `toml:"max_notifications" mapstructure:"max_notifications"` // 0 = unlimited
	StoreImages      bool `toml:"store_images" mapstructure:"store_images"`           // Store notification images
}

// DisplayConfig contains display-related settings.
// Note: Sizing (width, height) is controlled by layout templates.
// Note: Opacity/translucency is controlled by CSS themes.
type DisplayConfig struct {
	Position             string   `toml:"position" mapstructure:"position"`                               // "top-right", "top-left", etc.
	OffsetX              int      `toml:"offset_x" mapstructure:"offset_x"`                               // Pixels from screen edge
	OffsetY              int      `toml:"offset_y" mapstructure:"offset_y"`                               // Pixels from screen edge
	MaxVisible           int      `toml:"max_visible" mapstructure:"max_visible"`                         // Maximum simultaneous popups
	Monitor              int      `toml:"monitor" mapstructure:"monitor"`                                 // 0 = all, 1+ = specific monitor
	NewOnTop             bool     `toml:"new_on_top" mapstructure:"new_on_top"`                           // If true, new notifications appear at top of stack
	ImageDataPreviewSize ByteSize `toml:"image_data_preview_size" mapstructure:"image_data_preview_size"` // Control image-data display: -1/never, 0/always, or min size like "100 KiB"
}

// TimeoutConfig contains timeout settings per urgency level.
// Durations can be specified as "5s", "10s", "1m", etc. or as integer milliseconds.
//
// Special values:
//   - "0" or "0s" = honor the client's requested timeout (from the notification)
//   - "-1", "-1s", or "never" = notification never expires
//   - positive value (e.g., "10s") = override with this timeout regardless of client request
type TimeoutConfig struct {
	Low      Duration `toml:"low" mapstructure:"low"`           // e.g., "5s", "0" (honor client), or "never"
	Normal   Duration `toml:"normal" mapstructure:"normal"`     // e.g., "10s", "0" (honor client), or "never"
	Critical Duration `toml:"critical" mapstructure:"critical"` // e.g., "never" (default), "0" (honor client), or "30s"
	Fallback Duration `toml:"fallback" mapstructure:"fallback"` // Used when honoring client but client says "server decides" (-1)
}

// BehaviorConfig contains behavior settings.
type BehaviorConfig struct {
	StackDuplicates bool `toml:"stack_duplicates" mapstructure:"stack_duplicates"` // Combine identical notifications
	ShowCount       bool `toml:"show_count" mapstructure:"show_count"`             // Show "(2)" for stacked duplicates
	PauseOnHover    bool `toml:"pause_on_hover" mapstructure:"pause_on_hover"`     // Pause timeout when mouse hovers
	HistoryLength   int  `toml:"history_length" mapstructure:"history_length"`     // Max notifications in session memory
}

// AudioConfig contains audio settings.
type AudioConfig struct {
	Enabled bool        `toml:"enabled" mapstructure:"enabled"`
	Volume  int         `toml:"volume" mapstructure:"volume"` // 0-100
	Sounds  SoundConfig `toml:"sounds" mapstructure:"sounds"`
}

// SoundConfig contains per-urgency sound file paths.
type SoundConfig struct {
	Low      string `toml:"low" mapstructure:"low"`
	Normal   string `toml:"normal" mapstructure:"normal"`
	Critical string `toml:"critical" mapstructure:"critical"`
}

// ThemeConfig contains theme settings.
type ThemeConfig struct {
	Name        string `toml:"name" mapstructure:"name"`                 // Theme name without .css extension
	ColorScheme string `toml:"color_scheme" mapstructure:"color_scheme"` // "system", "light", or "dark"
	FontFamily  string `toml:"font_family" mapstructure:"font_family"`   // Font family (empty = inherit from system)
	FontSize    int    `toml:"font_size" mapstructure:"font_size"`       // Base font size in pixels (0 = use theme default)
}

// ColorScheme represents the color scheme preference.
type ColorScheme string

const (
	ColorSchemeSystem ColorScheme = "system"
	ColorSchemeLight  ColorScheme = "light"
	ColorSchemeDark   ColorScheme = "dark"
)

// DnDConfig contains Do Not Disturb settings.
type DnDConfig struct {
	Enabled          bool `toml:"enabled" mapstructure:"enabled"`                     // Initial state
	SuppressCritical bool `toml:"suppress_critical" mapstructure:"suppress_critical"` // Also suppress critical notifications in DnD mode
}

// MouseConfig contains mouse button action mappings.
type MouseConfig struct {
	Left   string `toml:"left" mapstructure:"left"`     // "dismiss", "do-action", "close-all", "context-menu", "none"
	Middle string `toml:"middle" mapstructure:"middle"` // "dismiss", "do-action", "close-all", "context-menu", "none"
	Right  string `toml:"right" mapstructure:"right"`   // "dismiss", "do-action", "close-all", "context-menu", "none"
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

// setDefaults configures Viper with default values.
func setDefaults(v *viper.Viper) {
	// Display defaults
	v.SetDefault("display.position", string(PositionTopRight))
	v.SetDefault("display.offset_x", 10)
	v.SetDefault("display.offset_y", 10)
	v.SetDefault("display.max_visible", 5)
	v.SetDefault("display.monitor", 0)
	v.SetDefault("display.new_on_top", false)
	v.SetDefault("display.image_data_preview_size", "100 KiB") // Filter profile pics (<128x128), show album art

	// Timeout defaults (as strings for Duration parsing)
	// "0" = honor client, "-1" or "never" = never expire, positive = override
	v.SetDefault("timeouts.low", "0")
	v.SetDefault("timeouts.normal", "0")
	v.SetDefault("timeouts.critical", "never")
	v.SetDefault("timeouts.fallback", "10s") // Used when client says "server decides"

	// Behavior defaults
	v.SetDefault("behavior.stack_duplicates", true)
	v.SetDefault("behavior.show_count", true)
	v.SetDefault("behavior.pause_on_hover", true)
	v.SetDefault("behavior.history_length", 100)

	// Audio defaults
	v.SetDefault("audio.enabled", true)
	v.SetDefault("audio.volume", 80)
	v.SetDefault("audio.sounds.low", "")
	v.SetDefault("audio.sounds.normal", "")
	v.SetDefault("audio.sounds.critical", "")

	// Theme defaults
	// Note: Layout is loaded from theme directory (themes/{name}/layout.xml)
	v.SetDefault("theme.name", "default")
	v.SetDefault("theme.color_scheme", string(ColorSchemeSystem))
	v.SetDefault("theme.font_family", "")
	v.SetDefault("theme.font_size", 0)

	// DnD defaults
	v.SetDefault("dnd.enabled", false)
	v.SetDefault("dnd.suppress_critical", false)

	// Mouse defaults
	v.SetDefault("mouse.left", string(MouseActionDismiss))
	v.SetDefault("mouse.middle", string(MouseActionDoAction))
	v.SetDefault("mouse.right", string(MouseActionCloseAll))

	// History defaults
	v.SetDefault("history.max_notifications", 500) // 0 = unlimited
	v.SetDefault("history.store_images", true)

	// Logging defaults
	v.SetDefault("log_level", "info") // debug, info, warn, error
}

// stringToDurationHookFunc returns a mapstructure decode hook for Duration.
func stringToDurationHookFunc() mapstructure.DecodeHookFunc {
	return func(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
		if t != reflect.TypeOf(Duration(0)) {
			return data, nil
		}

		switch v := data.(type) {
		case string:
			// Handle "never" as alias for -1 (never expire)
			if strings.EqualFold(v, "never") {
				return Duration(-1 * time.Millisecond), nil
			}
			// Try parsing as duration string
			if dur, err := time.ParseDuration(v); err == nil {
				return Duration(dur), nil
			}
			// Try parsing as integer milliseconds
			var ms int64
			if _, err := fmt.Sscanf(v, "%d", &ms); err == nil {
				return Duration(time.Duration(ms) * time.Millisecond), nil
			}
			return nil, fmt.Errorf("invalid duration %q", v)
		case int, int64:
			// Integer milliseconds
			var ms int64
			switch val := v.(type) {
			case int:
				ms = int64(val)
			case int64:
				ms = val
			}
			return Duration(time.Duration(ms) * time.Millisecond), nil
		case float64:
			// Float milliseconds (from JSON/TOML number)
			return Duration(time.Duration(int64(v)) * time.Millisecond), nil
		default:
			return data, nil
		}
	}
}

// stringToByteSizeHookFunc returns a mapstructure decode hook for ByteSize.
func stringToByteSizeHookFunc() mapstructure.DecodeHookFunc {
	return func(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
		if t != reflect.TypeOf(ByteSize(0)) {
			return data, nil
		}

		switch v := data.(type) {
		case string:
			var b ByteSize
			if err := b.UnmarshalText([]byte(v)); err != nil {
				return nil, err
			}
			return b, nil
		case int, int64:
			// Integer bytes
			var bytes int64
			switch val := v.(type) {
			case int:
				bytes = int64(val)
			case int64:
				bytes = val
			}
			return ByteSize(bytes), nil
		case float64:
			// Float bytes (from JSON/TOML number)
			return ByteSize(int64(v)), nil
		default:
			return data, nil
		}
	}
}

// DefaultDaemonConfig returns a new DaemonConfig with default values.
func DefaultDaemonConfig() *DaemonConfig {
	return &DaemonConfig{
		Display: DisplayConfig{
			Position:             string(PositionTopRight),
			OffsetX:              10,
			OffsetY:              10,
			MaxVisible:           5,
			Monitor:              0,
			ImageDataPreviewSize: 100 * KiB, // filters profile pics, shows album art
		},
		Timeouts: TimeoutConfig{
			Low:      Duration(0),                     // Honor client
			Normal:   Duration(0),                     // Honor client
			Critical: Duration(-1 * time.Millisecond), // Never expires (-1)
			Fallback: Duration(10 * time.Second),      // Used when client says "server decides"
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
		DnD: DnDConfig{
			Enabled:          false,
			SuppressCritical: false,
		},
		Mouse: MouseConfig{
			Left:   string(MouseActionDismiss),
			Middle: string(MouseActionDoAction),
			Right:  string(MouseActionCloseAll),
		},
		History: HistoryConfig{
			MaxNotifications: 500,
			StoreImages:      true,
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

// DaemonConfigDir returns the directory containing the daemon config file.
func DaemonConfigDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "histui"), nil
}

// NewViper creates and configures a new Viper instance for histuid configuration.
// This sets up:
// - Default values
// - Config file location (~/.config/histui/histuid.toml)
// - Environment variable binding with HISTUID_ prefix
//
// Call BindPFlags() to bind command-line flags before loading.
func NewViper() (*viper.Viper, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Configure config file
	configDir, err := DaemonConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get config directory: %w", err)
	}

	v.SetConfigName("histuid")
	v.SetConfigType("toml")
	v.AddConfigPath(configDir)

	// Configure environment variables
	// HISTUID_DISPLAY_POSITION -> display.position
	v.SetEnvPrefix("HISTUID")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	return v, nil
}

// BindPFlags binds command-line flags to Viper configuration keys.
// Call this after defining pflags but before ReadInConfig.
func BindPFlags(v *viper.Viper, flags *pflag.FlagSet) error {
	// Bind specific flags to config keys
	bindings := map[string]string{
		"position":        "display.position",
		"offset-x":        "display.offset_x",
		"offset-y":        "display.offset_y",
		"max-visible":     "display.max_visible",
		"display-monitor": "display.monitor",
		"new-on-top":      "display.new_on_top",
		"theme":           "theme.name",
		"font":            "theme.font_family",
		"font-size":       "theme.font_size",
	}

	for flagName, configKey := range bindings {
		if flag := flags.Lookup(flagName); flag != nil {
			if err := v.BindPFlag(configKey, flag); err != nil {
				return fmt.Errorf("failed to bind flag %s: %w", flagName, err)
			}
		}
	}

	return nil
}

// LoadDaemonConfigWithViper loads daemon configuration using the provided Viper instance.
// This allows callers to set up pflags before loading.
func LoadDaemonConfigWithViper(v *viper.Viper) (*DaemonConfig, error) {
	// Read config file (ignore "not found" errors - defaults will be used)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Unmarshal with custom decode hooks for Duration and ByteSize
	var cfg DaemonConfig
	decoderConfig := func(dc *mapstructure.DecoderConfig) {
		dc.DecodeHook = mapstructure.ComposeDecodeHookFunc(
			stringToDurationHookFunc(),
			stringToByteSizeHookFunc(),
			mapstructure.StringToTimeDurationHookFunc(),
		)
		dc.TagName = "mapstructure"
	}

	if err := v.Unmarshal(&cfg, decoderConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate the configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// LoadDaemonConfig loads the daemon configuration from disk.
// If the file doesn't exist, returns the default configuration.
// This is a convenience wrapper that doesn't support pflags.
// For pflag support, use NewViper() + BindPFlags() + LoadDaemonConfigWithViper().
func LoadDaemonConfig() (*DaemonConfig, error) {
	v, err := NewViper()
	if err != nil {
		return nil, err
	}
	return LoadDaemonConfigWithViper(v)
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

	// Validate max_visible
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
// The clientTimeout parameter is the timeout requested by the notification client (in ms):
//   - clientTimeout = -1: server should decide
//   - clientTimeout = 0: never expire
//   - clientTimeout > 0: client's requested timeout in ms
//
// Config values are interpreted as:
//   - 0: honor client's request (if client = -1, use fallback defaults)
//   - -1 (or "never"): never expire
//   - positive: override with this value
//
// Returns timeout in milliseconds. 0 means never expire.
func (c *DaemonConfig) GetTimeoutForUrgency(urgency int, clientTimeout int32) int {
	var configDuration Duration
	switch urgency {
	case 0: // Low
		configDuration = c.Timeouts.Low
	case 2: // Critical
		configDuration = c.Timeouts.Critical
	default: // Normal (1) or unknown
		configDuration = c.Timeouts.Normal
	}

	configMs := configDuration.Milliseconds()

	// Config < 0 means never expire
	if configMs < 0 {
		return 0
	}

	// Config = 0 means honor client's request
	if configMs == 0 {
		// Client requested server to decide
		if clientTimeout == -1 {
			// Use configurable fallback (critical always uses 0 = never expire)
			if urgency == 2 {
				return 0 // Critical never expires by default
			}
			fallbackMs := c.Timeouts.Fallback.Milliseconds()
			if fallbackMs <= 0 {
				return 10000 // Default fallback if not configured
			}
			return fallbackMs
		}
		// Client requested never expire
		if clientTimeout == 0 {
			return 0
		}
		// Use client's requested timeout
		return int(clientTimeout)
	}

	// Config > 0 means override with config value
	return configMs
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

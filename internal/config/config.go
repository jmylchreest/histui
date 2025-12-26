// Package config handles configuration file loading and parsing.
package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// Default configuration values.
const (
	DefaultSince      = "48h"
	DefaultSortField  = "timestamp"
	DefaultSortOrder  = "desc"
	DefaultOlderThan  = "48h"
	DefaultIconSize   = 64
	DefaultDmenuTmpl  = "{{.AppName}} | {{.Summary}} - {{.BodyTruncated 50}} | {{.RelativeTime}}"
	DefaultFullTmpl   = "{{.Timestamp | formatTime}} {{.AppName}}: {{.Summary}}\n{{.Body}}"
	DefaultBodyTmpl   = "{{.Body}}"
	DefaultTUIOutput  = "{{.Timestamp | formatTime}} {{.AppName}}: {{.Summary}}\n{{.Body}}"
)

// Config represents the histui configuration.
type Config struct {
	Filter    FilterConfig    `toml:"filter"`
	Sort      SortConfig      `toml:"sort"`
	Prune     PruneConfig     `toml:"prune"`
	Templates TemplatesConfig `toml:"templates"`
	TUI       TUIConfig       `toml:"tui"`
	Clipboard ClipboardConfig `toml:"clipboard"`
}

// FilterConfig holds default filtering options.
type FilterConfig struct {
	Since string `toml:"since"` // Default time filter (0 = all time)
	Limit int    `toml:"limit"` // Max notifications (0 = unlimited)
}

// SortConfig holds default sorting options.
type SortConfig struct {
	Field string `toml:"field"` // timestamp, app, urgency
	Order string `toml:"order"` // asc, desc
}

// PruneConfig holds default prune options.
type PruneConfig struct {
	OlderThan string `toml:"older_than"` // Default age threshold
	Keep      int    `toml:"keep"`       // Max to keep (0 = unlimited)
}

// TemplatesConfig holds output templates.
type TemplatesConfig struct {
	Dmenu     string            `toml:"dmenu"`
	Full      string            `toml:"full"`
	Body      string            `toml:"body"`
	JSON      string            `toml:"json"` // Empty = use built-in marshaling
	TUIOutput string            `toml:"tui_output"`
	Custom    map[string]string `toml:"custom"`
}

// TUIConfig holds TUI-specific settings.
type TUIConfig struct {
	ShowIcons bool `toml:"show_icons"`
	IconSize  int  `toml:"icon_size"`
	ShowHelp  bool `toml:"show_help"`
}

// ClipboardConfig holds clipboard settings (TUI only).
type ClipboardConfig struct {
	Command string `toml:"command"` // Auto-detected if empty
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() *Config {
	return &Config{
		Filter: FilterConfig{
			Since: DefaultSince,
			Limit: 0,
		},
		Sort: SortConfig{
			Field: DefaultSortField,
			Order: DefaultSortOrder,
		},
		Prune: PruneConfig{
			OlderThan: DefaultOlderThan,
			Keep:      0,
		},
		Templates: TemplatesConfig{
			Dmenu:     DefaultDmenuTmpl,
			Full:      DefaultFullTmpl,
			Body:      DefaultBodyTmpl,
			JSON:      "",
			TUIOutput: DefaultTUIOutput,
			Custom:    make(map[string]string),
		},
		TUI: TUIConfig{
			ShowIcons: true,
			IconSize:  DefaultIconSize,
			ShowHelp:  true,
		},
		Clipboard: ClipboardConfig{
			Command: "", // Auto-detect
		},
	}
}

// ConfigPath returns the path to the config file.
// Uses XDG_CONFIG_HOME if set, otherwise ~/.config.
func ConfigPath() string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "histui", "config.toml")
}

// DataPath returns the path to the data directory.
// Uses XDG_DATA_HOME if set, otherwise ~/.local/share.
func DataPath() string {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, "histui")
}

// HistoryPath returns the path to the history JSONL file.
func HistoryPath() string {
	return filepath.Join(DataPath(), "history.jsonl")
}

// TombstonePath returns the path to the tombstones file.
func TombstonePath() string {
	return filepath.Join(DataPath(), "tombstones.json")
}

// LoadConfig loads configuration from the specified path.
// If path is empty, uses the default config path.
// Returns default config if file doesn't exist.
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		path = ConfigPath()
	}

	// Start with defaults
	cfg := DefaultConfig()

	// Check if file exists
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// No config file, use defaults
			return cfg, nil
		}
		return nil, err
	}

	// Parse TOML
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Save writes the configuration to the specified path.
// Creates parent directories if needed.
func (c *Config) Save(path string) error {
	if path == "" {
		path = ConfigPath()
	}

	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := toml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// GetTemplate returns the template for the given name.
// First checks custom templates, then built-in ones.
// Returns empty string if not found.
func (c *Config) GetTemplate(name string) string {
	// Check custom templates first
	if tmpl, ok := c.Templates.Custom[name]; ok {
		return tmpl
	}

	// Check built-in templates
	switch name {
	case "dmenu":
		return c.Templates.Dmenu
	case "full":
		return c.Templates.Full
	case "body":
		return c.Templates.Body
	case "json":
		return c.Templates.JSON
	case "tui_output":
		return c.Templates.TUIOutput
	default:
		return ""
	}
}

// EnsureDataDir creates the data directory if it doesn't exist.
func EnsureDataDir() error {
	path := DataPath()
	if path == "" {
		return errors.New("unable to determine data directory")
	}
	return os.MkdirAll(path, 0755)
}

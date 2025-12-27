// Package theme provides CSS theming for notification popups.
package theme

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

// Manifest represents a theme's configuration and assets.
// Theme directories can contain a manifest file (manifest.toml, manifest.yaml, or manifest.json)
// to define audio, icon settings, and other theme-specific configuration.
type Manifest struct {
	// Theme metadata
	Name        string `toml:"name" yaml:"name" json:"name"`
	Description string `toml:"description" yaml:"description" json:"description"`
	Author      string `toml:"author" yaml:"author" json:"author"`
	Version     string `toml:"version" yaml:"version" json:"version"`

	// Audio configuration per urgency level
	Audio AudioManifest `toml:"audio" yaml:"audio" json:"audio"`

	// Icon configuration
	Icon IconManifest `toml:"icon" yaml:"icon" json:"icon"`
}

// AudioManifest defines sound configuration for each urgency level.
type AudioManifest struct {
	Low      SoundConfig `toml:"low" yaml:"low" json:"low"`
	Normal   SoundConfig `toml:"normal" yaml:"normal" json:"normal"`
	Critical SoundConfig `toml:"critical" yaml:"critical" json:"critical"`
}

// SoundConfig defines a sound and its playback parameters.
type SoundConfig struct {
	// Path to the audio file (relative to theme directory or absolute)
	Path string `toml:"path" yaml:"path" json:"path"`

	// Volume override for this sound (0.0-1.0, 0 = use global volume)
	Volume float64 `toml:"volume" yaml:"volume" json:"volume"`

	// RepeatCount controls how many times to repeat the sound:
	//   -1 = don't repeat (play once)
	//    0 = repeat until notification is dismissed
	//   >0 = repeat exactly N times
	RepeatCount int `toml:"repeat_count" yaml:"repeat_count" json:"repeat_count"`

	// RepeatDelay is the delay between repeats (default: 10s)
	// Ignored if RepeatCount is -1
	RepeatDelay Duration `toml:"repeat_delay" yaml:"repeat_delay" json:"repeat_delay"`
}

// Duration is a time.Duration that supports TOML/YAML/JSON string parsing.
type Duration time.Duration

// DefaultRepeatDelay is the default delay between sound repeats.
const DefaultRepeatDelay = 10 * time.Second

// UnmarshalText parses duration from string (e.g., "10s", "1m", "500ms").
func (d *Duration) UnmarshalText(text []byte) error {
	duration, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}
	*d = Duration(duration)
	return nil
}

// MarshalText converts duration to string.
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(time.Duration(d).String()), nil
}

// Duration returns the underlying time.Duration.
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// GetRepeatDelay returns the repeat delay, using DefaultRepeatDelay if not set.
func (s *SoundConfig) GetRepeatDelay() time.Duration {
	if s.RepeatDelay == 0 {
		return DefaultRepeatDelay
	}
	return s.RepeatDelay.Duration()
}

// ShouldRepeat returns true if the sound should be repeated.
func (s *SoundConfig) ShouldRepeat() bool {
	return s.RepeatCount >= 0
}

// IsEnabled returns true if a sound path is configured.
func (s *SoundConfig) IsEnabled() bool {
	return s.Path != ""
}

// IconManifest defines icon configuration for the theme.
type IconManifest struct {
	// Size is the icon size in pixels (default: 48)
	Size int `toml:"size" yaml:"size" json:"size"`
}

// GetIconSize returns the configured icon size or the default (48).
func (m *Manifest) GetIconSize() int {
	if m.Icon.Size > 0 {
		return m.Icon.Size
	}
	return 48
}

// GetSoundConfig returns the sound configuration for a given urgency level.
func (m *Manifest) GetSoundConfig(urgency int) *SoundConfig {
	switch urgency {
	case 0:
		return &m.Audio.Low
	case 1:
		return &m.Audio.Normal
	case 2:
		return &m.Audio.Critical
	default:
		return &m.Audio.Normal
	}
}

// LoadManifest loads a manifest from a file.
// Supports .toml, .yaml, .yml, and .json extensions.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	ext := strings.ToLower(filepath.Ext(path))
	return ParseManifest(data, ext)
}

// ParseManifest parses a manifest from bytes.
// The ext parameter hints at the format (.toml, .yaml, .yml, .json).
// If ext is empty, TOML is tried first, then YAML.
func ParseManifest(data []byte, ext string) (*Manifest, error) {
	manifest := &Manifest{}
	var err error

	switch ext {
	case ".toml", "toml":
		err = toml.Unmarshal(data, manifest)
	case ".yaml", ".yml", "yaml", "yml":
		err = yaml.Unmarshal(data, manifest)
	case ".json", "json":
		// JSON is handled by yaml.Unmarshal since it's a subset of YAML
		err = yaml.Unmarshal(data, manifest)
	default:
		// Try TOML first, then YAML
		err = toml.Unmarshal(data, manifest)
		if err != nil {
			err = yaml.Unmarshal(data, manifest)
		}
	}

	if err != nil {
		return nil, err
	}

	return manifest, nil
}

// FindManifest looks for a manifest file in a theme directory.
// Returns the path to the manifest file and whether it was found.
// Checks for: manifest.toml, manifest.yaml, manifest.yml, manifest.json
func FindManifest(themeDir string) (string, bool) {
	candidates := []string{
		"manifest.toml",
		"manifest.yaml",
		"manifest.yml",
		"manifest.json",
	}

	for _, name := range candidates {
		path := filepath.Join(themeDir, name)
		if _, err := os.Stat(path); err == nil {
			return path, true
		}
	}

	return "", false
}

// DefaultManifest returns a manifest with default values.
func DefaultManifest() *Manifest {
	return &Manifest{
		Icon: IconManifest{
			Size: 48,
		},
		Audio: AudioManifest{
			Low: SoundConfig{
				RepeatCount: -1, // Don't repeat
			},
			Normal: SoundConfig{
				RepeatCount: -1, // Don't repeat
			},
			Critical: SoundConfig{
				RepeatCount: -1, // Don't repeat
			},
		},
	}
}

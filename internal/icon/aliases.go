package icon

import (
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"

	"github.com/jmylchreest/histui/embed"
)

// IconEntry represents an icon with both symbol and GTK icon variants.
// Used in [apps] and [categories] sections of the new aliases format.
type IconEntry struct {
	Symbol  string `toml:"symbol"`
	GtkIcon string `toml:"gtk_icon"`
}

// AliasesMeta contains metadata about the aliases file.
type AliasesMeta struct {
	Version     int    `toml:"version"`
	GeneratedAt string `toml:"generated_at"`
	Generator   string `toml:"generator"`
}

// AliasesFile represents the structure of the icon-aliases.toml file.
// Supports both old format ([symbols], [gtk-icons]) and new format ([apps], [categories]).
type AliasesFile struct {
	// Meta contains version and generation metadata
	Meta AliasesMeta `toml:"meta"`

	// Aliases maps app names to canonical icon names
	// Example: zapzap = "whatsapp", evolution = "email"
	Aliases map[string]string `toml:"aliases"`

	// Apps maps canonical app names to their icons (symbol + gtk_icon)
	// These are brand-specific icons (e.g., spotify, discord, firefox)
	// Example: spotify = { symbol = "", gtk_icon = "spotify-client" }
	Apps map[string]IconEntry `toml:"apps"`

	// Categories maps category names to their fallback icons
	// Used when an app has no brand icon, falls back to generic category icon
	// Example: email = { symbol = "", gtk_icon = "mail-symbolic" }
	Categories map[string]IconEntry `toml:"categories"`

	// Legacy format support (for backwards compatibility with theme/user files):

	// Symbols maps canonical icon names to symbol font glyph characters
	// Deprecated: Use [apps] and [categories] sections instead
	Symbols map[string]string `toml:"symbols"`

	// GtkIcons maps canonical icon names to GTK icon names
	// Deprecated: Use [apps] and [categories] sections instead
	GtkIcons map[string]string `toml:"gtk-icons"`
}

// AliasesStats contains statistics about loaded aliases.
type AliasesStats struct {
	Meta       AliasesMeta
	Aliases    int // Number of app-to-icon aliases
	Apps       int // Number of brand/app icons
	Categories int // Number of category fallback icons
}

// GetEmbeddedAliasesStats returns metadata and statistics about the embedded aliases file.
func GetEmbeddedAliasesStats() (AliasesStats, error) {
	file, err := LoadEmbeddedAliasesFile()
	if err != nil {
		return AliasesStats{}, err
	}
	return AliasesStats{
		Meta:       file.Meta,
		Aliases:    len(file.Aliases),
		Apps:       len(file.Apps),
		Categories: len(file.Categories),
	}, nil
}

// UserAliasesPath returns the path to the user's icon aliases file.
func UserAliasesPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "histui", "icon-aliases.toml"), nil
}

// loadAliasesFileFromData parses TOML data into an AliasesFile.
// It merges [apps] and [categories] sections into Symbols/GtkIcons maps
// for backwards-compatible resolver usage.
func loadAliasesFileFromData(data []byte) (*AliasesFile, error) {
	var file AliasesFile
	if err := toml.Unmarshal(data, &file); err != nil {
		return nil, err
	}

	// Initialize maps if nil
	if file.Aliases == nil {
		file.Aliases = make(map[string]string)
	}
	if file.Apps == nil {
		file.Apps = make(map[string]IconEntry)
	}
	if file.Categories == nil {
		file.Categories = make(map[string]IconEntry)
	}
	if file.Symbols == nil {
		file.Symbols = make(map[string]string)
	}
	if file.GtkIcons == nil {
		file.GtkIcons = make(map[string]string)
	}

	// Merge [apps] into Symbols/GtkIcons (apps take priority)
	for name, entry := range file.Apps {
		if entry.Symbol != "" {
			file.Symbols[name] = entry.Symbol
		}
		if entry.GtkIcon != "" {
			file.GtkIcons[name] = entry.GtkIcon
		}
	}

	// Merge [categories] into Symbols/GtkIcons (categories are fallbacks,
	// so only add if not already present from apps or legacy sections)
	for name, entry := range file.Categories {
		if entry.Symbol != "" {
			if _, exists := file.Symbols[name]; !exists {
				file.Symbols[name] = entry.Symbol
			}
		}
		if entry.GtkIcon != "" {
			if _, exists := file.GtkIcons[name]; !exists {
				file.GtkIcons[name] = entry.GtkIcon
			}
		}
	}

	return &file, nil
}

// loadAliasesFromData parses TOML data into an aliases map (for backwards compatibility).
func loadAliasesFromData(data []byte) (map[string]string, error) {
	file, err := loadAliasesFileFromData(data)
	if err != nil {
		return nil, err
	}
	return file.Aliases, nil
}

// LoadEmbeddedAliasesFile returns the built-in default aliases, symbols, and GTK icons.
func LoadEmbeddedAliasesFile() (*AliasesFile, error) {
	return loadAliasesFileFromData(embed.AliasesDefaultTOML)
}

// LoadUserAliasesFile loads icon aliases, symbols, and GTK icons from the user's config file.
// Returns nil if the file doesn't exist.
func LoadUserAliasesFile() (*AliasesFile, error) {
	path, err := UserAliasesPath()
	if err != nil {
		return nil, nil
	}

	// If file doesn't exist, return nil (not an error)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return loadAliasesFileFromData(data)
}

// LoadUserAliases loads icon aliases from the user's config file.
// Returns an empty map if the file doesn't exist.
func LoadUserAliases() (map[string]string, error) {
	path, err := UserAliasesPath()
	if err != nil {
		return make(map[string]string), nil
	}

	// If file doesn't exist, return empty map (not an error)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return make(map[string]string), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return loadAliasesFromData(data)
}

// NewResolverWithAliases creates a resolver with embedded defaults merged with user aliases.
// User aliases, symbols, and GTK icons take priority over embedded defaults.
func NewResolverWithAliases() (*Resolver, error) {
	r := NewResolver()

	// Load embedded defaults first (aliases, symbols, and GTK icons)
	defaults, err := LoadEmbeddedAliasesFile()
	if err != nil {
		return r, err
	}
	r.SetDefaultAliases(defaults.Aliases)
	r.SetDefaultSymbols(defaults.Symbols)
	r.SetDefaultGtkIcons(defaults.GtkIcons)

	// Load and merge user aliases (these override defaults)
	userFile, err := LoadUserAliasesFile()
	if err != nil {
		return r, err
	}
	if userFile != nil {
		r.AddAliases(userFile.Aliases)
		r.AddSymbols(userFile.Symbols)
		r.AddGtkIcons(userFile.GtkIcons)
	}

	return r, nil
}

// ThemeAliasesFile represents the structure of a theme's aliases.toml file.
// Supports both old format ([symbols], [gtk-icons]) and new format ([apps], [categories]).
type ThemeAliasesFile struct {
	// Aliases maps app names/urgency keys to canonical icon names
	Aliases map[string]string `toml:"aliases"`

	// Apps maps canonical app names to their icons (new format)
	Apps map[string]IconEntry `toml:"apps"`

	// Categories maps category names to their fallback icons (new format)
	Categories map[string]IconEntry `toml:"categories"`

	// Symbols maps canonical icon names to symbol font glyph characters (legacy)
	Symbols map[string]string `toml:"symbols"`

	// GtkIcons maps canonical icon names to GTK icon names (legacy)
	GtkIcons map[string]string `toml:"gtk-icons"`
}

// LoadThemeAliasesFile parses theme aliases TOML and returns all sections.
// The aliasesData should be the raw TOML content (e.g., from theme.GetEmbeddedAliases).
// It merges [apps] and [categories] into Symbols/GtkIcons for backwards compatibility.
func LoadThemeAliasesFile(aliasesData string) (*ThemeAliasesFile, error) {
	var file ThemeAliasesFile
	if err := toml.Unmarshal([]byte(aliasesData), &file); err != nil {
		return nil, err
	}

	// Initialize maps if nil
	if file.Aliases == nil {
		file.Aliases = make(map[string]string)
	}
	if file.Apps == nil {
		file.Apps = make(map[string]IconEntry)
	}
	if file.Categories == nil {
		file.Categories = make(map[string]IconEntry)
	}
	if file.Symbols == nil {
		file.Symbols = make(map[string]string)
	}
	if file.GtkIcons == nil {
		file.GtkIcons = make(map[string]string)
	}

	// Merge [apps] into Symbols/GtkIcons (apps take priority)
	for name, entry := range file.Apps {
		if entry.Symbol != "" {
			file.Symbols[name] = entry.Symbol
		}
		if entry.GtkIcon != "" {
			file.GtkIcons[name] = entry.GtkIcon
		}
	}

	// Merge [categories] into Symbols/GtkIcons
	for name, entry := range file.Categories {
		if entry.Symbol != "" {
			if _, exists := file.Symbols[name]; !exists {
				file.Symbols[name] = entry.Symbol
			}
		}
		if entry.GtkIcon != "" {
			if _, exists := file.GtkIcons[name]; !exists {
				file.GtkIcons[name] = entry.GtkIcon
			}
		}
	}

	return &file, nil
}

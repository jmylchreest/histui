package icon

import (
	_ "embed"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

//go:embed aliases_default.toml
var defaultAliasesData []byte

// AliasesFile represents the structure of the icon-aliases.toml file.
type AliasesFile struct {
	// Aliases maps app names to icon names
	// Example: zapzap = "whatsapp"
	Aliases map[string]string `toml:"aliases"`

	// Symbols maps icon names to Nerd Font glyph characters
	// Example: spotify = "\U000F04C7"
	Symbols map[string]string `toml:"symbols"`
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
func loadAliasesFileFromData(data []byte) (*AliasesFile, error) {
	var file AliasesFile
	if err := toml.Unmarshal(data, &file); err != nil {
		return nil, err
	}

	if file.Aliases == nil {
		file.Aliases = make(map[string]string)
	}
	if file.Symbols == nil {
		file.Symbols = make(map[string]string)
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

// LoadEmbeddedAliasesFile returns the built-in default aliases and symbols.
func LoadEmbeddedAliasesFile() (*AliasesFile, error) {
	return loadAliasesFileFromData(defaultAliasesData)
}

// LoadEmbeddedAliases returns the built-in default aliases.
func LoadEmbeddedAliases() (map[string]string, error) {
	return loadAliasesFromData(defaultAliasesData)
}

// LoadUserAliasesFile loads icon aliases and symbols from the user's config file.
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
// User aliases and symbols take priority over embedded defaults.
func NewResolverWithAliases() (*Resolver, error) {
	r := NewResolver()

	// Load embedded defaults first (aliases and symbols)
	defaults, err := LoadEmbeddedAliasesFile()
	if err != nil {
		return r, err
	}
	r.SetDefaultAliases(defaults.Aliases)
	r.SetDefaultSymbols(defaults.Symbols)

	// Load and merge user aliases (these override defaults)
	userFile, err := LoadUserAliasesFile()
	if err != nil {
		return r, err
	}
	if userFile != nil {
		r.AddAliases(userFile.Aliases)
		r.AddSymbols(userFile.Symbols)
	}

	return r, nil
}

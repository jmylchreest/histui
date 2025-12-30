package theme

import (
	"os"
	"path/filepath"
)

// ThemesDir returns the path to the user's themes directory.
func ThemesDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "histui", "themes"), nil
}

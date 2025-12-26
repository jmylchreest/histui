package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, "48h", cfg.Filter.Since)
	assert.Equal(t, 0, cfg.Filter.Limit)
	assert.Equal(t, "timestamp", cfg.Sort.Field)
	assert.Equal(t, "desc", cfg.Sort.Order)
	assert.Equal(t, "48h", cfg.Prune.OlderThan)
	assert.Equal(t, 0, cfg.Prune.Keep)
	assert.True(t, cfg.TUI.ShowIcons)
	assert.Equal(t, 64, cfg.TUI.IconSize)
	assert.True(t, cfg.TUI.ShowHelp)
	assert.NotEmpty(t, cfg.Templates.Dmenu)
	assert.NotEmpty(t, cfg.Templates.Full)
	assert.NotEmpty(t, cfg.Templates.Body)
	assert.NotEmpty(t, cfg.Templates.TUIOutput)
}

func TestLoadConfig_DefaultsWhenNoFile(t *testing.T) {
	// Use a path that doesn't exist
	cfg, err := LoadConfig("/nonexistent/path/config.toml")
	require.NoError(t, err)
	assert.Equal(t, DefaultConfig().Filter.Since, cfg.Filter.Since)
}

func TestLoadConfig_ParsesTOML(t *testing.T) {
	// Create a temporary config file
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[filter]
since = "24h"
limit = 100

[sort]
field = "app"
order = "asc"

[prune]
older_than = "7d"
keep = 500

[templates]
dmenu = "{{.AppName}}: {{.Summary}}"

[templates.custom]
slack = "{{.Summary}}: {{.Body}}"

[tui]
show_icons = false
icon_size = 32
show_help = false

[clipboard]
command = "xclip"
`
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := LoadConfig(path)
	require.NoError(t, err)

	assert.Equal(t, "24h", cfg.Filter.Since)
	assert.Equal(t, 100, cfg.Filter.Limit)
	assert.Equal(t, "app", cfg.Sort.Field)
	assert.Equal(t, "asc", cfg.Sort.Order)
	assert.Equal(t, "7d", cfg.Prune.OlderThan)
	assert.Equal(t, 500, cfg.Prune.Keep)
	assert.Equal(t, "{{.AppName}}: {{.Summary}}", cfg.Templates.Dmenu)
	assert.Equal(t, "{{.Summary}}: {{.Body}}", cfg.Templates.Custom["slack"])
	assert.False(t, cfg.TUI.ShowIcons)
	assert.Equal(t, 32, cfg.TUI.IconSize)
	assert.False(t, cfg.TUI.ShowHelp)
	assert.Equal(t, "xclip", cfg.Clipboard.Command)
}

func TestLoadConfig_PartialConfig(t *testing.T) {
	// Create a config with only some fields
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[filter]
since = "1h"
`
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := LoadConfig(path)
	require.NoError(t, err)

	// Changed field
	assert.Equal(t, "1h", cfg.Filter.Since)

	// Unchanged fields should have defaults
	assert.Equal(t, 0, cfg.Filter.Limit)
	assert.Equal(t, "timestamp", cfg.Sort.Field)
	assert.True(t, cfg.TUI.ShowIcons)
}

func TestLoadConfig_InvalidTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `this is not valid toml [`
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)

	_, err = LoadConfig(path)
	assert.Error(t, err)
}

func TestConfig_Save(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "config.toml")

	cfg := DefaultConfig()
	cfg.Filter.Since = "1h"
	cfg.Templates.Custom["test"] = "custom template"

	err := cfg.Save(path)
	require.NoError(t, err)

	// Verify file was created
	_, err = os.Stat(path)
	require.NoError(t, err)

	// Reload and verify
	loaded, err := LoadConfig(path)
	require.NoError(t, err)
	assert.Equal(t, "1h", loaded.Filter.Since)
	assert.Equal(t, "custom template", loaded.Templates.Custom["test"])
}

func TestConfig_GetTemplate(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Templates.Custom["mytemplate"] = "custom: {{.Body}}"

	tests := []struct {
		name     string
		expected string
	}{
		{"dmenu", cfg.Templates.Dmenu},
		{"full", cfg.Templates.Full},
		{"body", cfg.Templates.Body},
		{"json", cfg.Templates.JSON},
		{"tui_output", cfg.Templates.TUIOutput},
		{"mytemplate", "custom: {{.Body}}"},
		{"nonexistent", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, cfg.GetTemplate(tt.name))
		})
	}
}

func TestConfigPath(t *testing.T) {
	// Test with XDG_CONFIG_HOME set
	t.Setenv("XDG_CONFIG_HOME", "/custom/config")
	assert.Equal(t, "/custom/config/histui/config.toml", ConfigPath())
}

func TestConfigPathDefault(t *testing.T) {
	// Test without XDG_CONFIG_HOME (uses default)
	path := ConfigPath()
	assert.Contains(t, path, "histui/config.toml")
}

func TestDataPath(t *testing.T) {
	// Test with XDG_DATA_HOME set
	t.Setenv("XDG_DATA_HOME", "/custom/data")
	assert.Equal(t, "/custom/data/histui", DataPath())
}

func TestDataPathDefault(t *testing.T) {
	// Test without XDG_DATA_HOME (uses default)
	path := DataPath()
	assert.Contains(t, path, "histui")
}

func TestHistoryPath(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/custom/data")
	assert.Equal(t, "/custom/data/histui/history.jsonl", HistoryPath())
}

func TestEnsureDataDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)

	err := EnsureDataDir()
	require.NoError(t, err)

	// Verify directory was created
	info, err := os.Stat(filepath.Join(dir, "histui"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

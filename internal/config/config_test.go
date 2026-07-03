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

func TestDuration_UnmarshalText(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantMs  int
		wantErr bool
	}{
		{"seconds", "5s", 5000, false},
		{"minutes", "2m", 120000, false},
		{"hours", "1h", 3600000, false},
		{"complex", "1h30m", 5400000, false},
		{"zero", "0", 0, false},
		{"milliseconds_int", "5000", 5000, false},
		{"ms_suffix", "500ms", 500, false},
		{"never", "never", -1, false},
		{"never_upper", "NEVER", -1, false},
		{"negative", "-1s", -1000, false},
		{"invalid", "invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d Duration
			err := d.UnmarshalText([]byte(tt.input))
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantMs, d.Milliseconds())
			}
		})
	}
}

func TestDuration_MarshalText(t *testing.T) {
	tests := []struct {
		name  string
		input Duration
		want  string
	}{
		{"5_seconds", Duration(5 * 1e9), "5s"},
		{"2_minutes", Duration(120 * 1e9), "2m0s"},
		{"zero", Duration(0), "0s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, err := tt.input.MarshalText()
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(text))
		})
	}
}

func TestDaemonConfig_Timeouts(t *testing.T) {
	cfg := DefaultDaemonConfig()

	// Default config: low=0 (honor client), normal=0 (honor client), critical=never, fallback=10s
	// When client timeout is -1 (server decides), use fallback
	assert.Equal(t, 10000, cfg.GetTimeoutForUrgency(0, -1)) // Low: fallback 10s
	assert.Equal(t, 10000, cfg.GetTimeoutForUrgency(1, -1)) // Normal: fallback 10s
	assert.Equal(t, 0, cfg.GetTimeoutForUrgency(2, -1))     // Critical: never expires

	// Client timeout should be honored when config is 0
	assert.Equal(t, 3000, cfg.GetTimeoutForUrgency(0, 3000)) // Low: honor client 3s
	assert.Equal(t, 3000, cfg.GetTimeoutForUrgency(1, 3000)) // Normal: honor client 3s
}

func TestDaemonConfig_TimeoutsHonorClient(t *testing.T) {
	cfg := DefaultDaemonConfig()
	// Default already has config to "0" (honor client) for low/normal
	// Set critical to 0 as well for this test
	cfg.Timeouts.Critical = Duration(0)

	// Client timeout -1 means server decides -> use fallback (10s default)
	assert.Equal(t, 10000, cfg.GetTimeoutForUrgency(0, -1)) // Low: fallback 10s
	assert.Equal(t, 10000, cfg.GetTimeoutForUrgency(1, -1)) // Normal: fallback 10s
	assert.Equal(t, 0, cfg.GetTimeoutForUrgency(2, -1))     // Critical: always 0 (never)

	// Client timeout 0 means never expire
	assert.Equal(t, 0, cfg.GetTimeoutForUrgency(0, 0)) // Honor client: never
	assert.Equal(t, 0, cfg.GetTimeoutForUrgency(1, 0)) // Honor client: never
	assert.Equal(t, 0, cfg.GetTimeoutForUrgency(2, 0)) // Honor client: never

	// Client positive timeout should be honored
	assert.Equal(t, 3000, cfg.GetTimeoutForUrgency(0, 3000))   // Honor client: 3s
	assert.Equal(t, 7500, cfg.GetTimeoutForUrgency(1, 7500))   // Honor client: 7.5s
	assert.Equal(t, 15000, cfg.GetTimeoutForUrgency(2, 15000)) // Honor client: 15s
}

func TestDaemonConfig_LoadWithDurations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "histuid.toml")

	content := `
[timeouts]
low = "3s"
normal = "15s"
critical = "0"
`
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)

	// Temporarily override config path
	oldConfigPath := os.Getenv("XDG_CONFIG_HOME")
	t.Setenv("XDG_CONFIG_HOME", dir)
	defer func() {
		if oldConfigPath != "" {
			os.Setenv("XDG_CONFIG_HOME", oldConfigPath)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	// Create the histui subdirectory and move the file
	histuiDir := filepath.Join(dir, "histui")
	require.NoError(t, os.MkdirAll(histuiDir, 0755))
	require.NoError(t, os.Rename(path, filepath.Join(histuiDir, "histuid.toml")))

	cfg, err := LoadDaemonConfig()
	require.NoError(t, err)

	// Config has positive values, so they override client timeout (-1 = server decides)
	assert.Equal(t, 3000, cfg.GetTimeoutForUrgency(0, -1))  // 3s override
	assert.Equal(t, 15000, cfg.GetTimeoutForUrgency(1, -1)) // 15s override
	assert.Equal(t, 0, cfg.GetTimeoutForUrgency(2, -1))     // 0 (never) from config
}

func TestDaemonConfig_LoadMonitorSelector(t *testing.T) {
	cases := []struct {
		name     string
		value    string // literal TOML value for `monitor = `
		wantIdx  int
		wantName string
	}{
		{"default_auto", "", 0, ""},
		{"index_int", "monitor = 2", 2, ""},
		{"index_string", `monitor = "3"`, 3, ""},
		{"connector_name", `monitor = "DP-1"`, 0, "DP-1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			histuiDir := filepath.Join(dir, "histui")
			require.NoError(t, os.MkdirAll(histuiDir, 0755))
			content := "[display]\n" + c.value + "\n"
			require.NoError(t, os.WriteFile(filepath.Join(histuiDir, "histuid.toml"), []byte(content), 0644))
			t.Setenv("XDG_CONFIG_HOME", dir)

			cfg, err := LoadDaemonConfig()
			require.NoError(t, err)
			assert.Equal(t, c.wantIdx, cfg.Display.Monitor.Index)
			assert.Equal(t, c.wantName, cfg.Display.Monitor.Name)
		})
	}
}

func TestDaemonConfig_LoadMonitorNegativeIndexErrors(t *testing.T) {
	dir := t.TempDir()
	histuiDir := filepath.Join(dir, "histui")
	require.NoError(t, os.MkdirAll(histuiDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(histuiDir, "histuid.toml"), []byte("[display]\nmonitor = -1\n"), 0644))
	t.Setenv("XDG_CONFIG_HOME", dir)

	_, err := LoadDaemonConfig()
	require.Error(t, err)
}

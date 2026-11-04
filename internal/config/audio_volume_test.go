package config

import (
	"github.com/spf13/viper"
	"os"
	"path/filepath"
	"testing"
)

func TestAudioVolumeScaleUpgrade(t *testing.T) {
	for _, tc := range []struct {
		name, body, env, scale string
		volume                 int
		bad                    bool
	}{
		{"fresh", "", "", "linear", 26, false},
		{"existing defaults", "[theme]\nname='default'\n", "", "legacy", 80, false},
		{"existing explicit", "[audio]\nvolume=20\n", "", "legacy", 20, false},
		{"explicit linear", "[audio]\nvolume_scale='linear'\nvolume=20\n", "", "linear", 20, false},
		{"new defaults", "[audio]\nvolume_scale='linear'\n", "", "linear", 26, false},
		{"legacy opt-in", "[audio]\nvolume_scale='legacy'\n", "", "legacy", 80, false},
		{"legacy env", "", "20", "legacy", 20, false},
		{"mute", "[audio]\nvolume_scale='linear'\nvolume=0\n", "", "linear", 0, false},
		{"invalid scale", "[audio]\nvolume_scale='typo'\n", "", "", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", dir)
			t.Setenv("HISTUID_AUDIO_VOLUME", tc.env)
			t.Setenv("HISTUID_AUDIO_VOLUME_SCALE", "")
			if tc.body != "" {
				if err := os.MkdirAll(filepath.Join(dir, "histui"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "histui", "histuid.toml"), []byte(tc.body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			v, err := NewViper()
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := LoadDaemonConfigWithViper(v)
			if tc.bad {
				if err == nil {
					t.Fatal("expected invalid scale error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Audio.VolumeScale != tc.scale || cfg.Audio.Volume != tc.volume {
				t.Fatalf("got %+v, want %s %d", cfg.Audio, tc.scale, tc.volume)
			}
		})
	}
}
func TestAudioVolumeScaleReloadDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "histuid.toml")
	v := viper.New()
	setDefaults(v)
	v.SetConfigFile(path)
	for _, tc := range []struct {
		body, scale string
		volume      int
	}{
		{"[audio]\n", "legacy", 80},
		{"[audio]\nvolume_scale='linear'\n", "linear", 26},
		{"[audio]\n", "legacy", 80},
	} {
		if err := os.WriteFile(path, []byte(tc.body), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := LoadDaemonConfigWithViper(v)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Audio.VolumeScale != tc.scale || cfg.Audio.Volume != tc.volume {
			t.Fatalf("reload got %+v", cfg.Audio)
		}
	}
}

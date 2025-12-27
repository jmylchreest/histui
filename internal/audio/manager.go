package audio

import (
	"context"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"sync"

	"github.com/jmylchreest/histui/internal/config"
)

// Manager manages audio playback for notifications with urgency-based sounds.
type Manager struct {
	mu      sync.RWMutex
	logger  *slog.Logger
	player  *Player
	watcher *Watcher
	config  *config.DaemonConfig

	// Urgency to sound path mapping
	sounds map[int]string
}

// NewManager creates a new audio manager.
func NewManager(cfg *config.DaemonConfig, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}

	player := NewPlayer(logger)

	m := &Manager{
		logger:  logger,
		player:  player,
		watcher: NewWatcher(player, logger),
		config:  cfg,
		sounds:  make(map[int]string),
	}

	// Load sound configuration
	m.loadSoundConfig()

	return m
}

// loadSoundConfig loads sounds from the configuration.
func (m *Manager) loadSoundConfig() {
	if m.config == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Set volume (config uses 0-100, player uses 0.0-1.0)
	if m.config.Audio.Volume > 0 {
		m.player.SetVolume(float64(m.config.Audio.Volume) / 100.0)
	}

	// Load per-urgency sounds
	sounds := map[int]string{
		0: m.config.Audio.Sounds.Low,      // Low urgency
		1: m.config.Audio.Sounds.Normal,   // Normal urgency
		2: m.config.Audio.Sounds.Critical, // Critical urgency
	}

	for urgency, path := range sounds {
		if path == "" {
			continue
		}

		// Expand path
		expandedPath := expandPath(path)

		// Check if file exists
		if _, err := os.Stat(expandedPath); err != nil {
			m.logger.Warn("sound file not found", "urgency", urgency, "path", expandedPath)
			continue
		}

		m.sounds[urgency] = expandedPath
		m.logger.Debug("loaded sound", "urgency", urgency, "path", expandedPath)
	}
}

// Start initializes the audio manager and starts the file watcher.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.RLock()
	sounds := make(map[int]string, len(m.sounds))
	maps.Copy(sounds, m.sounds)
	m.mu.RUnlock()

	// Preload all sounds
	for _, path := range sounds {
		if err := m.player.Preload(path); err != nil {
			m.logger.Warn("failed to preload sound", "path", path, "error", err)
		}
		m.watcher.Watch(path)
	}

	// Start the watcher
	if err := m.watcher.Start(ctx); err != nil {
		return err
	}

	m.logger.Info("audio manager started", "sounds", len(sounds))
	return nil
}

// Stop shuts down the audio manager.
func (m *Manager) Stop() {
	m.watcher.Stop()
	m.player.Close()
	m.logger.Debug("audio manager stopped")
}

// PlayForUrgency plays the sound configured for the given urgency level.
func (m *Manager) PlayForUrgency(urgency int) error {
	if !m.config.Audio.Enabled {
		return nil
	}

	m.mu.RLock()
	path, ok := m.sounds[urgency]
	m.mu.RUnlock()

	if !ok {
		m.logger.Debug("no sound configured for urgency", "urgency", urgency)
		return nil
	}

	return m.player.Play(path)
}

// PlayFile plays a specific sound file.
func (m *Manager) PlayFile(path string) error {
	if !m.config.Audio.Enabled {
		return nil
	}
	return m.player.Play(path)
}

// SetVolume sets the playback volume (0.0 to 1.0).
func (m *Manager) SetVolume(volume float64) {
	m.player.SetVolume(volume)
}

// GetVolume returns the current volume.
func (m *Manager) GetVolume() float64 {
	return m.player.GetVolume()
}

// Reload reloads the sound configuration.
func (m *Manager) Reload() {
	m.player.ClearCache()
	m.loadSoundConfig()

	// Re-preload and watch sounds
	m.mu.RLock()
	sounds := make(map[int]string, len(m.sounds))
	maps.Copy(sounds, m.sounds)
	m.mu.RUnlock()

	for _, path := range sounds {
		if err := m.player.Preload(path); err != nil {
			m.logger.Warn("failed to preload sound on reload", "path", path, "error", err)
		}
		m.watcher.Watch(path)
	}

	m.logger.Debug("audio manager reloaded")
}

// UpdateConfig updates the configuration and reloads sounds.
// This is called when the config file is hot-reloaded.
func (m *Manager) UpdateConfig(cfg *config.DaemonConfig) {
	m.mu.Lock()
	m.config = cfg
	m.mu.Unlock()

	m.logger.Debug("audio manager config updated")
	m.Reload()
}

// expandPath expands ~ to home directory.
func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}

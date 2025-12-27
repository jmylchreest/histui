package audio

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/jmylchreest/histui/internal/config"
)

// soundEntry holds the path and volume for a sound.
type soundEntry struct {
	path   string
	volume float64
}

// Manager manages audio playback for notifications with urgency-based sounds.
type Manager struct {
	mu      sync.RWMutex
	logger  *slog.Logger
	player  *Player
	watcher *Watcher
	config  *config.DaemonConfig

	// Urgency to sound configuration mapping
	sounds map[int]soundEntry
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
		sounds:  make(map[int]soundEntry),
	}

	// Load sound configuration
	m.loadSoundConfig()

	return m
}

// loadSoundConfig loads audio settings from the configuration.
// Note: Sound files come from theme manifests, not daemon config.
// This only sets the global volume from daemon config.
func (m *Manager) loadSoundConfig() {
	if m.config == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Set global volume (config uses 0-100, player uses 0.0-1.0)
	if m.config.Audio.Volume > 0 {
		globalVolume := float64(m.config.Audio.Volume) / 100.0
		m.player.SetVolume(globalVolume)
	}

	// Sound files are loaded from theme manifests via SetSoundForUrgency
	// called by the theme loader after loading a theme
}

// ClearSounds removes all configured sounds.
// This is called when switching themes to ensure old theme sounds are cleared.
func (m *Manager) ClearSounds() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sounds = make(map[int]soundEntry)
	m.player.ClearCache()
}

// SetSoundForUrgency sets the sound path and volume for a specific urgency level.
// The volume is relative to the global volume (0.0-1.0, where 0 means use global).
// This is typically called when loading sounds from a theme manifest.
func (m *Manager) SetSoundForUrgency(urgency int, path string, volume float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if path == "" {
		delete(m.sounds, urgency)
		return
	}

	expandedPath := expandPath(path)
	if _, err := os.Stat(expandedPath); err != nil {
		m.logger.Warn("sound file not found", "urgency", urgency, "path", expandedPath)
		return
	}

	m.sounds[urgency] = soundEntry{
		path:   expandedPath,
		volume: volume,
	}
	m.logger.Debug("set sound", "urgency", urgency, "path", expandedPath, "volume", volume)

	// Preload and watch the new sound
	if err := m.player.Preload(expandedPath); err != nil {
		m.logger.Warn("failed to preload sound", "path", expandedPath, "error", err)
	}
	m.watcher.Watch(expandedPath)
}

// Start initializes the audio manager and starts the file watcher.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.RLock()
	sounds := make(map[int]soundEntry, len(m.sounds))
	for k, v := range m.sounds {
		sounds[k] = v
	}
	m.mu.RUnlock()

	// Preload all sounds
	for _, entry := range sounds {
		if err := m.player.Preload(entry.path); err != nil {
			m.logger.Warn("failed to preload sound", "path", entry.path, "error", err)
		}
		m.watcher.Watch(entry.path)
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
// Critical sounds (urgency 2) can interrupt any playing sound.
// Other urgencies won't play if a sound is already playing.
func (m *Manager) PlayForUrgency(urgency int) error {
	if !m.config.Audio.Enabled {
		return nil
	}

	m.mu.RLock()
	entry, ok := m.sounds[urgency]
	m.mu.RUnlock()

	if !ok {
		return nil
	}

	return m.player.PlayWithUrgency(entry.path, UrgencyLevel(urgency), entry.volume)
}

// PlayFile plays a specific sound file with normal urgency.
func (m *Manager) PlayFile(path string) error {
	return m.PlayFileWithUrgency(path, int(UrgencyNormal))
}

// PlayFileWithUrgency plays a specific sound file with the specified urgency.
// Uses the global volume (no per-sound override).
func (m *Manager) PlayFileWithUrgency(path string, urgency int) error {
	if !m.config.Audio.Enabled {
		return nil
	}
	return m.player.PlayWithUrgency(path, UrgencyLevel(urgency), 0) // 0 = use global volume
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
	sounds := make(map[int]soundEntry, len(m.sounds))
	for k, v := range m.sounds {
		sounds[k] = v
	}
	m.mu.RUnlock()

	for _, entry := range sounds {
		if err := m.player.Preload(entry.path); err != nil {
			m.logger.Warn("failed to preload sound on reload", "path", entry.path, "error", err)
		}
		m.watcher.Watch(entry.path)
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

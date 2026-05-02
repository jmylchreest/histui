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
//
// When audio is disabled (config.Audio.Enabled == false) the manager holds no
// Player or Watcher: no sounds are decoded, no inotify watches exist, and the
// pipewire/oto context is never opened. UpdateConfig handles transitions in
// either direction so hot-reload between enabled and disabled is symmetric.
type Manager struct {
	mu     sync.RWMutex
	logger *slog.Logger
	config *config.DaemonConfig

	// sounds is always populated, regardless of enabled state, so that a
	// disable→enable transition can preload without help from the caller.
	sounds map[int]soundEntry

	// Lazy resources. Non-nil only while audio is enabled.
	player  *Player
	watcher *Watcher

	// ctx is captured by Start so UpdateConfig can re-Start the watcher when
	// flipping back from disabled to enabled.
	ctx     context.Context
	started bool
}

// NewManager creates a new audio manager. No audio resources are allocated
// until Start is called and only if the config has audio enabled.
func NewManager(cfg *config.DaemonConfig, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}

	return &Manager{
		logger: logger,
		config: cfg,
		sounds: make(map[int]soundEntry),
	}
}

// isEnabledLocked reports whether audio is currently enabled by config.
// Caller must hold m.mu (R or W).
func (m *Manager) isEnabledLocked() bool {
	return m.config != nil && m.config.Audio.Enabled
}

// Start initializes the manager. If audio is enabled it creates the player
// and watcher and preloads any sounds already registered via
// SetSoundForUrgency. If disabled, it records the context and returns; a
// later UpdateConfig that flips audio on will then enable resources.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	m.ctx = ctx
	m.started = true
	enabled := m.isEnabledLocked()
	m.mu.Unlock()

	if !enabled {
		m.logger.Info("audio disabled, integration not started")
		return nil
	}
	return m.enable(ctx)
}

// enable creates the player + watcher, preloads sounds, and starts the
// watcher. It is a no-op if audio is already enabled. The expensive work
// (decoding sounds, starting the watcher) happens outside the manager lock.
func (m *Manager) enable(ctx context.Context) error {
	m.mu.Lock()
	if m.player != nil {
		m.mu.Unlock()
		return nil
	}

	player := NewPlayer(m.logger)
	watcher := NewWatcher(player, m.logger)

	if m.config != nil && m.config.Audio.Volume > 0 {
		player.SetVolume(float64(m.config.Audio.Volume) / 100.0)
	}

	sounds := make([]soundEntry, 0, len(m.sounds))
	for _, e := range m.sounds {
		sounds = append(sounds, e)
	}

	m.player = player
	m.watcher = watcher
	m.mu.Unlock()

	for _, e := range sounds {
		if err := player.Preload(e.path); err != nil {
			m.logger.Warn("failed to preload sound", "path", e.path, "error", err)
		}
		watcher.Watch(e.path)
	}

	if err := watcher.Start(ctx); err != nil {
		return err
	}

	m.logger.Info("audio integration enabled", "sounds", len(sounds))
	return nil
}

// disable tears down the player and watcher, releasing decoded sound buffers
// and any pipewire/oto context held by the player. Safe to call when already
// disabled.
func (m *Manager) disable() {
	m.mu.Lock()
	player, watcher := m.player, m.watcher
	m.player = nil
	m.watcher = nil
	m.mu.Unlock()

	if watcher != nil {
		watcher.Stop()
	}
	if player != nil {
		player.Close()
	}
	if player != nil || watcher != nil {
		m.logger.Info("audio integration disabled")
	}
}

// Stop shuts down the audio manager and releases all resources.
func (m *Manager) Stop() {
	m.disable()
	m.logger.Debug("audio manager stopped")
}

// ClearSounds removes all configured sounds. The player cache is cleared if
// audio is currently enabled.
func (m *Manager) ClearSounds() {
	m.mu.Lock()
	m.sounds = make(map[int]soundEntry)
	player := m.player
	m.mu.Unlock()

	if player != nil {
		player.ClearCache()
	}
}

// SetSoundForUrgency records a sound for an urgency level. The mapping is
// stored regardless of enabled state so a later disable→enable transition can
// preload it. Decoding and watching happen only while enabled.
func (m *Manager) SetSoundForUrgency(urgency int, path string, volume float64) {
	if path == "" {
		m.mu.Lock()
		delete(m.sounds, urgency)
		m.mu.Unlock()
		return
	}

	expandedPath := expandPath(path)
	if _, err := os.Stat(expandedPath); err != nil {
		m.logger.Warn("sound file not found", "urgency", urgency, "path", expandedPath)
		return
	}

	m.mu.Lock()
	m.sounds[urgency] = soundEntry{
		path:   expandedPath,
		volume: volume,
	}
	player := m.player
	watcher := m.watcher
	m.mu.Unlock()

	m.logger.Debug("set sound", "urgency", urgency, "path", expandedPath, "volume", volume)

	if player != nil {
		if err := player.Preload(expandedPath); err != nil {
			m.logger.Warn("failed to preload sound", "path", expandedPath, "error", err)
		}
		watcher.Watch(expandedPath)
	}
}

// PlayForUrgency plays the sound configured for the given urgency level.
// Critical sounds (urgency 2) can interrupt any playing sound.
// Other urgencies won't play if a sound is already playing.
func (m *Manager) PlayForUrgency(urgency int) error {
	m.mu.RLock()
	if !m.isEnabledLocked() || m.player == nil {
		m.mu.RUnlock()
		return nil
	}
	entry, ok := m.sounds[urgency]
	player := m.player
	m.mu.RUnlock()

	if !ok {
		return nil
	}
	return player.PlayWithUrgency(entry.path, UrgencyLevel(urgency), entry.volume)
}

// PlayFile plays a specific sound file with normal urgency.
func (m *Manager) PlayFile(path string) error {
	return m.PlayFileWithUrgency(path, int(UrgencyNormal))
}

// PlayFileWithUrgency plays a specific sound file with the specified urgency.
// Uses the global volume (no per-sound override).
func (m *Manager) PlayFileWithUrgency(path string, urgency int) error {
	m.mu.RLock()
	if !m.isEnabledLocked() || m.player == nil {
		m.mu.RUnlock()
		return nil
	}
	player := m.player
	m.mu.RUnlock()
	return player.PlayWithUrgency(path, UrgencyLevel(urgency), 0)
}

// StopPlayback stops any currently playing sound.
func (m *Manager) StopPlayback() {
	m.mu.RLock()
	player := m.player
	m.mu.RUnlock()
	if player != nil {
		player.Stop()
	}
}

// Reload re-applies volume from config and re-preloads sounds. No-op when
// audio is disabled.
func (m *Manager) Reload() {
	m.mu.RLock()
	player := m.player
	cfg := m.config
	sounds := make([]soundEntry, 0, len(m.sounds))
	for _, e := range m.sounds {
		sounds = append(sounds, e)
	}
	m.mu.RUnlock()

	if player == nil {
		return
	}

	if cfg != nil && cfg.Audio.Volume > 0 {
		player.SetVolume(float64(cfg.Audio.Volume) / 100.0)
	}
	player.ClearCache()

	for _, e := range sounds {
		if err := player.Preload(e.path); err != nil {
			m.logger.Warn("failed to preload sound on reload", "path", e.path, "error", err)
		}
	}

	m.logger.Debug("audio manager reloaded")
}

// UpdateConfig swaps in a new config and handles enable↔disable transitions.
// Called by the daemon's config hot-reload path.
func (m *Manager) UpdateConfig(cfg *config.DaemonConfig) {
	m.mu.Lock()
	wasEnabled := m.isEnabledLocked()
	m.config = cfg
	nowEnabled := m.isEnabledLocked()
	started := m.started
	ctx := m.ctx
	m.mu.Unlock()

	switch {
	case wasEnabled && !nowEnabled:
		m.disable()
	case !wasEnabled && nowEnabled && started:
		if err := m.enable(ctx); err != nil {
			m.logger.Error("failed to enable audio after config update", "error", err)
		}
	case nowEnabled:
		m.Reload()
	}

	m.logger.Debug("audio manager config updated", "enabled", nowEnabled)
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

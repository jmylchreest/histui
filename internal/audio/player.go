// Package audio provides notification sound playback.
package audio

import (
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/vorbis"
	"github.com/gopxl/beep/v2/wav"
	"github.com/jmylchreest/histui/internal/config"
)

// UrgencyLevel represents notification urgency for audio priority.
type UrgencyLevel int

const (
	UrgencyLow      UrgencyLevel = 0
	UrgencyNormal   UrgencyLevel = 1
	UrgencyCritical UrgencyLevel = 2
	UrgencyNone     UrgencyLevel = -1 // No sound playing
)

// Player handles audio playback for notifications.
type Player struct {
	mu     sync.Mutex
	logger *slog.Logger

	// Volume control (0.0 to 1.0)
	volume       float64
	legacyVolume bool

	// Whether this Player has opened an output stream
	initialized bool

	output         audioOutput
	closed         bool
	request        uint64
	loading        bool
	pendingUrgency UrgencyLevel
	current        *playbackSession

	// Sound cache
	cache      map[string]*cachedSound
	cacheMutex sync.RWMutex

	// Playback control
	currentUrgency UrgencyLevel // Urgency of currently playing sound
	playing        bool         // Whether a sound is currently playing
}

// cachedSound holds a decoded sound ready for playback.
type cachedSound struct {
	buffer *beep.Buffer
	path   string
}

// NewPlayer creates a new audio player.
func NewPlayer(logger *slog.Logger) *Player {
	if logger == nil {
		logger = slog.Default()
	}

	return &Player{
		logger:         logger,
		volume:         1.0,
		output:         &processAudioOutput,
		cache:          make(map[string]*cachedSound),
		currentUrgency: UrgencyNone,
	}
}

// SetVolume sets the playback volume (0.0 to 1.0).
func (p *Player) SetVolume(volume float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if volume < 0 {
		volume = 0
	}
	if volume > 1 {
		volume = 1
	}
	p.volume = volume
	p.legacyVolume = false
	p.logger.Debug("volume set", "volume", volume)
}

// setConfigVolume applies the upgrade compatibility policy atomically with gain.
func (p *Player) setConfigVolume(cfg config.AudioConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.volume = max(0, min(1, float64(cfg.Volume)/100))
	p.legacyVolume = cfg.VolumeScale == "" || cfg.VolumeScale == "legacy"
}

// PlayWithUrgency plays a sound file with the specified urgency level and volume.
// Critical urgency can interrupt any playing sound.
// Other urgencies won't play if a sound is already playing.
// If volume is <= 0, uses the player's global volume.
func (p *Player) PlayWithUrgency(path string, urgency UrgencyLevel, volume ...float64) error {
	if path == "" {
		return nil
	}

	// Determine volume to use
	var soundVolume float64
	if len(volume) > 0 && volume[0] > 0 {
		soundVolume = volume[0]
		if soundVolume > 1.0 {
			soundVolume = 1.0
		}
	}

	// Reserve admission before decoding, so concurrent non-critical requests
	// cannot both start and Stop/Close can invalidate a pending decode.
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return fmt.Errorf("audio player is closed")
	}
	if (p.playing || p.loading) && urgency != UrgencyCritical {
		p.mu.Unlock()
		return nil
	}
	p.request++
	request := p.request
	p.loading = true
	p.pendingUrgency = urgency
	p.mu.Unlock()

	// Expand path
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[1:])
		}
	}

	// Check cache first
	p.cacheMutex.RLock()
	cached, ok := p.cache[path]
	p.cacheMutex.RUnlock()

	if ok {
		return p.startBuffer(cached.buffer, request, soundVolume)
	}

	// Load the sound
	buffer, err := p.loadSound(path)
	if err != nil {
		p.mu.Lock()
		if p.request == request {
			p.loading = false
		}
		p.mu.Unlock()
		p.logger.Warn("failed to load sound", "path", path, "error", err)
		return err
	}

	// Cache it
	p.mu.Lock()
	if !p.closed {
		p.cacheMutex.Lock()
		p.cache[path] = &cachedSound{buffer: buffer, path: path}
		p.cacheMutex.Unlock()
	}
	p.mu.Unlock()

	return p.startBuffer(buffer, request, soundVolume)
}

// Stop stops the currently playing sound.
func (p *Player) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopCurrentLocked()
}

// stopCurrentLocked stops the current playback. Must be called with mutex held.
func (p *Player) stopCurrentLocked() {
	p.request++
	p.loading = false
	p.releaseCurrentLocked()
}

// releaseCurrentLocked ends a stream without cancelling a pending replacement.
func (p *Player) releaseCurrentLocked() {
	if p.current != nil {
		close(p.current.cancel)
		p.current.playback.Stop()
		p.current = nil
	}
	p.playing = false
	p.currentUrgency = UrgencyNone
}

// loadSound loads and decodes a sound file into a buffer.
func (p *Player) loadSound(path string) (*beep.Buffer, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open sound file: %w", err)
	}
	defer func() { _ = f.Close() }()

	ext := strings.ToLower(filepath.Ext(path))

	var streamer beep.StreamSeekCloser
	var format beep.Format

	switch ext {
	case ".wav":
		streamer, format, err = wav.Decode(f)
	case ".ogg":
		streamer, format, err = vorbis.Decode(f)
	case ".mp3":
		streamer, format, err = mp3.Decode(f)
	default:
		return nil, fmt.Errorf("unsupported audio format: %s", ext)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to decode sound: %w", err)
	}
	defer func() { _ = streamer.Close() }()

	// Create a buffer and read the entire sound
	buffer := beep.NewBuffer(format)
	buffer.Append(streamer)

	return buffer, nil
}

// playbackSession is owned by the Player mutex. Completion observes identity,
// so a late completion cannot stop a newer critical notification.
type playbackSession struct {
	playback audioPlayback
	cancel   chan struct{}
}

func (p *Player) startBuffer(buffer *beep.Buffer, request uint64, soundVolume float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || p.request != request {
		return nil
	}
	volume := p.volume
	if soundVolume > 0 {
		volume *= soundVolume
	}
	if volume > 1 {
		volume = 1
	}
	if p.legacyVolume && volume > 0 && volume < 1 {
		volume = math.Pow(2, volumeToDecibels(volume))
	}
	// Decode successfully before interrupting the current notification. A bad
	// critical sound must not silence valid playback.
	p.loading = false
	p.releaseCurrentLocked()
	playback, err := p.output.Start(buffer, volume)
	if err != nil {
		p.playing = false
		p.currentUrgency = UrgencyNone
		return err
	}
	p.initialized = true
	p.playing = true
	p.currentUrgency = p.pendingUrgency
	session := &playbackSession{playback: playback, cancel: make(chan struct{})}
	p.current = session
	go p.waitForPlayback(session)
	return nil
}

func (p *Player) waitForPlayback(session *playbackSession) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-session.cancel:
			return
		case <-ticker.C:
			if session.playback.IsPlaying() {
				continue
			}
			// IsPlaying stops at the mixer boundary. Give the configured 50 ms
			// device buffer time to drain before suspending the process context.
			// This grace period also retains non-critical admission protection;
			// Stop and critical preemption can cancel it immediately. Backends
			// expose no exact device-drained signal, so this is best effort.
			drain := time.NewTimer(100 * time.Millisecond)
			select {
			case <-session.cancel:
				drain.Stop()
				return
			case <-drain.C:
			}
			p.mu.Lock()
			if p.current == session {
				if err := session.playback.Err(); err != nil {
					p.logger.Warn("audio playback failed", "error", err)
				}
				p.releaseCurrentLocked()
			}
			p.mu.Unlock()
			return
		}
	}
}

// Preload loads a sound file into the cache for faster playback.
func (p *Player) Preload(path string) error {
	if path == "" {
		return nil
	}

	// Expand path
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[1:])
		}
	}

	// Check if already cached
	p.cacheMutex.RLock()
	_, ok := p.cache[path]
	p.cacheMutex.RUnlock()

	if ok {
		return nil
	}

	// Load the sound
	buffer, err := p.loadSound(path)
	if err != nil {
		return err
	}

	// Cache it
	p.mu.Lock()
	if !p.closed {
		p.cacheMutex.Lock()
		p.cache[path] = &cachedSound{buffer: buffer, path: path}
		p.cacheMutex.Unlock()
	}
	p.mu.Unlock()

	p.logger.Debug("preloaded sound", "path", path)
	return nil
}

// ClearCache clears the sound cache.
func (p *Player) ClearCache() {
	p.cacheMutex.Lock()
	defer p.cacheMutex.Unlock()
	p.cache = make(map[string]*cachedSound)
	p.logger.Debug("sound cache cleared")
}

// InvalidateCache removes a specific path from the cache.
func (p *Player) InvalidateCache(path string) {
	p.cacheMutex.Lock()
	defer p.cacheMutex.Unlock()
	delete(p.cache, path)
}

// Close stops all playback and releases resources.
func (p *Player) Close() {
	p.mu.Lock()
	p.closed = true
	p.stopCurrentLocked()
	p.initialized = false
	p.mu.Unlock()
	p.ClearCache()
}

// volumeToDecibels retains the previous approximation for legacy configuration.
// New linear playback must not use this as a base-2 gain exponent.
func volumeToDecibels(volume float64) float64 {
	if volume <= 0 {
		return -100 // Effectively silent
	}
	// Using log scale: 0.5 = -6dB, 0.25 = -12dB, etc.
	return 20 * log10(volume)
}

// log10 is a simple log base 10 implementation.
func log10(x float64) float64 {
	if x <= 0 {
		return -100
	}
	// Using natural log: log10(x) = ln(x) / ln(10)
	return ln(x) / ln(10)
}

// ln approximates natural logarithm using Taylor series.
func ln(x float64) float64 {
	if x <= 0 {
		return -100
	}
	// For better accuracy, use the standard library indirectly
	// But since we want to avoid importing math, use a simple approximation
	// This is sufficient for volume calculations
	result := 0.0
	y := (x - 1) / (x + 1)
	y2 := y * y
	term := y
	for i := 1; i < 50; i += 2 {
		result += term / float64(i)
		term *= y2
	}
	return 2 * result
}

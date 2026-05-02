// Package audio provides notification sound playback.
package audio

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"
	"github.com/gopxl/beep/v2/vorbis"
	"github.com/gopxl/beep/v2/wav"
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
	volume float64

	// Whether speaker has been initialized
	initialized bool

	// Sample rate for the speaker
	sampleRate beep.SampleRate

	// Sound cache
	cache      map[string]*cachedSound
	cacheMutex sync.RWMutex

	// Playback control
	currentUrgency UrgencyLevel  // Urgency of currently playing sound
	stopChan       chan struct{} // Channel to signal stop
	playing        bool          // Whether a sound is currently playing
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
		sampleRate:     beep.SampleRate(44100),
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
	p.logger.Debug("volume set", "volume", volume)
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

	// Check if we should play based on priority
	p.mu.Lock()
	if p.playing {
		// Critical can always interrupt
		if urgency == UrgencyCritical {
			p.logger.Debug("critical sound interrupting current playback")
			p.stopCurrentLocked()
		} else {
			// Non-critical won't interrupt any playing sound
			p.mu.Unlock()
			p.logger.Debug("sound skipped, another sound is playing", "urgency", urgency)
			return nil
		}
	}
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
		return p.playBufferWithUrgency(cached.buffer, urgency, soundVolume)
	}

	// Load the sound
	buffer, err := p.loadSound(path)
	if err != nil {
		p.logger.Warn("failed to load sound", "path", path, "error", err)
		return err
	}

	// Cache it
	p.cacheMutex.Lock()
	p.cache[path] = &cachedSound{
		buffer: buffer,
		path:   path,
	}
	p.cacheMutex.Unlock()

	return p.playBufferWithUrgency(buffer, urgency, soundVolume)
}

// Stop stops the currently playing sound.
func (p *Player) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopCurrentLocked()
}

// stopCurrentLocked stops the current playback. Must be called with mutex held.
func (p *Player) stopCurrentLocked() {
	if p.playing && p.stopChan != nil {
		close(p.stopChan)
		p.stopChan = nil
		p.playing = false
		p.currentUrgency = UrgencyNone
		// Clear the speaker to stop immediately, then suspend so we go idle.
		// ensureInitialized() will Resume() on the next play.
		speaker.Clear()
		if err := speaker.Suspend(); err != nil {
			p.logger.Debug("speaker suspend returned error", "error", err)
		}
	}
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

// ensureInitialized initializes the speaker on first call, or resumes it from
// an idle suspend on subsequent calls. The pairing Suspend in the playback
// callback keeps the oto/beep mixer goroutine from continuously draining
// samples (and allocating ~700KB/s of mixed silence) when nothing is playing.
func (p *Player) ensureInitialized(sampleRate beep.SampleRate) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.initialized {
		// Resume returns an error only if the audio context is unrecoverable;
		// speaker.Play below will surface anything fatal.
		if err := speaker.Resume(); err != nil {
			p.logger.Debug("speaker resume returned error", "error", err)
		}
		return nil
	}

	// Use a reasonable buffer size for low latency
	bufferSize := sampleRate.N(time.Millisecond * 100)

	if err := speaker.Init(sampleRate, bufferSize); err != nil {
		return fmt.Errorf("failed to initialize speaker: %w", err)
	}

	p.sampleRate = sampleRate
	p.initialized = true
	p.logger.Debug("speaker initialized", "sample_rate", sampleRate)
	return nil
}

// playBufferWithUrgency plays a buffered sound with the specified urgency and volume.
// The soundVolume is relative to the global volume (they are multiplied together).
// If soundVolume is <= 0, uses the player's global volume alone.
func (p *Player) playBufferWithUrgency(buffer *beep.Buffer, urgency UrgencyLevel, soundVolume float64) error {
	if buffer == nil {
		return nil
	}

	if err := p.ensureInitialized(buffer.Format().SampleRate); err != nil {
		return err
	}

	p.mu.Lock()
	globalVolume := p.volume

	// Calculate effective volume: sound volume is relative to global
	// If soundVolume is 0 or less, use global volume alone
	// Otherwise, multiply them together
	var volume float64
	if soundVolume <= 0 {
		volume = globalVolume
	} else {
		volume = globalVolume * soundVolume
	}
	if volume > 1.0 {
		volume = 1.0
	}

	sampleRate := p.sampleRate

	// Set up playback tracking
	p.playing = true
	p.currentUrgency = urgency
	stopChan := make(chan struct{})
	p.stopChan = stopChan
	p.mu.Unlock()

	// Create a streamer from the buffer
	var streamer beep.Streamer = buffer.Streamer(0, buffer.Len())

	// Resample if necessary
	if buffer.Format().SampleRate != sampleRate {
		streamer = beep.Resample(4, buffer.Format().SampleRate, sampleRate, streamer)
	}

	// Apply volume
	if volume < 1.0 {
		streamer = &effects.Volume{
			Streamer: streamer,
			Base:     2,
			Volume:   volumeToDecibels(volume),
			Silent:   volume == 0,
		}
	}

	// Wrap in a callback to mark playback complete
	callback := beep.Callback(func() {
		p.mu.Lock()
		// Only clear if this is still our stopChan (not replaced by another playback)
		isOurs := p.stopChan == stopChan
		if isOurs {
			p.playing = false
			p.currentUrgency = UrgencyNone
			p.stopChan = nil
		}
		idle := isOurs && !p.playing
		p.mu.Unlock()

		if idle {
			if err := speaker.Suspend(); err != nil {
				p.logger.Debug("speaker suspend returned error", "error", err)
			}
		}
	})

	// Play the sound with completion callback
	speaker.Play(beep.Seq(streamer, callback))

	return nil
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
	p.cacheMutex.Lock()
	p.cache[path] = &cachedSound{
		buffer: buffer,
		path:   path,
	}
	p.cacheMutex.Unlock()

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
	defer p.mu.Unlock()

	if p.initialized {
		speaker.Close()
		p.initialized = false
	}

	p.ClearCache()
	p.logger.Debug("audio player closed")
}

// volumeToDecibels converts a linear volume (0-1) to decibels.
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

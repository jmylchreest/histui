package audio

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/jmylchreest/histui/internal/config"
)

// These fixtures exercise the application's extension dispatch and the unchanged
// Beep decoders without opening an audio device. All encode the same 100 ms tone.
func TestCodecCompatibility(t *testing.T) {
	for _, ext := range []string{"wav", "ogg", "mp3"} {
		t.Run(ext, func(t *testing.T) {
			p := NewPlayer(nil)
			buffer, err := p.loadSound(filepath.Join("testdata", "tone."+ext))
			if err != nil {
				t.Fatal(err)
			}
			if buffer.Format().SampleRate != 22050 {
				t.Fatalf("sample rate: %v", buffer.Format())
			}
			// MP3 adds encoder delay and whole-frame padding to this short clip.
			if buffer.Len() < 2000 || buffer.Len() > 4410 {
				t.Fatalf("unexpected decoded length: %d", buffer.Len())
			}
			samples := make([][2]float64, buffer.Len())
			n, _ := buffer.Streamer(0, buffer.Len()).Stream(samples)
			var energy float64
			for _, sample := range samples[:n] {
				if math.IsNaN(sample[0]) || math.IsInf(sample[0], 0) {
					t.Fatal("non-finite PCM")
				}
				energy += sample[0] * sample[0]
			}
			if energy < 0.1 {
				t.Fatal("decoded signal is silent")
			}
			if p.initialized {
				t.Fatal("decoding must not initialize playback")
			}
		})
	}
}

func TestMalformedAudioRejected(t *testing.T) {
	for _, ext := range []string{"wav", "ogg", "mp3", "opus"} {
		t.Run(ext, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "bad."+ext)
			if err := os.WriteFile(path, []byte("not audio"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := NewPlayer(nil).loadSound(path); err == nil {
				t.Fatal("expected decoder error")
			}
		})
	}
}

func TestConcurrentSoundCache(t *testing.T) {
	p := NewPlayer(nil)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if err := p.Preload("testdata/tone.wav"); err != nil {
					t.Error(err)
					return
				}
				p.SetVolume(0.5)
				p.InvalidateCache("testdata/tone.wav")
				p.ClearCache()
			}
		}()
	}
	wg.Wait()
	if p.initialized {
		t.Fatal("cache operations must not initialize playback")
	}
}

func TestDisabledAudioDoesNotAllocatePlayback(t *testing.T) {
	cfg := &config.DaemonConfig{}
	m := NewManager(cfg, nil)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer m.Stop()
	m.SetSoundForUrgency(1, "testdata/tone.wav", 0.5)
	if err := m.PlayForUrgency(1); err != nil {
		t.Fatal(err)
	}
	if m.player != nil || m.watcher != nil {
		t.Fatal("disabled audio allocated playback resources")
	}
}

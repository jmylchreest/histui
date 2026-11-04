package audio

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/jmylchreest/histui/internal/config"
)

// This separate opt-in test uses the existing PulseAudio/PipeWire route with
// zero gain. It requires pactl and never changes the server configuration.
func TestAudioLiveSilent(t *testing.T) {
	if os.Getenv("HISTUI_AUDIO_LIVE_TEST") != "1" {
		t.Skip("requires explicitly authorized live audio route")
	}
	for _, path := range []string{"testdata/tone.wav", "testdata/tone.ogg", "testdata/tone.mp3"} {
		p := NewPlayer(nil)
		p.SetVolume(0)
		if err := p.PlayWithUrgency(path, UrgencyNormal); err != nil {
			p.Close()
			t.Fatal(err)
		}
		p.mu.Lock()
		playback := p.current.playback
		p.mu.Unlock()
		deadline := time.Now().Add(3 * time.Second)
		for {
			p.mu.Lock()
			playing := p.playing
			p.mu.Unlock()
			if !playing || time.Now().After(deadline) {
				if playing {
					p.Close()
					t.Fatal("silent playback did not complete")
				}
				break
			}
			time.Sleep(time.Millisecond)
		}
		p.Close()
		if err := playback.Err(); err != nil {
			t.Fatal(err)
		}
	}
	// Verify this process connected to Pulse, rather than silently falling back
	// to ALSA. Other applications' clients and streams are never modified.
	data, err := exec.Command("pactl", "--format=json", "list", "clients").Output()
	if err != nil {
		t.Fatal(err)
	}
	var clients []struct {
		Properties map[string]string `json:"properties"`
	}
	if err := json.Unmarshal(data, &clients); err != nil {
		t.Fatal(err)
	}
	for _, client := range clients {
		if client.Properties["application.process.id"] == strconv.Itoa(os.Getpid()) {
			processAudioOutput.mu.Lock()
			defer processAudioOutput.mu.Unlock()
			if processAudioOutput.active != 0 {
				t.Fatal("silent test leaked active streams")
			}
			if err := processAudioOutput.context.Err(); err != nil {
				t.Fatal(err)
			}
			t.Log("WAV, Vorbis and MP3 completed at zero gain through the existing Pulse route")
			return
		}
	}
	t.Fatal("test process did not connect to PulseAudio")
}

// Run only against an isolated audio sink, never the user's live speakers:
// HISTUI_AUDIO_TEST=1 PULSE_SERVER=unix:/tmp/nonexistent-histui-pulse \
// ALSA_CONFIG_PATH=/tmp/histui-null-alsa.conf go test -run TestAudioPlaybackLifecycle -timeout 10s ./internal/audio
// The ALSA config must define pcm.!default { type null }.
func TestAudioPlaybackLifecycle(t *testing.T) {
	if os.Getenv("HISTUI_AUDIO_TEST") != "1" {
		t.Skip("requires explicitly configured isolated audio sink")
	}
	p := NewPlayer(nil)
	if err := p.PlayWithUrgency("testdata/tone.wav", UrgencyNormal); err != nil {
		t.Fatal(err)
	}
	waitForPlayback := func() {
		t.Helper()
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			p.mu.Lock()
			playing := p.playing
			p.mu.Unlock()
			if !playing {
				return
			}
			time.Sleep(time.Millisecond)
		}
		t.Fatal("playback did not finish")
	}
	waitForPlayback()
	if err := p.PlayWithUrgency("testdata/tone.ogg", UrgencyCritical); err != nil {
		t.Fatal(err)
	}
	waitForPlayback()
	p.Stop()
	p.Close()
	// Audio hot-reload currently destroys and recreates Player. This must work
	// after at least one sound has initialized the process-wide output context.
	replacement := NewPlayer(nil)
	defer replacement.Close()
	if err := replacement.PlayWithUrgency("testdata/tone.mp3", UrgencyNormal); err != nil {
		t.Fatalf("audio disable/re-enable failed: %v", err)
	}
}

func TestAudioManagerReloadLifecycle(t *testing.T) {
	if os.Getenv("HISTUI_AUDIO_TEST") != "1" {
		t.Skip("requires explicitly configured isolated audio sink")
	}
	enabled := &config.DaemonConfig{Audio: config.AudioConfig{Enabled: true, Volume: 80}}
	disabled := &config.DaemonConfig{}
	m := NewManager(enabled, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer m.Stop()
	m.SetSoundForUrgency(1, "testdata/tone.wav", 0.5)
	for i := 0; i < 10; i++ {
		if err := m.PlayForUrgency(1); err != nil {
			t.Fatal(err)
		}
		m.UpdateConfig(disabled)
		if m.player != nil || m.watcher != nil {
			t.Fatal("disabled manager retained resources")
		}
		processAudioOutput.mu.Lock()
		active := processAudioOutput.active
		processAudioOutput.mu.Unlock()
		if active != 0 {
			t.Fatalf("disabled manager retained %d streams", active)
		}
		m.UpdateConfig(enabled)
	}
	if err := m.PlayForUrgency(1); err != nil {
		t.Fatal(err)
	}
	m.StopPlayback()
}

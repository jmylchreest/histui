package audio

import (
	"encoding/binary"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gopxl/beep/v2"
)

type fakePlayback struct {
	playing atomic.Bool
	stops   atomic.Int32
}

func (f *fakePlayback) IsPlaying() bool { return f.playing.Load() }
func (f *fakePlayback) Err() error      { return nil }
func (f *fakePlayback) Stop()           { f.stops.Add(1); f.playing.Store(false) }

type fakeOutput struct {
	mu      sync.Mutex
	streams []*fakePlayback
	volumes []float64
}

func (f *fakeOutput) Start(_ *beep.Buffer, volume float64) (audioPlayback, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := &fakePlayback{}
	p.playing.Store(true)
	f.streams = append(f.streams, p)
	f.volumes = append(f.volumes, volume)
	return p, nil
}

func TestPriorityStopAndStaleCompletion(t *testing.T) {
	output := &fakeOutput{}
	p := NewPlayer(nil)
	p.output = output
	defer p.Close()
	p.SetVolume(0.5)
	if err := p.PlayWithUrgency("testdata/tone.wav", UrgencyNormal, 0.4); err != nil {
		t.Fatal(err)
	}
	first := p.current
	if err := p.PlayWithUrgency("testdata/tone.ogg", UrgencyLow); err != nil {
		t.Fatal(err)
	}
	if len(output.streams) != 1 {
		t.Fatal("non-critical playback interrupted active stream")
	}
	if output.volumes[0] != 0.2 {
		t.Fatalf("combined volume: %v", output.volumes[0])
	}
	if err := p.PlayWithUrgency("testdata/tone.mp3", UrgencyCritical); err != nil {
		t.Fatal(err)
	}
	if len(output.streams) != 2 || output.streams[0].stops.Load() != 1 {
		t.Fatal("critical stream did not replace previous stream")
	}
	// A completion watcher delayed until after replacement must not stop it.
	p.waitForPlayback(first)
	if !output.streams[1].IsPlaying() || output.streams[1].stops.Load() != 0 {
		t.Fatal("stale completion stopped replacement")
	}
	p.Stop()
	p.Stop()
	if p.playing || output.streams[1].stops.Load() != 1 {
		t.Fatal("Stop must be idempotent")
	}
}

func TestCompletionReleasesPlayback(t *testing.T) {
	output := &fakeOutput{}
	p := NewPlayer(nil)
	p.output = output
	defer p.Close()
	if err := p.PlayWithUrgency("testdata/tone.wav", UrgencyNormal); err != nil {
		t.Fatal(err)
	}
	output.streams[0].playing.Store(false)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		p.mu.Lock()
		playing := p.playing
		p.mu.Unlock()
		if !playing {
			if output.streams[0].stops.Load() != 1 {
				t.Fatal("completion did not release stream")
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("completion failed to release playback")
}

func TestInvalidCriticalSoundPreservesCurrentPlayback(t *testing.T) {
	output := &fakeOutput{}
	p := NewPlayer(nil)
	p.output = output
	defer p.Close()
	if err := p.PlayWithUrgency("testdata/tone.wav", UrgencyNormal); err != nil {
		t.Fatal(err)
	}
	first := p.current
	if err := p.PlayWithUrgency("testdata/missing.wav", UrgencyCritical); err == nil {
		t.Fatal("expected missing file error")
	}
	if p.current != first || !p.playing || p.loading || output.streams[0].stops.Load() != 0 {
		t.Fatal("invalid critical sound disturbed valid current playback")
	}
	if err := p.PlayWithUrgency("testdata/tone.ogg", UrgencyNormal); err != nil {
		t.Fatal(err)
	}
	if len(output.streams) != 1 {
		t.Fatal("failed critical request lost noncritical admission guard")
	}
}

func TestCompletionDoesNotCancelPendingReplacement(t *testing.T) {
	p := NewPlayer(nil)
	output := &fakeOutput{}
	p.output = output
	defer p.Close()
	if err := p.PlayWithUrgency("testdata/tone.wav", UrgencyNormal); err != nil {
		t.Fatal(err)
	}
	buffer, err := p.loadSound("testdata/tone.ogg")
	if err != nil {
		t.Fatal(err)
	}
	p.mu.Lock()
	p.request++
	request := p.request
	p.loading = true
	p.pendingUrgency = UrgencyCritical
	p.releaseCurrentLocked()
	p.mu.Unlock()
	if err := p.startBuffer(buffer, request, 1); err != nil {
		t.Fatal(err)
	}
	if len(output.streams) != 2 || !p.playing || p.loading || p.currentUrgency != UrgencyCritical {
		t.Fatal("completion discarded the pending replacement")
	}
}

func TestStopInvalidatesPendingDecode(t *testing.T) {
	p := NewPlayer(nil)
	output := &fakeOutput{}
	p.output = output
	buffer, err := p.loadSound("testdata/tone.wav")
	if err != nil {
		t.Fatal(err)
	}
	request := p.request
	p.Stop()
	if err := p.startBuffer(buffer, request, 1); err != nil {
		t.Fatal(err)
	}
	if len(output.streams) != 0 {
		t.Fatal("stale decode started playback after Stop")
	}
}

func TestConcurrentPlaybackStopClose(t *testing.T) {
	p := NewPlayer(nil)
	output := &fakeOutput{}
	p.output = output
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_ = p.PlayWithUrgency("testdata/tone.wav", UrgencyLevel(j%3))
				p.Stop()
			}
		}()
	}
	wg.Add(1)
	go func() { defer wg.Done(); p.Close() }()
	wg.Wait()
	p.Close()
	for _, stream := range output.streams {
		if stream.IsPlaying() || stream.stops.Load() != 1 {
			t.Fatal("stream leaked or stopped more than once")
		}
	}
}

type fixedSamples struct{ samples [][2]float64 }

func (s *fixedSamples) Stream(dst [][2]float64) (int, bool) {
	n := copy(dst, s.samples)
	s.samples = s.samples[n:]
	return n, n != 0
}
func (*fixedSamples) Err() error { return nil }

func TestPCMReaderPreservesChannelsAndClips(t *testing.T) {
	r := &pcmReader{stream: &fixedSamples{samples: [][2]float64{{1, -1}, {0.5, -0.5}, {2, -2}}}}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	want := []int16{32767, -32767, 16383, -16383, 32767, -32767}
	if len(data) != len(want)*2 {
		t.Fatalf("PCM length: %d", len(data))
	}
	for i, sample := range want {
		if got := int16(binary.LittleEndian.Uint16(data[i*2:])); got != sample {
			t.Fatalf("sample %d: got %d want %d", i, got, sample)
		}
	}
}

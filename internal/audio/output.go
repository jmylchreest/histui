package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
)

type audioPlayback interface {
	IsPlaying() bool
	Err() error
	Stop()
}

type audioOutput interface {
	Start(*beep.Buffer, float64) (audioPlayback, error)
}

// Oto permits one context per process. Retain it across audio config reloads,
// suspend it while idle, and give every playback an independent stream.
// This value does not initialize a device until Start is called.
var processAudioOutput otoOutput

type otoOutput struct {
	mu         sync.Mutex
	context    *oto.Context
	sampleRate beep.SampleRate
	active     int
	initErr    error
}

func (o *otoOutput) Start(buffer *beep.Buffer, volume float64) (audioPlayback, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.initErr != nil {
		return nil, o.initErr
	}
	if o.context == nil {
		var ready chan struct{}
		o.sampleRate = buffer.Format().SampleRate
		o.context, ready, o.initErr = oto.NewContext(&oto.NewContextOptions{
			SampleRate: int(o.sampleRate), ChannelCount: 2,
			Format: oto.FormatSignedInt16LE, BufferSize: 50 * time.Millisecond,
			ApplicationName: "histuid",
		})
		if o.initErr != nil {
			return nil, o.initErr
		}
		<-ready
	}
	if err := o.context.Err(); err != nil {
		return nil, err
	}
	var stream beep.Streamer = buffer.Streamer(0, buffer.Len())
	if buffer.Format().SampleRate != o.sampleRate {
		stream = beep.Resample(4, buffer.Format().SampleRate, o.sampleRate, stream)
	}
	// Keep the existing Beep gain curve, including its zero-volume behavior.
	if volume < 1 {
		stream = &effects.Volume{Streamer: stream, Base: 2, Volume: volumeToDecibels(volume), Silent: volume == 0}
	}
	player := o.context.NewPlayer(&pcmReader{stream: stream})
	player.SetBufferSize(o.sampleRate.N(50*time.Millisecond) * 4)
	if o.active == 0 {
		if err := o.context.Resume(); err != nil {
			return nil, err
		}
	}
	o.active++
	player.Play()
	return &otoPlayback{player: player, output: o}, nil
}

type otoPlayback struct {
	player *oto.Player
	output *otoOutput
	once   sync.Once
}

func (p *otoPlayback) IsPlaying() bool { return p.player.IsPlaying() }
func (p *otoPlayback) Err() error      { return p.player.Err() }
func (p *otoPlayback) Stop() {
	p.once.Do(func() {
		// Oto Player.Close is a no-op. Explicitly stop consuming the source.
		p.player.PauseAndStopReading()
		p.output.mu.Lock()
		defer p.output.mu.Unlock()
		p.output.active--
		if p.output.active == 0 {
			_ = p.output.context.Suspend()
		}
	})
}

// pcmReader converts Beep stereo samples to Oto's interleaved signed 16-bit PCM.
// Its reusable buffer bounds allocation independently of notification length.
type pcmReader struct {
	stream  beep.Streamer
	samples [512][2]float64
}

func (r *pcmReader) Read(dst []byte) (int, error) {
	if len(dst) == 0 {
		return 0, nil
	}
	if len(dst)%4 != 0 {
		return 0, fmt.Errorf("audio read size is not frame aligned")
	}
	frames := min(len(dst)/4, len(r.samples))
	n, ok := r.stream.Stream(r.samples[:frames])
	for i, sample := range r.samples[:n] {
		for channel, value := range sample {
			if math.IsNaN(value) {
				value = 0
			}
			value = max(-1, min(1, value))
			binary.LittleEndian.PutUint16(dst[i*4+channel*2:], uint16(int16(value*32767)))
		}
	}
	if !ok {
		if err := r.stream.Err(); err != nil {
			return n * 4, err
		}
		return n * 4, io.EOF
	}
	return n * 4, nil
}

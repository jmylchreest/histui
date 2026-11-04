package audio

import (
	"context"
	"encoding/binary"
	"fmt"
	"github.com/gopxl/beep/v2"
	"github.com/jmylchreest/histui/internal/config"
	"math"
	"testing"
)

func TestLinearVolumePCM(t *testing.T) {
	for _, volume := range []float64{0, .01, .2, .5, 1} {
		t.Run(fmt.Sprint(volume), func(t *testing.T) {
			source := beep.StreamerFunc(func(s [][2]float64) (int, bool) {
				for i := range s {
					s[i] = [2]float64{.25, -.25}
				}
				return len(s), true
			})
			r := pcmReader{stream: scaleVolume(source, volume)}
			data := make([]byte, 4)
			if _, err := r.Read(data); err != nil {
				t.Fatal(err)
			}
			want := int16(.25 * volume * 32767)
			if got := int16(binary.LittleEndian.Uint16(data)); got != want {
				t.Fatalf("gain %v: PCM %d, want %d", volume, got, want)
			}
			if got := int16(binary.LittleEndian.Uint16(data[2:])); got != -want {
				t.Fatalf("negative PCM %d, want %d", got, -want)
			}
		})
	}
}

func TestConfigVolumeCompatibilityAndReloadMute(t *testing.T) {
	cfg := config.DefaultDaemonConfig()
	cfg.Audio.Volume = 80
	cfg.Audio.VolumeScale = "legacy"
	m := NewManager(cfg, nil)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer m.Stop()
	output := &fakeOutput{}
	m.player.output = output
	m.SetSoundForUrgency(1, "testdata/tone.wav", .8)
	if err := m.PlayForUrgency(1); err != nil {
		t.Fatal(err)
	}
	want := math.Pow(2, volumeToDecibels(.8*.8))
	if math.Abs(output.volumes[0]-want) > 1e-12 {
		t.Fatalf("legacy gain %v want %v", output.volumes[0], want)
	}
	m.StopPlayback()
	cfg2 := *cfg
	cfg2.Audio.VolumeScale = "linear"
	cfg2.Audio.Volume = 20
	m.UpdateConfig(&cfg2)
	if err := m.PlayForUrgency(1); err != nil {
		t.Fatal(err)
	}
	if math.Abs(output.volumes[1]-.16) > 1e-12 {
		t.Fatalf("linear gain %v", output.volumes[1])
	}
	m.StopPlayback()
	cfg3 := cfg2
	cfg3.Audio.Volume = 0
	m.UpdateConfig(&cfg3)
	if err := m.PlayForUrgency(1); err != nil {
		t.Fatal(err)
	}
	if output.volumes[2] != 0 {
		t.Fatalf("zero volume must mute: %v", output.volumes[2])
	}
}

func TestNewDefaultDoesNotIncreaseNormalSoundGain(t *testing.T) {
	cfg := config.DefaultDaemonConfig()
	old := math.Pow(2, volumeToDecibels(.8))
	got := float64(cfg.Audio.Volume) / 100
	if cfg.Audio.VolumeScale != "linear" || got > old || old-got > .01 {
		t.Fatalf("new default %+v vs old gain %v", cfg.Audio, old)
	}
}

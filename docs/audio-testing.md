# Audio validation

Run commands from the repository root. Audio retains Beep's WAV, MP3 and Ogg
Vorbis decoders and uses Oto for output. Linux output builds without CGO; ALSA
fallback still loads the system ALSA library at runtime. The Go race detector
itself requires CGO and a C compiler.

## Automated checks

These checks do not open an audio device unless explicitly enabled:

```sh
CGO_ENABLED=0 go test -v -timeout 60s ./internal/audio
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
  go test -c -o /tmp/histui-audio-arm64.test ./internal/audio
```

Exercise actual output and manager disable/re-enable against ALSA's null sink
(install the system ALSA library first). The explicit Pulse socket prevents
connection to an existing PulseAudio or PipeWire server; the ALSA configuration
routes fallback to the null sink, not hardware:

```sh
audio_test_dir=$(mktemp -d)
printf 'pcm.!default { type null }\n' > "$audio_test_dir/alsa.conf"
CGO_ENABLED=1 HISTUI_AUDIO_TEST=1 \
  PULSE_SERVER="unix:$audio_test_dir/nonexistent-pulse" \
  ALSA_CONFIG_PATH="$audio_test_dir/alsa.conf" \
  go test -race -count=3 -v -timeout 60s ./internal/audio
rm -r "$audio_test_dir"
```

CI runs all three checks. Unit tests cover codec decoding, priority admission,
invalid critical replacements, stale completion, cancellation, concurrent
play/stop/close, cache operations, and disabled audio initialization.

## Existing PulseAudio or PipeWire route

With permission to use the current session's audio server, run this separate
opt-in test. It requires `pactl`, plays all codecs at zero gain, closes and
recreates players, and verifies this test process connected to PulseAudio.
It does not change server configuration or other applications' streams.
Run without the isolated test's `PULSE_SERVER` or `ALSA_CONFIG_PATH` overrides:

```sh
CGO_ENABLED=0 HISTUI_AUDIO_LIVE_TEST=1 \
  go test -v -run '^TestAudioLiveSilent$' -count=3 -timeout 30s ./internal/audio
```

## Remaining acceptance checks

- Human listening is needed to judge audible quality, volume parity, clipping,
  critical preemption, and sound tails on representative devices.
- ARM64 cross-compilation does not establish ARM64 runtime correctness.
- ALSA null output does not establish compatibility with physical ALSA devices.
- Audio-server loss and recovery need separate acceptance testing. Oto retains
  one process-wide context; a permanent backend failure may require a restart.

On completion, playback waits a best-effort 100 ms for the configured 50 ms
device buffer before suspending output. Noncritical requests remain suppressed
during this grace period; Stop and critical replacement cancel it. Oto exposes
no precise hardware-drained signal, and stopping cannot retract samples already
queued by the device. After first playback, the process context remains
suspended when idle; initially disabled audio does not initialize it.

## Local acceptance (2026-11-04)

CGO-disabled unit tests, three ALSA-null race runs and three silent live-route
codec runs passed. The user confirmed WAV, Vorbis and MP3 tones, a source-level
quiet/louder pair, critical interruption and stop/restart sounded clean through
the current PipeWire/PulseAudio output. Synthetic tones had at most 5% peak
amplitude, with player volume 1.0. This is one device/session, not broad hardware
qualification or an old/new backend comparison.

The initial listening attempt at player volume 0.2 was inaudible. The existing
volume curve uses Beep Base=2 with 20*log10(volume) as its exponent, giving about
0.000062 amplitude gain at 0.2 rather than linear 0.2. Combined with quiet source
material, the new signed-16-bit PCM conversion can round the result to silence.
The same gain curve exists in the previous backend and is deliberately preserved
by this backend migration; the corrected listening test bypassed it. Configured
volume-scale correctness remains a separate issue requiring regression coverage
and a decision about changing existing users' perceived volume.

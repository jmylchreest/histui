Synthetic 440 Hz, 100 ms, mono 22050 Hz tone generated for these tests.
No third-party recordings are used.

Generation (ffmpeg is not needed to run tests):
ffmpeg -f lavfi -i sine=frequency=440:sample_rate=22050:duration=0.1 -ac 1 -c:a pcm_s16le tone.wav
ffmpeg -i tone.wav -c:a libmp3lame -b:a 64k tone.mp3
ffmpeg -i tone.wav -c:a libvorbis -q:a 3 tone.ogg

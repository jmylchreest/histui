# Shared Linux FFI runtime

Oto 3.5.1 imports Ebitengine purego; Codeberg GTK imports its callback-recycling
fork. Linking both in one CGO-disabled binary fails on duplicate assembly symbols
and would introduce competing fake-cgo runtimes. This small Linux-only bridge
forwards Oto's four dynamic-library APIs and flags to the pinned Codeberg runtime.
It contains no playback or codec implementation. `objc` is only a module-graph
placeholder; non-Linux platforms are deliberately unsupported.

The `github.com/ebitengine/purego` version in the root module describes Oto's API
dependency, not the code executed: the replacement routes calls to
`codeberg.org/puregotk/purego` at the root module's explicit pin. Security reviews
must examine that actual implementation. Do not treat Ebitengine updates as updates
to this bridge's implementation. Revisit/remove the bridge when the two bindings
can share a maintained upstream runtime with callback reclamation.

A new API used by Oto fails compilation rather than silently acquiring a second
runtime. Upgrade checks must include Linux amd64/arm64 compilation, codec and
output lifecycle tests, GTK callbacks and daemon integration. The isolated ALSA
null-sink tests exercise WAV, MP3, Vorbis, stop/restart and configuration reload.

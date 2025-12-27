# Implementation Plan: histuid - Wayland Notification Daemon

**Branch**: `002-wayland-notification-daemon` | **Date**: 2025-12-27 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/002-wayland-notification-daemon/spec.md`

## Summary

histuid is a Wayland-native notification daemon that implements the org.freedesktop.Notifications D-Bus interface, displays styled popup windows using GTK4/libadwaita on Wayland layer-shell surfaces, and shares a unified history store with histui. The daemon supports CSS theming, animated images, per-urgency audio notifications, Do Not Disturb mode, and hot-reload via file watching.

## Technical Context

**Language/Version**: Go 1.21+ (required for stdlib `slog`, consistent with existing histui)
**Primary Dependencies**:
- Existing: Cobra (CLI), BubbleTea + Lipgloss (TUI), testify (assertions), fsnotify (file watching), go-toml/v2 (config)
- New:
  - godbus/dbus/v5 (D-Bus service)
  - gotk4 + gotk4-layer-shell (Wayland layer-shell popups)
  - gotk4-adwaita (libadwaita for modern GNOME styling)
  - GdkPixbufAnimation (animated GIF support via gotk4)
  - gopxl/beep (Audio playback)

**Storage**: JSONL file (shared with histui at `~/.local/share/histui/`)
**Testing**: go test + testify
**Target Platform**: Linux (Wayland compositors with wlr-layer-shell support: Hyprland, Sway)
**Project Type**: Single project (extension of existing histui binary + new daemon binary)
**Performance Goals**:
- Notification display: <100ms from D-Bus signal to popup visible
- Persist to history: <50ms from display
- State sync: <200ms between histui/histuid
- Popup animations: 60fps

**Constraints**:
- <100MB memory with 1000 notifications in session
- Daemon startup: <1 second to claim D-Bus name
- CGO_ENABLED required for Wayland/WebView bindings (deviation from histui)

**Scale/Scope**:
- 50 simultaneous notifications max
- Rate limiting for high-frequency scenarios (100+ notifications/sec)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Pluggable Adapter Architecture | PASS | histuid becomes a new stream adapter (continuous injection) per ADR-002 |
| II. Idiomatic Go | PASS | Error handling via explicit returns, context propagation, channels for change notification |
| III. Clean Code Structure | PASS | New packages: `cmd/histuid/`, `internal/daemon/`, `internal/display/`, `internal/dbus/`, `internal/audio/` |
| IV. Test-First Development | PASS | Unit tests for D-Bus interface parsing, store integration, config parsing |
| V. Security Considerations | PASS | Validate D-Bus input, sanitize notification content in WebView, restrictive file permissions |
| VI. User Experience Focus | PASS | Fast startup, sensible defaults, auto-detection, XDG paths |
| VII. Structured Logging | PASS | slog for diagnostics, no emojis, stderr for logs |
| VIII. Build Standards | DEVIATION | CGO_ENABLED=1 required for Wayland/WebView bindings (vs CGO_ENABLED=0 for histui) |

**Gate Assessment**: PASS with justified deviation on CGO requirement.

## Project Structure

### Documentation (this feature)

```text
specs/002-wayland-notification-daemon/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (D-Bus interface spec)
└── tasks.md             # Phase 2 output
```

### Source Code (repository root)

```text
cmd/
├── histui/              # Existing CLI (extended with set, dnd commands)
│   ├── main.go
│   ├── root.go
│   ├── get.go
│   ├── set.go           # NEW: histui set command
│   ├── dnd.go           # NEW: histui dnd command
│   ├── status.go        # MODIFIED: Include DnD state
│   └── ...
└── histuid/             # NEW: Notification daemon
    └── main.go

internal/
├── model/               # Existing notification model
│   └── notification.go  # EXTENDED: Display state fields
├── store/               # Existing store (shared)
│   ├── store.go
│   ├── persistence.go
│   └── state.go         # NEW: DnD state persistence
├── config/              # Existing + extended
│   ├── config.go        # histui config
│   └── daemon.go        # NEW: histuid config (TOML)
├── adapter/
│   ├── input/
│   │   ├── dunst.go     # Existing
│   │   ├── stdin.go     # Existing
│   │   └── histuid.go   # NEW: Stream adapter for daemon mode
│   └── output/          # Existing formatters
│       ├── ids.go       # NEW: --format ids output
│       └── ...
├── core/
│   ├── filter.go        # EXTENDED: Rich filter parsing
│   └── ...
├── daemon/              # NEW: histuid daemon logic
│   ├── daemon.go        # Main daemon orchestration
│   ├── lifecycle.go     # Startup, shutdown, signals
│   └── hotreload.go     # File watching for config/theme/audio
├── dbus/                # NEW: D-Bus interface
│   ├── server.go        # org.freedesktop.Notifications implementation
│   ├── types.go         # D-Bus notification types
│   └── signals.go       # Signal emission
├── display/             # NEW: Wayland popup display
│   ├── manager.go       # Popup lifecycle management
│   ├── popup.go         # Individual popup window (GTK4/libadwaita)
│   ├── widgets.go       # Notification widget construction
│   ├── animated.go      # GdkPixbufAnimation paintable wrapper
│   └── layout.go        # Stacking and positioning
├── theme/               # NEW: CSS theming
│   ├── loader.go        # Theme file loading
│   ├── watcher.go       # Hot-reload support
│   └── default.go       # Embedded default theme
└── audio/               # NEW: Sound playback
    ├── player.go        # Audio playback interface
    ├── loader.go        # Sound file loading/caching
    └── watcher.go       # Hot-reload support
```

**Structure Decision**: Single project with shared internal packages between histui CLI and histuid daemon. The daemon shares the store, model, and config packages with the existing CLI.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| CGO_ENABLED=1 | GTK4/libadwaita and layer-shell require C bindings | Pure Go alternatives don't exist for Wayland GUI toolkits |
| Separate histuid binary | Daemon must run continuously and claim D-Bus name | Adding daemon mode to histui would complicate CLI ergonomics and make binary size larger for non-daemon users |

## Research Completed (Phase 0)

All technical unknowns have been resolved. See [research.md](./research.md) for full details.

| Unknown | Decision | Rationale |
|---------|----------|-----------|
| Wayland Layer-Shell | gotk4 + gotk4-layer-shell | Production-ready GTK4 bindings with layer-shell support |
| UI Rendering | GTK4 + libadwaita (no WebKit) | Reduced attack surface, native CSS theming, modern GNOME styling |
| Animated Images | GdkPixbufAnimation | Native GTK4 support for animated GIF/APNG |
| Audio Playback | gopxl/beep | Pure Go, WAV/OGG/MP3, volume control, non-blocking |
| D-Bus Service | godbus/dbus/v5 | Mature, pure Go, full server/signal support |
| File Watching | fsnotify | Already used in histui, works for hot-reload |

### CGO Justification

CGO_ENABLED=1 is required because:
- No pure-Go Wayland layer-shell implementation exists
- GTK4/libadwaita are C libraries with no pure-Go alternatives
- GdkPixbuf (for animated images) requires C bindings

This is a justified deviation from the constitution's CGO_ENABLED=0 preference. The histui CLI binary remains CGO-free; only histuid requires CGO.

**Security note**: By choosing GTK4/libadwaita over WebKit, we significantly reduce the attack surface - no browser engine, no JavaScript execution, no web content parsing.

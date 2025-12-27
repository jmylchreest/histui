---
title: Configuration
description: histuid configuration file reference
sidebar_position: 1
---

# Configuration

histuid is configured via `~/.config/histui/histuid.toml`.

## Minimal Configuration

histuid works with no configuration file.
All settings have sensible defaults.

## Configuration File Location

```
~/.config/histui/histuid.toml
```

Or set a custom path:

```bash
histuid --config /path/to/config.toml
```

## Full Reference

### [display]

Controls notification popup appearance and behavior.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `position` | string | "top-right" | Popup position on screen |
| `width` | int | 350 | Popup width in pixels |
| `margin` | int | 10 | Margin from screen edge |
| `gap` | int | 5 | Gap between notifications |

### [timeouts]

Controls how long notifications are displayed by urgency level.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `low` | duration | "0" | Timeout for low urgency (default: honor client) |
| `normal` | duration | "0" | Timeout for normal urgency (default: honor client) |
| `critical` | duration | "never" | Timeout for critical urgency |
| `fallback` | duration | "10s" | Fallback timeout when client says "server decides" |

**Timeout Values:**

| Value | Meaning |
|-------|---------|
| `"0"` or `"0s"` | Honor the client's requested timeout |
| `"never"` or `"-1"` | Notification never expires (must be dismissed) |
| Positive duration (e.g., `"5s"`, `"30s"`, `"2m"`) | Override with this timeout |

**Duration Format:**

Durations can be specified as:
- `"5s"` - 5 seconds
- `"500ms"` - 500 milliseconds
- `"2m"` - 2 minutes
- `"1h30m"` - 1 hour 30 minutes
- `"5000"` - 5000 milliseconds (integer)

**Client Timeout Behavior:**

When set to `"0"` (honor client), the daemon respects what the sending application requested:
- If the client requests `-1` (server decides), the daemon uses the `fallback` timeout (default 10s, critical always uses 0/never)
- If the client requests `0` (never expire), the notification stays until dismissed
- If the client requests a positive value, that timeout is used

### [history]

Controls notification history persistence.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `max_count` | int | 1000 | Maximum notifications to store |
| `path` | string | XDG data dir | History database location |

### [behavior]

Controls daemon behavior.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `monitor_mode` | bool | false | Run alongside another daemon |

### [audio]

Controls notification sounds.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `enabled` | bool | true | Enable notification sounds |
| `volume` | int | 80 | Global volume (0-100) |

**Audio Behavior:**

- Sounds are defined in theme manifests (see [Theming](/docs/histuid/theming))
- Only one sound plays at a time (newer sounds are skipped unless critical)
- Critical notifications can interrupt any playing sound
- Per-theme volumes are multiplied with the global volume

**Supported Formats:**

| Format | Extension | Notes |
|--------|-----------|-------|
| WAV | `.wav` | PCM only (16-bit signed little-endian). IEEE Float format is **not** supported |
| Ogg Vorbis | `.ogg` | Fully supported |
| MP3 | `.mp3` | Fully supported |

**Command-line Flags:**

```bash
# Disable all sounds (overrides theme and config)
histuid --no-audio

# Override global volume (0.0 to 1.0)
histuid --volume 0.5
```

## Example Configuration

```toml
[display]
position = "top-right"
max_visible = 5
offset_x = 10
offset_y = 10

[timeouts]
# Honor whatever the application requests (default for low/normal)
low = "0"
normal = "0"

# Critical notifications never expire (default)
critical = "never"

# Fallback timeout when client says "server decides" (-1)
fallback = "10s"

[behavior]
stack_duplicates = true
pause_on_hover = true

[audio]
enabled = true
volume = 80
```

## Icon Aliases

Icon aliases are configured in a separate file for easy sharing:

```
~/.config/histui/icon-aliases.toml
```

This maps application names to standard icon names:

```toml
# ~/.config/histui/icon-aliases.toml

[aliases]
zapzap = "whatsapp"
telegram-desktop = "telegram"
firefox-esr = "firefox"
vesktop = "discord"
signal-desktop = "signal"
```

Built-in aliases are provided for common apps. User aliases take precedence.
See [Advanced Theming](/docs/histuid/theming/advanced#icon-resolution) for more details.

## See Also

- [Theming](/docs/histuid/theming) - Customize appearance
- [Monitor Mode](/docs/histuid/monitor-mode) - Run with dunst

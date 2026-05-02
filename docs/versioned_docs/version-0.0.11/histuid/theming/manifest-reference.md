---
title: Manifest Reference
description: Complete manifest.toml configuration reference
sidebar_position: 7
---

# Manifest Reference

The `manifest.toml` file defines theme metadata, audio configuration, and icon settings. It's optional - themes without a manifest inherit defaults.

## File Location

Manifest files belong in theme directories:

```
~/.config/histui/themes/mytheme/
├── manifest.toml   # Theme configuration
├── theme.css       # CSS styling
├── layout.xml      # Widget layout (optional)
└── sounds/         # Audio files (optional)
    └── notify.ogg
```

## File Format

Manifest files use TOML format. The filename must be `manifest.toml`.

## Complete Reference

```toml
# Theme Metadata
name = "My Theme"
description = "A custom notification theme with sounds"
author = "Your Name"
version = "1.0.0"

# Icon Configuration
[icon]
size = 48    # Default icon size in pixels

# Audio Configuration
[audio.low]
path = ""                    # No sound for low urgency
volume = 0.5                 # Volume level 0.0-1.0
repeat_count = -1            # Play once (-1 = no repeat)

[audio.normal]
path = "sounds/notify.ogg"   # Sound file path
volume = 0.8                 # 80% volume
repeat_count = -1            # Play once

[audio.critical]
path = "sounds/alert.ogg"    # Critical notification sound
volume = 1.0                 # Full volume
repeat_count = 0             # Repeat until dismissed
repeat_delay = "10s"         # 10 seconds between repeats
```

## Metadata Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Display name for the theme |
| `description` | string | Brief description |
| `author` | string | Theme author name |
| `version` | string | Version string (e.g., "1.0.0") |

These are informational and displayed when listing themes.

## Icon Configuration

```toml
[icon]
size = 48
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `size` | int | 48 | Default icon size in pixels |

This sets the default icon size. It can be overridden in `layout.xml`:

```xml
<!-- Override manifest size for this layout -->
<icon size="32" />
```

## Audio Configuration

Audio is configured per urgency level:

```toml
[audio.low]
# ... low urgency settings

[audio.normal]
# ... normal urgency settings

[audio.critical]
# ... critical urgency settings
```

### Audio Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `path` | string | "" | Path to audio file |
| `volume` | float | 1.0 | Volume level (0.0 to 1.0) |
| `repeat_count` | int | -1 | Repeat behavior |
| `repeat_delay` | duration | "10s" | Delay between repeats |

### Path Resolution

Paths can be:

- **Relative**: Relative to the theme directory
  ```toml
  path = "sounds/notify.ogg"  # ~/.config/histui/themes/mytheme/sounds/notify.ogg
  ```

- **Absolute**: Full path on the filesystem
  ```toml
  path = "/usr/share/sounds/notify.ogg"
  ```

- **Empty**: No sound for this urgency level
  ```toml
  path = ""
  ```

### Repeat Behavior

The `repeat_count` field controls sound repetition:

| Value | Behavior |
|-------|----------|
| `-1` | Play once, no repeat (default) |
| `0` | Repeat indefinitely until notification is dismissed |
| `N` | Repeat exactly N times |

For repeating sounds, `repeat_delay` sets the pause between plays:

```toml
[audio.critical]
path = "sounds/alert.ogg"
volume = 1.0
repeat_count = 0           # Repeat until dismissed
repeat_delay = "10s"       # 10 second gap between plays
```

Duration format examples: `"5s"`, `"30s"`, `"1m"`, `"500ms"`

### Supported Audio Formats

| Format | Extension | Notes |
|--------|-----------|-------|
| WAV | `.wav` | PCM only (16-bit signed little-endian) |
| Ogg Vorbis | `.ogg` | Fully supported |
| MP3 | `.mp3` | Fully supported |

:::warning WAV Format
IEEE Float WAV files are not supported. Convert to PCM:
```bash
ffmpeg -i input.wav -acodec pcm_s16le -ar 44100 -ac 2 output.wav
```
:::

## Manifest Merging

When you create a partial theme, your manifest is merged with the default:

- **Your values take precedence** for fields you specify
- **Default values fill in** for fields you omit

### Example: Override Only Critical Sound

```toml
# Only specify what you want to change
[audio.critical]
path = "sounds/alarm.ogg"
volume = 1.0
repeat_count = 3
repeat_delay = "5s"

# audio.low and audio.normal are inherited from default
```

### Merge Rules by Field

| Field | Behavior |
|-------|----------|
| `name`, `description`, `author`, `version` | Your value if set, otherwise default |
| `icon.size` | Your value if non-zero, otherwise default (48) |
| `audio.*.path` | Your value if non-empty, otherwise default |
| `audio.*.volume` | Your value if non-zero, otherwise default |
| `audio.*.repeat_count` | Complex - see below |
| `audio.*.repeat_delay` | Your value if set, otherwise default ("10s") |

:::info Repeat Count Inheritance
Since `0` is a valid `repeat_count` value (repeat forever), it cannot be distinguished from "not set" in TOML. If you want to explicitly set no repeat, use `-1`.
:::

## Examples

### Minimal Manifest

Just metadata:

```toml
name = "My Colors"
description = "Custom color palette"
author = "me"
version = "1.0.0"
```

### Sound-Only Theme

Uses default styling but adds custom sounds:

```toml
name = "Corporate"
description = "Corporate branding with custom sounds"
version = "1.0.0"

[audio.normal]
path = "sounds/chime.ogg"
volume = 0.6

[audio.critical]
path = "sounds/alert.ogg"
volume = 1.0
repeat_count = 2
repeat_delay = "5s"
```

### Silent Theme

Disables all sounds:

```toml
name = "Silent"
description = "No notification sounds"
version = "1.0.0"

[audio.low]
path = ""

[audio.normal]
path = ""

[audio.critical]
path = ""
```

### Compact Theme with Small Icons

```toml
name = "Compact"
description = "Compact notifications with small icons"
version = "1.0.0"

[icon]
size = 24

# Inherit default sounds
```

### Full Theme Pack

```toml
name = "Cyberpunk"
description = "Neon-styled notifications with retro sounds"
author = "themecreator"
version = "2.0.0"

[icon]
size = 32

[audio.low]
path = "sounds/blip.ogg"
volume = 0.4
repeat_count = -1

[audio.normal]
path = "sounds/notify.ogg"
volume = 0.7
repeat_count = -1

[audio.critical]
path = "sounds/alarm.ogg"
volume = 1.0
repeat_count = 0
repeat_delay = "8s"
```

## Directory Structure

A complete theme pack with sounds:

```
~/.config/histui/themes/mytheme/
├── manifest.toml       # This file
├── theme.css           # CSS styling
├── layout.xml          # Widget layout (optional)
└── sounds/             # Audio files
    ├── notify.ogg      # Normal urgency sound
    ├── alert.ogg       # Critical urgency sound
    └── subtle.ogg      # Low urgency sound
```

## Embedded Theme Sounds

Bundled themes include embedded sounds that are extracted to the cache directory on first use:

```
~/.cache/histui/themes/default/sounds/
├── normal.wav
└── critical.wav
```

When you inherit from a bundled theme, the embedded sounds are automatically extracted and used.

## See Also

- [Layout Reference](/docs/histuid/theming/layout-reference) - Widget layout configuration
- [CSS Reference](/docs/histuid/theming/css-reference) - CSS styling
- [Extending Themes](/docs/histuid/theming/extending) - Creating partial themes

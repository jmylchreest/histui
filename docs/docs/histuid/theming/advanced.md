---
title: Advanced Theming
description: Theme pack structure, layouts, manifests, and icon configuration
sidebar_position: 4
---

# Advanced Theming

This guide covers the complete theme pack structure including layouts, manifests, audio, and icon configuration.

## Theme Pack Structure

A complete theme pack contains:

```
~/.config/histui/themes/mytheme/
├── theme.css           # Required: CSS styling
├── layout.xml          # Optional: Widget layout and sizing
├── manifest.toml       # Optional: Metadata, audio, and icon config
└── sounds/             # Optional: Audio files
    ├── notify.wav
    └── critical.ogg
```

## Layout Configuration (layout.xml)

The layout file defines which widgets appear and their arrangement.

### Basic Structure

```xml
<popup min-width="250" max-width="400" min-height="0" max-height="600">
  <header>
    <icon size="48" />
    <box orientation="vertical">
      <summary />
      <appname />
    </box>
    <stack-count />
    <close />
  </header>
  <body />
  <progress />
  <image />
  <actions />
</popup>
```

### Popup Attributes

| Attribute    | Type | Description                              |
|--------------|------|------------------------------------------|
| `min-width`  | int  | Minimum popup width in pixels            |
| `max-width`  | int  | Maximum popup width in pixels            |
| `min-height` | int  | Minimum popup height in pixels           |
| `max-height` | int  | Maximum popup height in pixels           |

### Available Elements

| Element        | Attributes       | Description                           |
|----------------|------------------|---------------------------------------|
| `<icon>`       | `size="48"`      | Application icon (Nerd Font fallback) |
| `<summary>`    |                  | Notification title                    |
| `<body>`       |                  | Notification message body             |
| `<appname>`    |                  | Application name                      |
| `<timestamp>`  |                  | Time since notification               |
| `<progress>`   |                  | Progress bar (if hint provided)       |
| `<image>`      |                  | Notification image                    |
| `<actions>`    |                  | Action buttons                        |
| `<close>`      |                  | Close button                          |
| `<stack-count>`|                  | Badge showing stacked notification count |
| `<header>`     |                  | Container for header elements         |
| `<box>`        | `orientation`    | Container with vertical/horizontal layout |

### Layout Examples

**Minimal (no icons):**
```xml
<popup min-width="150" max-width="250" min-height="0" max-height="200">
  <summary />
  <body />
  <progress />
</popup>
```

**Compact (smaller icons):**
```xml
<popup min-width="200" max-width="300" min-height="0" max-height="400">
  <header>
    <icon size="32" />
    <summary />
    <stack-count />
    <close />
  </header>
  <body />
  <progress />
  <actions />
</popup>
```

**Detailed (with timestamp):**
```xml
<popup min-width="250" max-width="450" min-height="0" max-height="700">
  <header>
    <icon size="48" />
    <box orientation="vertical">
      <summary />
      <box orientation="horizontal">
        <appname />
        <timestamp />
      </box>
    </box>
    <stack-count />
    <close />
  </header>
  <body />
  <progress />
  <image />
  <actions />
</popup>
```

## Theme Manifest (manifest.toml)

The manifest file configures metadata, audio, and icon settings.

### Complete Reference

```toml
# Theme metadata
name = "My Theme"
description = "A custom notification theme"
author = "Your Name"
version = "1.0.0"

# Icon configuration
[icon]
size = 48    # Default icon size (can be overridden in layout.xml)

# Audio per urgency level
[audio.low]
path = "sounds/subtle.wav"     # Path relative to theme directory
volume = 0.5                   # Volume 0.0-1.0
repeat_count = -1              # -1 = once, 0 = until dismissed, N = N times

[audio.normal]
path = "sounds/notify.wav"
volume = 0.8
repeat_count = -1

[audio.critical]
path = "sounds/alert.ogg"
volume = 1.0
repeat_count = 0               # Repeat until dismissed
repeat_delay = "10s"           # Delay between repeats
```

### Audio Repeat Behavior

| `repeat_count` | Behavior                          |
|----------------|-----------------------------------|
| `-1`           | Play once, no repeat              |
| `0`            | Repeat until notification dismissed |
| `N`            | Repeat N times                    |

### Supported Audio Formats

- WAV (`.wav`)
- OGG Vorbis (`.ogg`)
- MP3 (`.mp3`)
- FLAC (`.flac`)

## Icon Configuration

histuid uses a multi-tier icon resolution system:

1. **Application-provided icon** (from notification)
2. **GTK icon theme** (freedesktop icons)
3. **Nerd Font symbol** (fallback for missing icons)

### Icon Aliases

Map application names to icon names for better resolution:

```toml
# ~/.config/histui/icon-aliases.toml

[aliases]
# Map app name to standard icon name
zapzap = "whatsapp"
telegram-desktop = "telegram"
firefox-esr = "firefox"
vesktop = "discord"
my-custom-app = "application-default-icon"
```

### Nerd Font Symbols

When GTK icons aren't available, histuid displays Nerd Font symbols.
Override the default symbols:

```toml
# ~/.config/histui/icon-aliases.toml

[symbols]
# Override app symbols (icon name -> Nerd Font glyph)
spotify = "\U000F04C7"     # nf-md-spotify
discord = "\U000F066F"     # nf-md-discord

# Override urgency fallbacks
critical = "\U000F0026"    # nf-md-alert
normal = "\U000F009A"      # nf-md-bell
low = "\U000F02FC"         # nf-md-information
undefined = "\U000F009C"   # nf-md-bell-outline

# Override category fallbacks
notification = "\U000F009A"  # nf-md-bell
im = "\U000F0CE4"            # nf-md-chat
```

### Built-in Aliases

histuid includes aliases for 350+ common applications:
- Messaging: Discord, Slack, Telegram, WhatsApp, Signal
- Browsers: Firefox, Chrome, Brave, Edge, Opera
- Email: Thunderbird, Evolution, Geary
- Media: Spotify, VLC, MPV
- Development: VS Code, terminals, Git clients
- And many more...

### Icon Alias Generator

The icon aliases are generated from Nerd Fonts data. To regenerate with different preferences:

```bash
cd contrib/generate-icon-aliases

# Prefer Material Design icons (default)
go run . --fetch --output ../../internal/icon/aliases_default.toml

# Prefer Font Awesome icons
go run . --fetch --prefer fa --output ../../internal/icon/aliases_default.toml
```

## Font Configuration

### CSS Variables

Themes should use these variables for fonts:

```css
:root {
    --histui-font-family: inherit;  /* System font */
    --histui-font-size: 14px;
}

.notification-popup {
    font-family: var(--histui-font-family);
    font-size: var(--histui-font-size);
}
```

### Override Priority

1. **CLI flags** (highest): `histuid --font "Ubuntu" --font-size 16`
2. **Config file**: `theme.font = "Ubuntu"`
3. **Theme CSS**: `:root { --histui-font-family: "Ubuntu"; }`

## CSS Animations

GTK4 supports CSS animations via `@keyframes`.

### Pulsing Glow (Critical Notifications)

```css
@keyframes critical-pulse {
    0%, 100% {
        text-shadow: 0 0 4px alpha(@error_color, 0.4);
    }
    50% {
        text-shadow: 0 0 12px alpha(@error_color, 0.8),
                     0 0 20px alpha(@error_color, 0.4);
    }
}

.notification-popup.urgency-critical .notification-summary {
    color: @error_color;
    animation: critical-pulse 2s ease-in-out infinite;
}
```

### Slide-In Effect

```css
@keyframes slide-in {
    from {
        opacity: 0;
        transform: translateX(100px);
    }
    to {
        opacity: 1;
        transform: translateX(0);
    }
}

.notification-popup {
    animation: slide-in 0.3s ease-out;
}
```

### Animation Properties

| Property | Values | Description |
|----------|--------|-------------|
| `animation-name` | keyframes name | Which animation to use |
| `animation-duration` | `2s`, `500ms` | How long one cycle takes |
| `animation-timing-function` | `ease`, `ease-in`, `ease-out`, `linear` | Easing curve |
| `animation-iteration-count` | `1`, `infinite` | How many times to run |
| `animation-delay` | `0s`, `200ms` | Delay before starting |

## Compositor Integration

### Hyprland Blur

For translucent notifications with blur:

```ini
# hyprland.conf
layerrule = blur, histui-notification
layerrule = ignorealpha 0.5, histui-notification
```

### Color Mixing

GTK4 CSS supports `color-mix()` for solid blended colors:

```css
/* Muted danger background - solid color, not transparent */
.notification-popup.urgency-critical {
    background-color: color-mix(in srgb, @error_color 8%, @window_bg_color);
    border-color: color-mix(in srgb, @error_color 40%, @borders);
}
```

| Function | Result | Use Case |
|----------|--------|----------|
| `alpha(@color, 0.1)` | Semi-transparent | Compositor blur |
| `color-mix(in srgb, @color 10%, @base)` | Solid blended | Muted backgrounds |

## Complete Theme Example

```
~/.config/histui/themes/custom/
├── theme.css
├── layout.xml
├── manifest.toml
└── sounds/
    ├── notify.ogg
    └── alert.ogg
```

**layout.xml:**
```xml
<popup min-width="250" max-width="400" min-height="0" max-height="600">
  <header>
    <icon size="48" />
    <box orientation="vertical">
      <summary />
      <appname />
    </box>
    <stack-count />
    <close />
  </header>
  <body />
  <progress />
  <image />
  <actions />
</popup>
```

**manifest.toml:**
```toml
name = "Custom"
description = "Custom theme with sounds and animations"
version = "1.0.0"

[icon]
size = 48

[audio.normal]
path = "sounds/notify.ogg"
volume = 0.7

[audio.critical]
path = "sounds/alert.ogg"
volume = 1.0
repeat_count = 3
repeat_delay = "5s"
```

**theme.css:**
```css
:root {
    --histui-font-family: "Inter", sans-serif;
    --histui-font-size: 14px;
}

window {
    background-color: transparent;
}

.notification-popup {
    background-color: alpha(#1e1e2e, 0.9);
    color: #cdd6f4;
    border-radius: 12px;
    border: 1px solid #45475a;
    padding: 12px;
    margin: 8px;
    font-family: var(--histui-font-family);
    animation: fade-in 0.2s ease-out;
}

@keyframes fade-in {
    from { opacity: 0; transform: scale(0.95); }
    to { opacity: 1; transform: scale(1); }
}

.notification-popup.urgency-critical {
    background-color: color-mix(in srgb, #f38ba8 10%, #1e1e2e);
    border-color: #f38ba8;
}
```

## See Also

- [CSS Reference](/docs/histuid/theming/css-reference) - All CSS selectors
- [Theme Examples](/docs/histuid/theming/examples) - Ready-to-use themes
- [Configuration](/docs/histuid/configuration) - Main config reference

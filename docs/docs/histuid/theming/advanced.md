---
title: Advanced Theming
description: Directory themes, audio, fonts, and CSS animations
sidebar_position: 4
---

# Advanced Theming

This guide covers advanced theming features including directory-based themes with audio, font configuration, and CSS animations.

## Theme Formats

histuid supports two theme formats:

### Single CSS File

Simple themes can be a single CSS file:

```
~/.config/histui/themes/mytheme.css
```

### Directory-Based Theme

For themes with audio, icons, or multiple files, use a directory structure:

```
~/.config/histui/themes/mytheme/
├── theme.css           # Required: Main stylesheet (or mytheme.css)
├── manifest.toml       # Optional: Audio and icon configuration
└── sounds/             # Optional: Audio files
    ├── notify.wav
    └── critical.ogg
```

## Theme Manifest

The `manifest.toml` file configures audio and icon behavior for directory-based themes.

### Audio Configuration

```toml
# manifest.toml

[audio.low]
path = "sounds/subtle.wav"     # Path relative to theme directory
volume = 0.5                   # Volume 0.0-1.0 (optional, uses global default)

[audio.normal]
path = "sounds/notify.wav"

[audio.critical]
path = "sounds/alert.ogg"
volume = 1.0
repeat_count = 0               # 0 = repeat until dismissed
repeat_delay = "10s"           # Delay between repeats (default: 10s)
```

### Repeat Behavior

| `repeat_count` | Behavior |
|----------------|----------|
| `-1` | Play once, no repeat |
| `0` | Repeat until dismissed |
| `N` | Repeat N times |

### Icon Configuration

```toml
[icon]
size = 48    # Icon size in pixels (default: 48)
```

### Full Manifest Example

```toml
# mytheme/manifest.toml

# Theme metadata (optional)
name = "My Custom Theme"
description = "A custom theme with sounds"
author = "Your Name"
version = "1.0.0"

# Audio per urgency level
[audio.low]
path = "sounds/ping.wav"
volume = 0.3

[audio.normal]
path = "sounds/notify.wav"
volume = 0.6

[audio.critical]
path = "sounds/alarm.ogg"
volume = 1.0
repeat_count = 0
repeat_delay = "5s"

# Icon settings
[icon]
size = 48
```

## Font Configuration

histuid uses CSS custom properties for font configuration, which can be overridden at multiple levels.

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

### Theme Font Override

Override fonts in your theme:

```css
/* ~/.config/histui/themes/mytheme.css */
:root {
    --histui-font-family: "JetBrains Mono";
    --histui-font-size: 13px;
}
```

### CLI Font Override

Override fonts via command line (highest priority):

```bash
histuid --font "Ubuntu" --font-size 16
```

### Config File Override

```toml
# ~/.config/histui/histuid.toml
[theme]
name = "default"
font = "Ubuntu"
font_size = 14
```

## CSS Animations

GTK4 supports CSS animations via `@keyframes`. Use these for attention-grabbing effects.

### Pulsing Glow Effect

The default theme uses this for critical notifications:

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

### Breathing Effect

```css
@keyframes breathe {
    0%, 100% {
        opacity: 1;
    }
    50% {
        opacity: 0.7;
    }
}

.notification-popup.is-transient {
    animation: breathe 3s ease-in-out infinite;
}
```

### Animation Properties

| Property | Values | Description |
|----------|--------|-------------|
| `animation-name` | keyframes name | Which animation to use |
| `animation-duration` | `2s`, `500ms` | How long one cycle takes |
| `animation-timing-function` | `ease`, `ease-in`, `ease-out`, `ease-in-out`, `linear` | Easing curve |
| `animation-iteration-count` | `1`, `2`, `infinite` | How many times to run |
| `animation-direction` | `normal`, `reverse`, `alternate` | Play direction |
| `animation-delay` | `0s`, `200ms` | Delay before starting |

### Shorthand

```css
/* animation: name duration timing-function iteration-count */
animation: critical-pulse 2s ease-in-out infinite;
```

## Translucent Notifications

For compositor blur effects, use semi-transparent backgrounds.

### CSS Setup

```css
.notification-popup {
    background-color: alpha(@window_bg_color, 0.85);
}

/* Or add a translucent class */
.notification-popup.translucent {
    background-color: alpha(@window_bg_color, 0.85);
}
```

### Hyprland Configuration

Add to your `hyprland.conf`:

```ini
# Blur for histui notifications
layerrule = blur, histui-notification
layerrule = ignorealpha 0.5, histui-notification
```

### Sway Configuration

Sway doesn't support blur for layer surfaces natively. Consider using a patched version or alternative effect.

## Color Mixing

GTK4 CSS supports `color-mix()` for creating solid blended colors:

```css
/* Muted danger background - solid color, not transparent */
.notification-popup.urgency-critical {
    background-color: color-mix(in srgb, @error_color 8%, @window_bg_color);
    border-color: color-mix(in srgb, @error_color 40%, @borders);
}
```

### alpha() vs color-mix()

| Function | Result | Use Case |
|----------|--------|----------|
| `alpha(@color, 0.1)` | Semi-transparent | Compositor blur, overlays |
| `color-mix(in srgb, @color 10%, @base)` | Solid blended color | Muted backgrounds, tints |

## Creating a Complete Theme

Here's a complete example combining all features:

```
~/.config/histui/themes/custom/
├── theme.css
├── manifest.toml
└── sounds/
    ├── notify.ogg
    └── alert.ogg
```

**theme.css:**
```css
/* Custom Theme with animations and proper fonts */

:root {
    --histui-font-family: "Inter", sans-serif;
    --histui-font-size: 14px;
}

window {
    background-color: transparent;
}

.notification-popup {
    background-color: #1e1e2e;
    color: #cdd6f4;
    border-radius: 12px;
    border: 1px solid #45475a;
    padding: 12px;
    font-family: var(--histui-font-family);
    font-size: var(--histui-font-size);
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

@keyframes pulse {
    0%, 100% { text-shadow: 0 0 4px #f38ba8; }
    50% { text-shadow: 0 0 12px #f38ba8, 0 0 20px #f38ba8; }
}

.notification-popup.urgency-critical .notification-summary {
    color: #f38ba8;
    animation: pulse 2s ease-in-out infinite;
}
```

**manifest.toml:**
```toml
name = "Custom"
description = "Custom theme with sounds and animations"
version = "1.0.0"

[audio.normal]
path = "sounds/notify.ogg"
volume = 0.7

[audio.critical]
path = "sounds/alert.ogg"
volume = 1.0
repeat_count = 3
repeat_delay = "5s"

[icon]
size = 48
```

## Icon Resolution

histuid includes a built-in icon resolver with common app name aliases. This helps find icons when applications use non-standard names.

### Built-in Aliases

The resolver includes aliases for common apps:
- `zapzap` → `whatsapp`
- `telegram-desktop` → `telegram`
- `firefox-esr` → `firefox`
- `vesktop` → `discord`
- And many more...

### Custom Aliases

Add your own aliases in a separate file for easy sharing:

```toml
# ~/.config/histui/icon-aliases.toml

[aliases]
myapp = "standard-icon-name"
custom-browser = "web-browser"
zapzap = "whatsapp"
telegram-desktop = "telegram"
```

This file is loaded automatically on startup. User aliases take precedence over built-in aliases.

### Nerd Font Symbols (Planned)

Future versions may support Nerd Font symbols as icon fallbacks. The resolver includes mappings for common symbols:
- Discord, Slack, Telegram, WhatsApp
- Firefox, Chrome, Chromium
- Terminal, Code, Folder, File
- Network, Bluetooth, Volume
- Warning, Error, Info, Success

## See Also

- [CSS Reference](/docs/histuid/theming/css-reference) - All CSS selectors
- [Theme Examples](/docs/histuid/theming/examples) - Ready-to-use themes
- [Configuration](/docs/histuid/configuration) - Main config reference
- [GTK4 CSS Properties](https://docs.gtk.org/gtk4/css-properties.html) - Full GTK4 reference

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
| `low` | duration | "5s" | Timeout for low urgency |
| `normal` | duration | "10s" | Timeout for normal urgency |
| `critical` | duration | "never" | Timeout for critical urgency |

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
- If the client requests `-1` (server decides), the daemon uses fallback defaults (5s low, 10s normal, never critical)
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

## Example Configuration

```toml
[display]
position = "top-right"
max_visible = 5
offset_x = 10
offset_y = 10

[timeouts]
# Override low urgency to 3 seconds
low = "3s"

# Honor whatever the application requests for normal urgency
normal = "0"

# Critical notifications never expire (default)
critical = "never"

[behavior]
stack_duplicates = true
pause_on_hover = true
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

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

Controls how long notifications are displayed.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `default` | int | 5000 | Default timeout in milliseconds |
| `low` | int | 3000 | Timeout for low urgency |
| `critical` | int | 0 | Timeout for critical (0 = never) |

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
width = 400
margin = 15

[timeouts]
default = 5000
critical = 0

[history]
max_count = 500
```

## See Also

- [Theming](/docs/histuid/theming) - Customize appearance
- [Monitor Mode](/docs/histuid/monitor-mode) - Run with dunst

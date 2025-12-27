---
title: Monitor Mode
description: Run histuid alongside other notification daemons
sidebar_position: 3
---

# Monitor Mode

Monitor mode allows histuid to capture notification history without displaying popups.
This is useful when you want to use another notification daemon like dunst for display.

## Use Case

- Use dunst for notification popups (familiar, well-configured)
- Use histui for history browsing and querying
- Best of both worlds

## Enabling Monitor Mode

In your configuration file:

```toml
[behavior]
monitor_mode = true
```

Or via command line:

```bash
histuid --monitor
```

## How It Works

In monitor mode, histuid:

1. Registers as a secondary listener on the D-Bus notification interface
2. Captures all notifications for history storage
3. Does NOT display popups (leaves that to dunst/mako/etc.)
4. Still provides the histui CLI for querying

## Example Setup

### 1. Configure histuid for monitor mode

```toml
# ~/.config/histui/histuid.toml
[behavior]
monitor_mode = true
```

### 2. Start both daemons

In your session startup (e.g., sway/hyprland config):

```bash
# Start dunst for popups
exec dunst

# Start histuid for history (monitor mode)
exec histuid
```

### 3. Query history with histui

```bash
histui get --app discord --since 1h
```

## See Also

- [D-Bus Notifications Specification](https://specifications.freedesktop.org/notification-spec/)
- [Configuration](/docs/histuid/configuration)

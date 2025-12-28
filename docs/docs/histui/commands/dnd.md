---
title: histui dnd
description: Manage Do Not Disturb mode
sidebar_label: dnd
sidebar_position: 6
---

# histui dnd

Manage Do Not Disturb (DnD) mode for histuid.

## Synopsis

```bash
histui dnd [command] [flags]
```

## Description

The `dnd` command controls Do Not Disturb mode for histuid. When DnD is enabled,
notification popups and sounds are suppressed while notifications are still persisted
to the history store for later review.

## Subcommands

| Command | Description |
|---------|-------------|
| `status` | Show current DnD status (default if no command given) |
| `on` | Enable Do Not Disturb mode |
| `off` | Disable Do Not Disturb mode |
| `toggle` | Toggle DnD mode between enabled and disabled |

## Options

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-q`, `--quiet` | bool | false | Suppress output, return exit code only |

## Exit Codes

The exit code indicates the final DnD state:

| Code | Meaning |
|------|---------|
| 0 | DnD is disabled (off) |
| 1 | DnD is enabled (on) |

This allows use in scripts without parsing output.

## Examples

### Check DnD status

```bash
histui dnd
# Or explicitly:
histui dnd status
```

Output:
```
Do Not Disturb: enabled
  Last change: 5 minutes ago
  Trigger: user
  Reason: dnd on
  Source: cli
```

### Enable DnD

```bash
histui dnd on
```

### Disable DnD

```bash
histui dnd off
```

### Toggle DnD

```bash
histui dnd toggle
```

### Use in scripts

```bash
# Check status without output
if histui dnd status --quiet; then
  echo "DnD is off"
else
  echo "DnD is on"
fi

# Toggle with script
histui dnd toggle --quiet
```

## Waybar Integration

Use with Waybar to show DnD status and toggle on click:

```json
{
  "custom/notifications": {
    "exec": "histui status",
    "interval": 5,
    "return-type": "json",
    "on-click": "histui tui",
    "on-click-right": "histui dnd toggle",
    "format": "{icon}",
    "format-icons": {
      "dnd": "󰂛",
      "empty": "",
      "normal": "󰂚"
    }
  }
}
```

## Critical Notifications

By default, critical notifications bypass DnD mode and are still shown.
This ensures important system alerts are never missed.

To also suppress critical notifications when DnD is enabled:

```toml
[dnd]
suppress_critical = true  # Also silence critical notifications in DnD
```

The default (`suppress_critical = false`) allows critical notifications through.

## State Persistence

DnD state is persisted in `~/.local/share/histui/state.json` and survives
daemon restarts. The state includes:

- Whether DnD is enabled
- When it was last changed
- What triggered the change (user, schedule, etc.)
- The source of the change (cli, dbus, waybar, etc.)

## See Also

- [histui status](/docs/histui/commands/status) - Includes DnD state in output
- [histui tui](/docs/histui/commands/tui) - Interactive browser
- [histuid Configuration](/docs/histuid/configuration#dnd) - DnD config options

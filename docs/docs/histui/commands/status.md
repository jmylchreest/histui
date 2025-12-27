---
title: histui status
description: Get notification status for status bars
sidebar_label: status
sidebar_position: 3
---

# histui status

Output notification status for integration with status bars like Waybar.

## Synopsis

```bash
histui status [flags]
```

## Description

The `status` command outputs notification counts and status information in formats suitable for status bar integration.

## Options

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--format` | string | plain | Output format: plain, json, waybar |
| `--unread` | bool | false | Only count unread notifications |

## Examples

### Get notification count

```bash
histui status
```

### Waybar format

```bash
histui status --format waybar
```

## Waybar Integration

Add to your Waybar config:

```json
{
  "custom/notifications": {
    "exec": "histui status --format waybar",
    "return-type": "json",
    "interval": 5
  }
}
```

See the [Waybar Wiki](https://github.com/Alexays/Waybar/wiki/Module:-Custom) for more details.

## See Also

- [histui get](/docs/histui/commands/get) - Query notifications

---
title: histui status
description: Get notification status for status bars
sidebar_label: status
sidebar_position: 5
---

# histui status

Output notification status for integration with status bars like Waybar.

## Synopsis

```bash
histui status [flags]
```

## Description

The `status` command outputs notification counts in Waybar's custom module JSON format.
By default, it shows only **active** (unacknowledged) notifications - those currently
displayed or waiting to be displayed. Use `--all` to include history.

## Output Format

The output is always JSON suitable for Waybar's `return-type: json`:

```json
{
  "text": "3",
  "alt": "normal",
  "tooltip": "3 active\nDisplayed: 2\nWaiting: 1",
  "class": "has-notifications normal",
  "percentage": 3
}
```

| Field | Description |
|-------|-------------|
| `text` | Notification count (empty if none) |
| `alt` | Urgency level: `empty`, `low`, `normal`, `critical`, `dnd`, `error` |
| `tooltip` | Breakdown of notification counts |
| `class` | CSS class for styling (e.g., `has-notifications normal`) |
| `percentage` | Count capped at 100 (for progress bar styling) |

## Options

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--source` | string | auto | Notification source: `dunst`, `histuid` (auto-detects) |
| `--all` | bool | false | Include history (acknowledged) notifications in count |
| `--since` | duration | | Only count notifications from the last duration (with `--all`) |
| `--urgency` | string | | Only count notifications of this urgency level |

## Examples

### Get active notification count

```bash
histui status
```

### Include history in count

```bash
histui status --all
```

### Filter by urgency

```bash
histui status --urgency critical
```

## Waybar Integration

Add to your Waybar config:

```json
{
  "custom/notifications": {
    "exec": "histui status",
    "interval": 5,
    "return-type": "json",
    "on-click": "histui tui",
    "format": "{icon}",
    "format-icons": {
      "empty": "",
      "low": "󰂚",
      "normal": "󰂚",
      "critical": "󰂛",
      "dnd": "󰂛",
      "error": ""
    }
  }
}
```

### With notification count

```json
{
  "custom/notifications": {
    "exec": "histui status",
    "interval": 5,
    "return-type": "json",
    "on-click": "histui tui",
    "format": "{icon} {text}"
  }
}
```

### Styling with CSS

```css
#custom-notifications {
  padding: 0 10px;
}

#custom-notifications.empty {
  opacity: 0.5;
}

#custom-notifications.critical {
  background-color: #f38ba8;
  color: #1e1e2e;
}

#custom-notifications.dnd {
  opacity: 0.3;
}
```

## Do Not Disturb

When DnD is enabled (via `histui dnd on`), the status reflects this:

- `alt` and `class` are set to `dnd`
- Tooltip shows "Do Not Disturb: enabled"
- Count still shows suppressed notifications

See the [Waybar Wiki](https://github.com/Alexays/Waybar/wiki/Module:-Custom) for more details.

## See Also

- [histui get](/docs/histui/commands/get) - Query notifications
- [histui dnd](/docs/histui/commands/dnd) - Control Do Not Disturb

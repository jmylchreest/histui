---
title: histui tui
description: Interactive terminal user interface
sidebar_label: tui
sidebar_position: 3
---

# histui tui

Launch an interactive terminal user interface for browsing notifications.

## Synopsis

```bash
histui tui [flags]
# Or simply:
histui
```

## Description

The `tui` command (or just `histui` with no arguments) launches an interactive terminal interface built with BubbleTea. Browse, search, filter, and manage your notification history with vim-style keyboard navigation.

## Features

- **Live search and filtering** - Type to search, use expressions for advanced filtering
- **Preview panel** - See notification details without leaving the list
- **Replay notifications** - Re-send notifications through D-Bus
- **Clipboard integration** - Copy notification content to clipboard
- **Image preview** - View notification images inline (halfblock rendering)

## Status Indicators

Notifications may show status indicators in the left margin:

| Indicator | Meaning |
|-----------|---------|
| `D` | Dismissed (soft-deleted) |
| `R` | Replayed (was re-sent via D-Bus) |

## Keybindings

### Navigation

| Key | Action |
|-----|--------|
| `j` / `Down` | Move down |
| `k` / `Up` | Move up |
| `g` / `Home` | Go to top |
| `G` / `End` | Go to bottom |
| `PgUp` / `Ctrl+U` | Page up |
| `PgDn` / `Ctrl+D` | Page down |

### Actions

| Key | Action |
|-----|--------|
| `Enter` | View notification details |
| `p` | Toggle preview panel |
| `c` | Copy body to clipboard |
| `s` | Copy summary to clipboard |
| `C` | Copy all visible as JSON |
| `Alt+C` | Copy all visible as YAML |
| `d` | Dismiss/undismiss notification |
| `Alt+D` | Dismiss all visible notifications |
| `D` | Delete permanently |
| `R` | Replay notification |
| `/` | Start search/filter |
| `r` | Refresh list |
| `a` | Toggle showing dismissed |

### General

| Key | Action |
|-----|--------|
| `?` | Show help (two pages) |
| `Esc` | Back / close search |
| `q` / `Ctrl+C` | Quit |

## Search and Filtering

Press `/` to enter search mode. You can:

### Text Search

Just type to search in summary, body, and app name:

```
meeting
```

### Expression Filters

Use field operators for precise filtering:

```
app=discord
urgency=critical
body~meeting
dismissed=false
timestamp<1h
app=slack,seen=false
```

See the [Filtering Guide](/docs/histui/filtering) for full syntax.

## Options

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--source` | string | auto | Notification source: `histuid`, `stdin` (auto-detects if empty) |

## Examples

### Launch TUI

```bash
histui tui
# Or simply:
histui
```

:::tip
To filter notifications, launch the TUI and press `/` to enter search mode.
You can then type expression filters like `app=discord` or `urgency=critical,dismissed=false`.
:::

## Workflow Tips

### Quick dismiss workflow

1. Press `a` to show all (including dismissed)
2. Navigate to notifications
3. Press `d` to toggle dismiss status

### Copy notification content

1. Navigate to a notification
2. Press `c` to copy body, or `s` to copy summary
3. Content is now in your clipboard

### Replay a notification

1. Navigate to a notification
2. Press `R` to replay
3. Notification appears through your notification daemon

## See Also

- [histui get](/docs/histui/commands/get) - Query notifications (CLI)
- [histui replay](/docs/histui/commands/replay) - Replay notifications (CLI)
- [Filtering Guide](/docs/histui/filtering) - Advanced filter syntax

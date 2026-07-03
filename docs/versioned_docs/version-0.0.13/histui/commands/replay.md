---
title: histui replay
description: Replay notifications through D-Bus
sidebar_label: replay
sidebar_position: 4
---

# histui replay

Replay historical notifications through the active notification daemon.

## Synopsis

```bash
histui replay [index|id] [flags]
```

## Description

The `replay` command sends a notification from history back through D-Bus to the currently active notification daemon (histuid, dunst, mako, etc.).

When replaying to histuid:
- The original notification is marked as replayed (shows `R` indicator in TUI)
- No duplicate history entry is created
- Original images and metadata are preserved

When replaying to other daemons:
- The notification appears as a new notification
- Custom hints (`x-histui-replay`, `x-histui-original-id`) identify it as a replay

## Pipe Support

When stdin is a pipe and no arguments are provided, the command automatically reads a notification selection from stdin. This enables clean pipe workflows without `xargs`:

```bash
histui get | fuzzel -d | histui replay
```

## Options

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--index` | int | | Lookup notification by 1-based index |
| `--id` | string | | Lookup notification by histui ID |
| `--stdin` | bool | false | Force reading from stdin (auto-detected when piped) |
| `--timeout` | int | -1 | Custom timeout in milliseconds (-1 = server default) |
| `--since` | duration | | Filter: replay notifications from the last duration |
| `--app` | string | | Filter: only replay notifications from this app |
| `--urgency` | string | | Filter: only replay notifications with this urgency |
| `--limit` | int | 10 | Maximum notifications to replay in batch mode |
| `--all` | bool | false | Required for batch replay with filters |

## Examples

### Replay the most recent notification

```bash
histui replay 1
```

### Replay by notification ID

```bash
histui replay 01HXYZ123ABC456DEF789GHI
```

### Replay with custom timeout (10 seconds)

```bash
histui replay 1 --timeout 10000
```

### Pipeline with launcher (recommended)

```bash
# Interactive selection with fuzzel
histui get | fuzzel -d | histui replay

# Interactive selection with rofi
histui get | rofi -dmenu | histui replay

# Interactive selection with walker
histui get | walker -d | histui replay

# Interactive selection with fzf
histui get | fzf | histui replay
```

### Batch replay with filters

```bash
# Replay all critical notifications from the last hour
histui replay --since 1h --urgency critical --all

# Replay last 5 notifications from Discord
histui replay --app discord --limit 5 --all
```

## Pipeline Workflows

### Copy and replay workflow

```bash
# Select a notification, copy its body, then replay it
histui get | fuzzel -d | tee >(histui replay) | cut -d'|' -f1 | xargs histui get --field body | wl-copy
```

### Replay missed notifications

```bash
# Replay all unseen notifications from the last 2 hours (with delay between each)
histui get --filter "seen=false" --since 2h --format ids | while read id; do
  histui replay "$id"
  sleep 1
done
```

## TUI Replay

You can also replay notifications from the TUI:

1. Open the TUI: `histui`
2. Navigate to a notification
3. Press `R` to replay

Replayed notifications show an `R` indicator in the left margin.

## See Also

- [histui get](/docs/histui/commands/get) - Query notifications
- [histui tui](/docs/histui/commands/tui) - Interactive browser
- [Filtering Guide](/docs/histui/filtering) - Advanced filter syntax

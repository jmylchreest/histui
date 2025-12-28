---
title: histui get
description: Query and output notification history
sidebar_label: get
sidebar_position: 1
---

# histui get

Query notification history and output in various formats.

## Synopsis

```bash
histui get [index|id] [flags]
```

## Description

The `get` command retrieves notifications from history and outputs them in the specified format.
Without arguments, it returns all notifications in dmenu format (suitable for fuzzel, rofi, walker, etc.).
With an index or ID, it returns a specific notification.

## Options

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--app` | string | | Filter by application name |
| `--urgency` | string | | Filter by urgency (low, normal, critical) |
| `--since` | duration | | Only show notifications from the last duration (e.g., 1h, 30m) |
| `--filter` | string | | Expression filter (e.g., `app=discord,urgency=critical`) |
| `--search` | string | | Search in summary and body |
| `--format` | string | dmenu | Output format: dmenu, json, plain, ids |
| `--field` | string | | Output single field: id, app, summary, body, all |
| `--sort` | string | timestamp | Sort by: timestamp, app, urgency |
| `--order` | string | desc | Sort order: asc, desc |
| `--limit` | int | 0 | Maximum number of results (0 = unlimited) |

## Output Formats

| Format | Description | Use Case |
|--------|-------------|----------|
| `dmenu` | Index, time, app, summary separated by `\|` | Fuzzel, rofi, walker, dmenu |
| `json` | Full JSON array | Scripting, jq processing |
| `plain` | Simple text output | Terminals, logs |
| `ids` | Just histui IDs, one per line | Piping to other commands |

## Examples

### Basic Queries

```bash
# List all notifications
histui get

# Filter by application
histui get --app firefox

# Filter by time and urgency
histui get --since 1h --urgency critical

# Search in content
histui get --search "meeting"
```

### Expression Filters

```bash
# Filter with expressions
histui get --filter "app=discord,urgency=critical"
histui get --filter "body~meeting,dismissed=false"
histui get --filter "timestamp<1h"
```

### Output Formats

```bash
# JSON output for scripting
histui get --format json | jq '.[] | .summary'

# Get just the IDs
histui get --format ids

# Get a specific field from a notification
histui get 1 --field body
histui get 1 --field summary
```

### Launcher Integration

```bash
# Use with fuzzel
histui get | fuzzel -d

# Use with rofi
histui get | rofi -dmenu -i

# Use with walker
histui get | walker --dmenu

# Use with fzf
histui get | fzf
```

### Pipeline Workflows

```bash
# Select and copy body to clipboard
histui get | fuzzel -d | xargs histui get --field body | wl-copy

# Select and replay notification
histui get | fuzzel -d | histui replay

# Get recent Discord messages as JSON
histui get --app discord --since 1h --format json

# Count notifications per app
histui get --format json | jq -r '.[].app_name' | sort | uniq -c | sort -rn
```

### Specific Notification Lookup

```bash
# Get notification by index (1-based, most recent first)
histui get 1

# Get notification by histui ID
histui get 01HXYZ123ABC456DEF789GHI

# Get body of most recent notification
histui get 1 --field body
```

## See Also

- [histui replay](/docs/histui/commands/replay) - Replay notifications
- [histui prune](/docs/histui/commands/prune) - Remove old notifications
- [histui tui](/docs/histui/commands/tui) - Interactive browser
- [Filtering Guide](/docs/histui/filtering) - Advanced filter syntax

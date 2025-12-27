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
Without arguments, it returns all notifications.
With an index or ID, it returns a specific notification.

## Options

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--app` | string | | Filter by application name |
| `--urgency` | string | | Filter by urgency (low, normal, critical) |
| `--since` | duration | | Only show notifications from the last duration (e.g., 1h, 30m) |
| `--format` | string | dmenu | Output format: dmenu, json, plain |
| `--limit` | int | 0 | Maximum number of results (0 = unlimited) |

## Examples

### List all notifications

```bash
histui get
```

### Filter by application

```bash
histui get --app firefox
```

### Filter by time and urgency

```bash
histui get --since 1h --urgency critical
```

### JSON output for scripting

```bash
histui get --format json | jq '.[] | .summary'
```

### Use with dmenu/rofi

```bash
histui get | dmenu -l 10
```

## See Also

- [histui prune](/docs/histui/commands/prune) - Remove old notifications
- [Filtering Guide](/docs/histui/filtering) - Advanced filter syntax

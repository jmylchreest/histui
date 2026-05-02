---
title: histui set
description: Modify notification state
sidebar_label: set
sidebar_position: 9
---

# histui set

Modify notification state (dismiss, undismiss, seen, delete).

## Synopsis

```bash
histui set [id...] [flags]
```

## Description

The `set` command modifies the state of notifications in history.
You can dismiss, undismiss, mark as seen, or permanently delete notifications.

IDs can be provided as positional arguments or piped via stdin. When reading
from stdin, the command scans each line for a ULID pattern, making it compatible
with various output formats.

## Options

### Input Options

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--stdin` | bool | false | Read IDs from stdin (scans for ULID pattern) |
| `--stdin-json` | bool | false | Read JSON from stdin and extract `histui_id` field |

### Action Options (mutually exclusive)

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--dismiss` | bool | false | Mark notification(s) as dismissed |
| `--undismiss` | bool | false | Clear dismissed state from notification(s) |
| `--seen` | bool | false | Mark notification(s) as seen |
| `--delete` | bool | false | Permanently delete notification(s) from history |

Exactly one action must be specified.

## Examples

### Dismiss a specific notification

```bash
histui set 01HZ3X2J5YFMK2V3P4Q6R7S8T9 --dismiss
```

### Dismiss multiple notifications

```bash
histui set ID1 ID2 ID3 --dismiss
```

### Pipeline: Dismiss all from an app

```bash
histui get --filter "app=discord" --format ids | histui set --stdin --dismiss
```

### Pipeline: Dismiss from JSON output

```bash
histui get --format json | histui set --stdin-json --dismiss
```

### Mark all from an app as seen

```bash
histui get --filter "app=slack" --format ids | histui set --stdin --seen
```

### Delete old notifications

```bash
histui get --filter "timestamp<7d" --format ids | histui set --stdin --delete
```

### Dismiss all critical notifications

```bash
histui get --filter "urgency=critical,dismissed=false" --format ids | histui set --stdin --dismiss
```

## Pipeline Workflows

### Interactive dismiss with fuzzel

```bash
# Select notifications to dismiss
histui get | fuzzel -d | histui set --stdin --dismiss
```

### Batch operations with fzf multi-select

```bash
# Multi-select with fzf, then dismiss
histui get --format json | jq -r '.[].histui_id' | fzf --multi | histui set --stdin --dismiss
```

### Delete and prune workflow

```bash
# Delete specific notifications, then prune to keep 500
histui get --filter "app=spam-app" --format ids | histui set --stdin --delete
histui prune --keep 500
```

## ULID Extraction

When using `--stdin`, the command scans each line for a ULID pattern
(26 alphanumeric characters). This means it works with:

- Bare ULIDs: `01HZ3X2J5YFMK2V3P4Q6R7S8T9`
- IDs format: `01HZ3X2J5YFMK2V3P4Q6R7S8T9`
- Any line containing a ULID

For dmenu-style output, use `--format ids` with `histui get` to get clean IDs.

## See Also

- [histui get](/docs/histui/commands/get) - Query notifications
- [histui prune](/docs/histui/commands/prune) - Remove old notifications
- [histui tui](/docs/histui/commands/tui) - Interactive browser with dismiss/delete

---
title: histui prune
description: Remove old notifications from history
sidebar_label: prune
sidebar_position: 2
---

# histui prune

Remove old notifications from history based on age or count.

## Synopsis

```bash
histui prune [flags]
```

## Description

The `prune` command removes notifications from history.
Use it to keep your notification history manageable.

## Options

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--older-than` | duration | | Remove notifications older than duration |
| `--keep` | int | | Keep only the most recent N notifications |
| `--app` | string | | Only prune notifications from specific app |
| `--dry-run` | bool | false | Show what would be deleted without deleting |

## Examples

### Remove notifications older than 7 days

```bash
histui prune --older-than 168h
```

### Keep only the last 100 notifications

```bash
histui prune --keep 100
```

### Preview what would be deleted

```bash
histui prune --older-than 24h --dry-run
```

## See Also

- [histui get](/docs/histui/commands/get) - Query notifications

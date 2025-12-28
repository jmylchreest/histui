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

The `prune` command removes notifications from history based on age or count limits.
Use it for manual cleanup - note that histuid also provides automatic pruning via the
`[history].max_notifications` config setting.

## Options

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--older-than` | duration | | Remove notifications older than duration (e.g., 48h, 7d, 1w) |
| `--keep` | int | 0 | Keep only the N most recent notifications (0 = unlimited) |
| `--dry-run` | bool | false | Show what would be removed without removing |

At least one of `--older-than` or `--keep` is required.

## Examples

### Remove notifications older than 7 days

```bash
histui prune --older-than 7d
```

### Keep only the last 100 notifications

```bash
histui prune --keep 100
```

### Preview what would be removed

```bash
histui prune --older-than 48h --dry-run
```

### Combine age and count limits

```bash
# Remove anything older than 1 week AND keep at most 500
histui prune --older-than 1w --keep 500
```

## Duration Format

Durations support various formats:
- `48h` - 48 hours
- `7d` - 7 days
- `1w` - 1 week
- `2w` - 2 weeks

## Automatic Pruning

For automatic pruning, configure histuid's retention settings:

```toml
[history]
max_notifications = 1000  # Auto-prune when exceeding this limit
```

See [histuid Configuration](/docs/histuid/configuration#history) for details.

## See Also

- [histui get](/docs/histui/commands/get) - Query notifications
- [histuid Configuration](/docs/histuid/configuration) - Automatic pruning settings

---
title: Command Reference
description: Complete reference for all histui commands
sidebar_position: 0
---

# histui Commands

Complete reference for all histui CLI commands.

## Query Commands

| Command | Description |
|---------|-------------|
| [get](/docs/histui/commands/get) | Query notification history, output in various formats |
| [apps](/docs/histui/commands/apps) | List unique application names from history |
| [status](/docs/histui/commands/status) | Output Waybar-compatible JSON status |

## Interactive

| Command | Description |
|---------|-------------|
| [tui](/docs/histui/commands/tui) | Launch interactive terminal UI |

## Actions

| Command | Description |
|---------|-------------|
| [replay](/docs/histui/commands/replay) | Replay notifications through D-Bus |
| [set](/docs/histui/commands/set) | Modify notification state (dismiss, seen, delete) |
| [prune](/docs/histui/commands/prune) | Remove old notifications from history |

## Control

| Command | Description |
|---------|-------------|
| [dnd](/docs/histui/commands/dnd) | Manage Do Not Disturb mode |
| [audio](/docs/histui/commands/audio) | Control audio playback |

## Common Workflows

### Browse and dismiss notifications

```bash
histui tui
```

### Select and replay with launcher

```bash
histui get | fuzzel -d | histui replay
```

### Dismiss all from an app

```bash
histui get --filter "app=discord" --format ids | histui set --stdin --dismiss
```

### Waybar integration

```bash
histui status  # Returns JSON for custom module
```

### Check DnD and toggle

```bash
histui dnd toggle
```

## Global Flags

These flags are available on all commands:

| Flag | Description |
|------|-------------|
| `--config` | Path to config file |
| `--log-level` | Log level: debug, info, warn, error |
| `-h`, `--help` | Help for any command |

## Environment Variables

| Variable | Description |
|----------|-------------|
| `HISTUI_LOG_LEVEL` | Override log level |
| `HISTUI_CONFIG` | Override config file path |

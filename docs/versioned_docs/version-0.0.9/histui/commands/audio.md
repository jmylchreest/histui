---
title: histui audio
description: Control audio playback
sidebar_label: audio
sidebar_position: 8
---

# histui audio

Control audio playback for histuid notification sounds.

## Synopsis

```bash
histui audio [command] [flags]
```

## Description

The `audio` command controls notification sound playback. Use it to stop
currently playing sounds or manage audio state.

## Subcommands

| Command | Description |
|---------|-------------|
| `stop` | Stop any currently playing notification sound |

## Options

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-q`, `--quiet` | bool | false | Suppress output |

## Examples

### Stop currently playing sound

```bash
histui audio stop
```

### Use in keybinding

Add to your Hyprland config:
```bash
bind = $mod, M, exec, histui audio stop
```

### Stop sound with toggle DnD

```bash
# Stop sound and enable DnD
histui audio stop && histui dnd on
```

## How It Works

The audio stop command sets a flag in the shared state file
(`~/.local/share/histui/state.json`). When histuid reads this flag,
it stops any currently playing sound and clears the flag.

This allows control even when histuid is running in the background
as a separate process.

## See Also

- [histui dnd](/docs/histui/commands/dnd) - Enable Do Not Disturb (also silences sounds)
- [histuid Configuration](/docs/histuid/configuration#audio) - Audio settings

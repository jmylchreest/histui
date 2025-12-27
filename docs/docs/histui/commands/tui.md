---
title: histui tui
description: Interactive terminal user interface
sidebar_label: tui
sidebar_position: 4
---

# histui tui

Launch an interactive terminal user interface for browsing notifications.

## Synopsis

```bash
histui tui [flags]
```

## Description

The `tui` command launches an interactive terminal interface built with BubbleTea.
Browse, search, and manage your notification history with keyboard navigation.

## Keybindings

| Key | Action |
|-----|--------|
| `j` / `Down` | Move down |
| `k` / `Up` | Move up |
| `/` | Start search |
| `Enter` | View notification details |
| `d` | Delete notification |
| `q` | Quit |

## Options

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--filter` | string | | Pre-apply a filter expression |

## Examples

### Launch TUI

```bash
histui tui
```

### Launch with filter

```bash
histui tui --filter "app:firefox"
```

## See Also

- [Filtering Guide](/docs/histui/filtering) - Filter syntax for the TUI

# histui

[![Build Release](https://github.com/jmylchreest/histui/actions/workflows/build-release.yml/badge.svg)](https://github.com/jmylchreest/histui/actions/workflows/build-release.yml)
[![Latest Tag](https://badgen.net/github/tag/jmylchreest/histui)](https://github.com/jmylchreest/histui/releases)

A terminal UI for browsing and managing notification history on Linux desktops.

## Features

- Browse notification history from dunst (more daemons planned)
- Search notifications by app, summary, or body
- Copy notification content to clipboard
- Dismiss or permanently delete notifications
- Persistent history across sessions
- Vim-style keybindings

## Quick Start

Download the latest [release binary](https://github.com/jmylchreest/histui/releases) and run:

```bash
# Launch the TUI
./histui

# Or import and list notifications via CLI
./histui get
```

## CLI Usage

### Output Formats

```bash
histui get                      # Default dmenu format (for fuzzel, rofi, etc.)
histui get --format json        # JSON output
histui get --format ids         # Just ULIDs, one per line (for piping)
histui get --format plain       # Plain text
```

### Filtering

Use `--filter` for expression-based filtering:

```bash
# Filter by app name
histui get --filter "app=discord"

# Multiple conditions (AND logic)
histui get --filter "app=slack,urgency=critical"

# Contains search
histui get --filter "body~meeting"

# Regex matching
histui get --filter "summary~=(?i)error|warning"

# Comparison operators
histui get --filter "urgency>=normal"
histui get --filter "timestamp>1h"          # Last hour
histui get --filter "dismissed=false"
```

**Supported fields:** `app`, `summary`, `body`, `urgency`, `category`, `dismissed`, `seen`, `timestamp`

**Operators:** `=` (equal), `!=` (not equal), `~` (contains), `~=` (regex), `>`, `<`, `>=`, `<=`

### Bulk Operations with Pipelines

The `set` command modifies notification state and can read IDs from stdin:

```bash
# Dismiss all Discord notifications
histui get --filter "app=discord" --format ids | histui set --stdin --dismiss

# Mark old notifications as seen
histui get --filter "timestamp>7d" --format ids | histui set --stdin --seen

# Delete dismissed notifications older than a week
histui get --filter "dismissed=true,timestamp>7d" --format ids | histui set --stdin --delete

# Undismiss notifications matching a pattern
histui get --filter "body~important" --format ids | histui set --stdin --undismiss
```

### Dmenu/Fuzzel Workflow

```bash
# Pick notification and copy body to clipboard
histui get | fuzzel -d | cut -d'|' -f1 | xargs histui get --field body | wl-copy

# Pick and dismiss
histui get --filter "dismissed=false" | fuzzel -d | cut -d'|' -f1 | xargs histui set --dismiss
```

## Keybindings

| Key | Action |
|-----|--------|
| `j/k` or arrows | Navigate up/down |
| `enter` | View notification details |
| `/` | Search |
| `d` | Dismiss/undismiss notification |
| `D` | Delete permanently |
| `a` | Toggle showing dismissed |
| `c` | Copy body to clipboard |
| `s` | Copy summary to clipboard |
| `?` | Show help |
| `q` | Quit |

## Configuration

Configuration file is created at `~/.config/histui/config.toml` on first run.

History is stored at `~/.local/share/histui/history.jsonl`.

## Waybar Integration

histui includes a status command for Waybar integration. See [contrib/waybar](contrib/waybar/) for full examples.

```jsonc
"custom/notifications": {
  "exec": "histui status --all --since 24h",
  "interval": 5,
  "return-type": "json",
  // Middle click: floating TUI
  "on-click-middle": "hyprctl dispatch exec '[float;size 900 600;center] kitty --class histui-float -e histui'",
  // Right click: dmenu picker (copies body to clipboard)
  "on-click-right": "histui get | fuzzel -d | cut -d'|' -f1 | xargs histui get --field body | wl-copy"
}
```

## License

MIT License - see [LICENSE](LICENSE) for details.

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

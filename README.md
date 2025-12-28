# histui

[![Build Release](https://github.com/jmylchreest/histui/actions/workflows/build-release.yml/badge.svg)](https://github.com/jmylchreest/histui/actions/workflows/build-release.yml)
[![Latest Release](https://img.shields.io/github/v/release/jmylchreest/histui)](https://github.com/jmylchreest/histui/releases)
[![AUR Package](https://img.shields.io/aur/version/histui-bin)](https://aur.archlinux.org/packages/histui-bin)
[![License](https://img.shields.io/github/license/jmylchreest/histui)](LICENSE)

**A highly themeable GTK4 notification daemon for Wayland with persistent history.**

histui displays desktop notifications with full CSS theming support, stores them for later browsing, and includes a TUI and CLI for querying your notification history.

**[Documentation](https://jmylchreest.github.io/histui)** | **[Releases](https://github.com/jmylchreest/histui/releases)**

[![histui showcase](https://img.youtube.com/vi/kFPKzW_4zOg/maxresdefault.jpg)](https://youtu.be/kFPKzW_4zOg)

[Watch the demo video](https://youtu.be/kFPKzW_4zOg)

## Features

### Notification Daemon (histuid)

- **Fully themeable** - Create your own themes with CSS, custom layouts, icons, and sounds
- **Theme packs** - Self-contained themes with styling, layout, and audio in one directory
- **Hot reload** - Edit themes live without restarting
- **Bundled themes** - Includes default, minimal, compact, detailed, and catppuccin
- **Light/dark mode** - Automatic switching based on system preference
- **Smart icons** - App aliases with Nerd Font fallbacks for 350+ apps
- **Clickable notifications** - URLs and deep links open the source app
- **Audio alerts** - Per-urgency sounds with customizable audio files
- **Wayland native** - Layer-shell support for Hyprland, Sway, river, and more

### History & CLI (histui)

- **Persistent history** - Notifications saved to SQLite across sessions
- **TUI browser** - Navigate history with vim-style keybindings
- **Powerful filtering** - Query by app, urgency, time range, regex
- **Pipeline friendly** - JSON, dmenu, and ID output formats
- **Waybar integration** - Real-time notification counts

### Flexible Deployment

- **Standalone daemon** - Replace dunst/mako entirely
- **Monitor mode** - Keep your existing daemon, just add history tracking
- **Adapter system** - Import history from dunst, mako, or swaync

## Quick Start

```bash
# Install (Arch Linux)
yay -S histui-bin

# Start the notification daemon
systemctl --user enable --now histuid

# Browse your history
histui
```

## Theming

Create custom themes with CSS and optional layout/audio:

```
~/.config/histui/themes/mytheme/
├── theme.css        # CSS styling
├── layout.xml       # Widget layout (optional)
├── manifest.toml    # Metadata and audio (optional)
└── sounds/          # Audio files (optional)
```

Or extend existing themes:

```css
/* ~/.config/histui/themes/my-colors/theme.css */
@import "catppuccin.css";

.notification-popup {
    background-color: #1e1e2e;
    border-radius: 16px;
}
```

See the [Theming Guide](https://jmylchreest.github.io/histui/docs/histuid/theming) for full documentation.

## CLI Examples

```bash
# List recent notifications
histui get

# Filter by app and time
histui get --filter "app=discord,timestamp>1h"

# Fuzzel picker
histui get | fuzzel -d | cut -d'|' -f1 | xargs histui get --field body | wl-copy

# Dismiss all Slack notifications
histui get --filter "app=slack" --format ids | histui set --stdin --dismiss
```

## Waybar Integration

```jsonc
"custom/notifications": {
  "exec": "histui status --all --since 24h",
  "interval": 5,
  "return-type": "json",
  "on-click-middle": "hyprctl dispatch exec '[float;size 900 600;center] kitty -e histui'"
}
```

## Configuration

- Daemon config: `~/.config/histui/histuid.toml`
- CLI config: `~/.config/histui/config.toml`
- Themes: `~/.config/histui/themes/`
- History: `~/.local/share/histui/history.db`

## License

MIT License - see [LICENSE](LICENSE) for details.

---
title: Icon Aliases
description: Map application names to icons and Nerd Font symbols
sidebar_position: 8
---

# Icon Aliases

histui uses an icon resolution system to display appropriate icons for notifications. This page explains how icon aliases work and how to customize them.

## How Icon Resolution Works

When a notification arrives, histui resolves the icon in this order:

1. **User custom aliases** - Your personal mappings take highest priority
2. **Built-in aliases** - Embedded default mappings
3. **Original name** - If no alias found, use the app name as-is

The resolver normalizes all names to lowercase for case-insensitive matching.

## Icon Aliases File

Create `~/.config/histui/icon-aliases.toml` to customize icon mappings:

```toml
# Map app names to icon names
[aliases]
my-custom-app = "firefox"
another-app = "terminal"

# Map icon names to Nerd Font glyphs
[symbols]
my-custom-icon = "\U0000E658"  # Firefox Nerd Font symbol
```

### Aliases Section

Maps Linux application names (package names, desktop file names) to canonical icon names:

```toml
[aliases]
# Variants map to canonical names
firefox-esr = "firefox"
firefox-nightly = "firefox"
librewolf = "firefox"

# Custom app mappings
my-chat-client = "discord"
work-email = "email"
```

### Symbols Section

Maps icon names to Nerd Font glyph characters:

```toml
[symbols]
# Application icons
discord = "\U000F066F"    # nf-md-discord
firefox = "\U0000E658"    # nf-dev-firefox
terminal = "\U000F0257"   # nf-md-console

# Category fallbacks
notification = "\U000F009A"  # nf-md-bell
im = "\U000F0CE4"            # nf-md-chat
```

## Built-in Aliases

histui includes comprehensive built-in aliases for common applications:

| Category | Examples |
|----------|----------|
| Messaging | discord, slack, telegram, whatsapp, signal |
| Browsers | firefox, chrome, brave, edge, opera |
| Email | thunderbird, evolution, geary, kmail |
| Media | spotify, vlc, rhythmbox, mpv |
| Development | vscode, terminal, git, docker |
| System | nautilus, settings, keyring |

The built-in aliases cover hundreds of Linux applications and their variants.

## Nerd Font Symbols

When the TUI or notification popup displays icons, it can use Nerd Font glyphs as fallbacks. The symbol lookup order is:

1. Icon name in user symbols
2. Icon name in built-in symbols
3. Category-based fallback (im, device, transfer, presence)
4. Urgency-based fallback (low, normal, critical)
5. Default notification symbol

### Urgency Symbols

Built-in urgency fallbacks:

| Urgency | Symbol | Codepoint |
|---------|--------|-----------|
| low | (info) | `\U000F02FC` |
| normal | (bell) | `\U000F009A` |
| critical | (alert) | `\U000F0026` |

### Category Symbols

Built-in category fallbacks (from freedesktop notification spec):

| Category | Symbol | Codepoint |
|----------|--------|-----------|
| im | (chat) | `\U000F0CE4` |
| device | (harddisk) | `\U000F03CF` |
| transfer | (download) | `\U000F01DA` |
| presence | (account) | `\U000F0061` |

## Duplicate Handling

The resolver warns about duplicate aliases but keeps the first value found. This ensures:

- User aliases always take priority over built-in
- Canonical icon names are never accidentally aliased to different icons

Example warning:
```
WARN duplicate icon alias, keeping first app=alacritty existing=terminal ignored=console_network
```

## Generating Aliases

The `contrib/generate-icon-aliases` tool generates the built-in aliases from Nerd Fonts glyph data. It can also generate custom aliases using AI.

### Basic Usage

```bash
cd contrib/generate-icon-aliases
go build .

# Generate aliases from Nerd Fonts data
./generate-icon-aliases --output icon-aliases.toml

# Fetch fresh glyph data from GitHub
./generate-icon-aliases --fetch --output icon-aliases.toml
```

### AI-Enhanced Generation

The generator supports AI-powered knowledge base generation:

```bash
# Generate AI knowledge base (requires OPENROUTER_API_KEY)
export OPENROUTER_API_KEY="your-key"
./generate-icon-aliases --generate-kb

# Use the AI knowledge base
./generate-icon-aliases --output icon-aliases.toml
```

### Generator Options

| Flag | Description |
|------|-------------|
| `--fetch` | Download fresh glyphnames.json from GitHub |
| `--output` | Output TOML file path (default: icon-aliases.toml) |
| `--prefer` | Icon set preference: md, fa, dev (default: md) |
| `--verbose` | Show detailed matching information |
| `--generate-kb` | Generate AI knowledge base |
| `--no-cache` | Disable API response caching |

### Icon Set Preferences

The `--prefer` flag selects which icon set to prioritize:

| Value | Description |
|-------|-------------|
| `md` | Material Design icons (default) |
| `fa` | Font Awesome icons |
| `dev` | Devicons |

### Overrides

Create `kb-overrides.toml` to customize generated aliases:

```toml
# Replace app list entirely
[icons]
discord = ["my-discord-fork", "custom-discord"]

# Add apps to existing mapping
[additions]
email = ["my-email-client"]

# Remove apps from mapping
[exclusions]
email = ["thunderbird"]

# Force specific glyph
[glyph_overrides]
discord = "fa-discord"
```

## Source Code

The icon generator is available at:
[github.com/jmylchreest/histui/tree/main/contrib/generate-icon-aliases](https://github.com/jmylchreest/histui/tree/main/contrib/generate-icon-aliases)

## Icons vs Images in Notifications

The freedesktop.org notification spec defines two distinct visual elements:

| Element | Source | Purpose |
|---------|--------|---------|
| **Icon** | `app_icon` parameter | Application's identifying icon (header/sidebar) |
| **Image** | `image-data` or `image-path` hints | Notification content image (body area) |

### How histui/histuid Renders Icons

The **icon** (displayed in header/sidebar) is resolved in this order:

1. **app_icon parameter** - The icon name/path sent by the application
   - Icon names (e.g., `firefox`) are looked up in the GTK icon theme
   - File paths (e.g., `/usr/share/icons/app.png`) are loaded directly
2. **Icon alias resolution** - User aliases → built-in aliases
3. **Nerd Font symbol fallback** - Based on app name, category, or urgency

### How histui/histuid Renders Images

The **image** (displayed in body area) follows the freedesktop spec priority:

1. **image-data hint** - Raw pixel data embedded in the notification
2. **image-path hint** - File path to an image

However, many messaging apps (Signal, Discord) misuse `image-data` for profile pictures when semantically they should be icons. To handle this:

```toml
# ~/.config/histui/histuid.toml
[display]
# Control image-data display in notification body
# "never"    - Never show image-data (profile pics won't appear)
# "always"   - Always show image-data
# "100 KiB"  - Only show if raw data >= threshold (filters small profile pics)
image_data_preview_size = "100 KiB"  # default
```

| Setting | Effect |
|---------|--------|
| `"never"` or `-1` | Never display image-data in body |
| `"always"` or `0` | Always display image-data in body |
| `"100 KiB"` | Only display if data size ≥ 100 KiB |

The default `100 KiB` filters out profile pictures (typically 64-128px = ~64KB raw) while showing larger content like album art (300px+ = ~360KB+ raw).

:::tip
The `image-path` hint is **always** displayed since explicit file paths indicate intentional content images (screenshots, etc.).
:::

### Real-World App Behavior

| App | app_icon | image-data | Typical Use |
|-----|----------|------------|-------------|
| Signal | `signal-desktop` | Profile pic (~96px) | Profile = icon-ish |
| Discord | `discord` | Profile pic (~128px) | Profile = icon-ish |
| Spotify | `spotify` | Album art (~300px+) | Content image |
| Browser | `firefox` | Screenshot (large) | Content image |

## See Also

- [Manifest Reference](/docs/histuid/theming/manifest-reference) - Icon size configuration
- [Layout Reference](/docs/histuid/theming/layout-reference) - Icon element in layouts
- [CSS Reference](/docs/histuid/theming/css-reference) - Styling notification icons

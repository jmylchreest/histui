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

# Map icon names to Nerd Font glyphs (use actual Unicode characters)
[symbols]
my-custom-icon = "󰈹"  # Firefox Nerd Font symbol (paste the actual glyph)
```

:::tip Symbol Format
The `[symbols]` section uses actual Unicode characters, not escape sequences. Copy glyphs directly from a [Nerd Fonts cheat sheet](https://www.nerdfonts.com/cheat-sheet) into your TOML file.
:::

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

Maps icon names to Nerd Font glyph characters. Use actual Unicode characters (paste from Nerd Fonts cheat sheet):

```toml
[symbols]
# Application icons (paste actual glyphs)
discord = "󰙯"    # nf-md-discord
firefox = "󰈹"    # nf-md-firefox
terminal = "󰆍"   # nf-md-console

# Category fallbacks
notification = "󰂚"  # nf-md-bell
im = "󱃲"            # nf-md-chat
```

:::note Finding Glyphs
Use the [Nerd Fonts Cheat Sheet](https://www.nerdfonts.com/cheat-sheet) to search for icons and copy the glyph character directly.
:::

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

| Urgency | Symbol | Description |
|---------|--------|-------------|
| low | 󰋼 | Info circle |
| normal | 󰂚 | Bell |
| critical | 󰀦 | Alert |

### Category Symbols

Built-in category fallbacks (from freedesktop notification spec):

| Category | Symbol | Description |
|----------|--------|-------------|
| im | 󱃲 | Chat bubble |
| device | 󰋊 | Hard disk |
| transfer | 󰇚 | Download |
| presence | 󰀄 | Account |

## Duplicate Handling

The resolver warns about duplicate aliases but keeps the first value found. This ensures:

- User aliases always take priority over built-in
- Canonical icon names are never accidentally aliased to different icons

Example warning:
```
WARN duplicate icon alias, keeping first app=alacritty existing=terminal ignored=console_network
```

## Generating Aliases

The `contrib/generate-icon-aliases` tool generates the built-in aliases from Nerd Fonts glyph data and upstream icon metadata. It uses AI to map Linux applications to appropriate icons.

### Workflow

The generator uses a multi-step workflow:

1. **Fetch** - Download upstream icon metadata (Font Awesome, Material Design, Devicons, Codicons)
2. **Generate KB** - Use AI to generate application-to-icon mappings
3. **Output** - Generate the final `icon-aliases.toml` with manual overrides applied

### Quick Start

```bash
cd contrib/generate-icon-aliases

# Build the generator
go build .

# Generate output using existing knowledge base
./generate-icon-aliases --output ../../internal/icon/aliases_default.toml

# Or use the Taskfile from project root
task generate:icons:output
```

### Full Regeneration

To regenerate everything from scratch (requires OpenRouter API key):

```bash
# Fetch fresh upstream metadata
./generate-icon-aliases --fetch

# Generate AI knowledge base
export OPENROUTER_API_KEY="your-key"
./generate-icon-aliases --generate-kb

# Generate final output
./generate-icon-aliases --output ../../internal/icon/aliases_default.toml
```

### Generator Flags

| Flag | Description |
|------|-------------|
| `--fetch` | Fetch upstream icon metadata and regenerate patterns |
| `--generate-kb` | Generate AI knowledge base (requires API key) |
| `--output` | Output TOML file path |
| `--font-output` | Also download Nerd Font symbols TTF |
| `--verbose` | Show detailed matching information |

### Manual Overrides

Create or edit `kb-patterns-manual.toml` to customize mappings. Manual overrides take highest priority over AI-generated mappings.

```toml
# Force specific apps to use a specific icon
[icons.magnet]
patterns = ["md-magnet", "fa-magnet"]
type = "category"
description = "BitTorrent and download clients"
upstream = "manual"
force_apps = [
    "qbittorrent",
    "transmission",
    "deluge",
    "aria2",
]

# Override an existing icon's app list
[icons.firefox]
patterns = ["md-firefox", "fa-firefox"]
type = "app"
description = "Firefox web browser"
upstream = "manual"
force_apps = [
    "firefox",
    "firefox-esr",
    "firefox-developer-edition",
    "librewolf",
]
```

### Override Fields

| Field | Description |
|-------|-------------|
| `patterns` | Nerd Font glyph patterns to match (e.g., `md-discord`) |
| `type` | `"app"` for brand icons, `"category"` for generic icons |
| `description` | Human-readable description |
| `force_apps` | List of app names - replaces AI-generated list entirely |
| `extra_apps` | List of app names - adds to AI-generated list |

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

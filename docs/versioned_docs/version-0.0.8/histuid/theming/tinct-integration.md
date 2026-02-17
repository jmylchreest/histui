---
title: Dynamic Themes with Tinct
description: Automatically match notification colors to your wallpaper using Tinct
sidebar_position: 8
---

# Dynamic Themes with Tinct

[Tinct](https://github.com/jmylchreest/tinct) is a color palette generator that extracts colors from images (like your wallpaper) and generates theme files for various applications. As of **Tinct v0.1.19**, histuid is a supported output format.

This integration enables dynamic, wallpaper-matched notification colors that update automatically when you change your wallpaper.

## How It Works

Tinct generates a `tinct-colors.css` file containing:

1. **libadwaita named color overrides** (`@define-color`) - automatically used by histuid's default theme
2. **CSS custom properties** (`--tinct-*`) - for use in custom themes

When you import `tinct-colors.css` before the default theme, the color overrides take effect and your notifications automatically use your wallpaper's color palette.

## Quick Setup

### 1. Install Tinct

```bash
# From AUR (Arch Linux)
yay -S tinct

# Or build from source
go install github.com/jmylchreest/tinct@latest
```

### 2. Generate Colors

```bash
# Generate histui colors from your wallpaper
tinct generate -i image -o histui ~/Pictures/wallpaper.jpg
```

This creates `~/.config/histui/themes/tinct-colors.css`.

### 3. Create the Tinct Theme

Create a theme directory and CSS file that imports the generated colors:

```bash
mkdir -p ~/.config/histui/themes/tinct
```

Create `~/.config/histui/themes/tinct/theme.css`:

```css
/* histui Tinct Theme
 * Uses tinct-generated colours on top of the default histui theme.
 *
 * IMPORTANT: Import order matters!
 * 1. tinct-colors.css - defines @define-color overrides for libadwaita colors
 * 2. default theme - uses the overridden colors
 *
 * To generate the tinct-colors.css file, run:
 *   tinct generate -i image -o histui <your-wallpaper.jpg>
 */

/* Import tinct colours FIRST - this overrides libadwaita named colors */
@import "tinct-colors.css";

/* Import the default theme - it will use our overridden colors */
@import "default/theme.css";
```

### 4. Enable the Theme

```toml
# ~/.config/histui/histuid.toml
[theme]
name = "tinct"
```

## Understanding tinct-colors.css

The generated file provides two ways to use the colors:

### libadwaita Named Colors (Automatic)

These are picked up automatically by the default theme:

```css
/* Window colors */
@define-color window_bg_color #28323b;
@define-color window_fg_color #eaeff3;

/* Accent colors */
@define-color accent_bg_color #afe8f8;
@define-color accent_fg_color #000000;
@define-color accent_color #afe8f8;

/* Semantic/status colors */
@define-color error_color #fb4c36;
@define-color warning_color #e5bf4c;
@define-color success_color #4ce54c;

/* And more... */
```

### CSS Custom Properties (For Custom Themes)

For advanced customization, use the `--tinct-*` variables:

```css
* {
    /* Core colours */
    --tinct-background: #28323b;
    --tinct-background-muted: #4f5860;
    --tinct-foreground: #eaeff3;
    --tinct-foreground-muted: #c0c9cf;

    /* Accent colours */
    --tinct-accent1: #afe8f8;
    --tinct-accent2: #c2d4e3;
    --tinct-accent3: #7cc2ec;
    --tinct-accent4: #a1b0c0;

    /* Semantic colours */
    --tinct-danger: #fb4c36;
    --tinct-warning: #e5bf4c;
    --tinct-success: #4ce54c;
    --tinct-info: #43c9ee;

    /* Surface colours */
    --tinct-surface: #34404a;
    --tinct-on-surface: #eaeff3;
    --tinct-border: #6a737a;

    /* Container elevation levels */
    --tinct-surface-container-lowest: #2b353e;
    --tinct-surface-container-low: #2f3a44;
    --tinct-surface-container: #34404a;
    --tinct-surface-container-high: #38454f;
    --tinct-surface-container-highest: #3c4a55;
}
```

## Custom Theme with Tinct Variables

For more control, create a custom theme using the CSS variables:

```css
/* ~/.config/histui/themes/tinct-custom/theme.css */
@import "tinct-colors.css";
@import "default/theme.css";

/* Custom overrides using tinct variables */
.notification-popup {
    background-color: alpha(var(--tinct-surface-container-high), 0.95);
    border-color: var(--tinct-accent1);
    border-width: 2px;
}

.notification-popup.urgency-critical {
    border-color: var(--tinct-danger);
    background-color: alpha(var(--tinct-surface-container-highest), 0.98);
}

.notification-summary {
    color: var(--tinct-accent1);
}
```

## Automatic Wallpaper Updates

### With Hyprland

Add to your wallpaper change script or Hyprland config:

```bash
#!/bin/bash
# ~/.config/hypr/scripts/wallpaper.sh

WALLPAPER="$1"

# Set wallpaper
hyprctl hyprpaper wallpaper ",$WALLPAPER"

# Regenerate tinct colors
tinct generate -i image -o histui "$WALLPAPER"

# histuid will automatically hot-reload the theme!
```

### With swww

```bash
#!/bin/bash
swww img "$1" --transition-type grow

# Regenerate colors - histuid hot-reloads automatically
tinct generate -i image -o histui "$1"
```

### With systemd User Service

Create a service that watches for wallpaper changes:

```ini
# ~/.config/systemd/user/tinct-histui.service
[Unit]
Description=Generate histui colors from wallpaper
After=graphical-session.target

[Service]
Type=oneshot
ExecStart=/usr/bin/tinct generate -i image -o histui %h/Pictures/current-wallpaper.jpg

[Install]
WantedBy=default.target
```

## Hot Reload

histuid automatically watches `tinct-colors.css` for changes. When Tinct regenerates the color file (e.g., on wallpaper change), your notifications update instantly without restarting the daemon.

The watch includes all imported files, so changes to either `tinct-colors.css` or your theme's `theme.css` trigger a reload.

## Troubleshooting

### Colors Not Updating

1. Verify the import order in your theme.css - `tinct-colors.css` must be imported BEFORE `default/theme.css`
2. Check that histuid is using the tinct theme: `histui status`
3. Verify tinct-colors.css was generated: `ls -la ~/.config/histui/themes/tinct-colors.css`
4. Check histuid logs for import resolution: `journalctl --user -u histuid -f`

### Debug Import Resolution

Run histuid with debug logging to see import resolution:

```bash
histuid --log-level debug
```

Look for lines like:
```
level=DEBUG msg="import resolved (user)" path=/home/user/.config/histui/themes/tinct-colors.css
level=DEBUG msg="watching imported files" count=1 files=[/home/user/.config/histui/themes/tinct-colors.css]
```

### Light/Dark Mode

Tinct generates colors for either light or dark mode based on the image analysis. You can force a specific mode:

```bash
# Force dark theme generation
tinct generate -i image -o histui --dark ~/Pictures/wallpaper.jpg

# Force light theme generation
tinct generate -i image -o histui --light ~/Pictures/wallpaper.jpg
```

## See Also

- [Tinct GitHub Repository](https://github.com/jmylchreest/tinct) - Tinct documentation and releases
- [Extending Themes](/docs/histuid/theming/extending) - More on CSS imports and inheritance
- [CSS Imports and Inheritance](/docs/histuid/theming/css-inheritance) - Deep dive into @import resolution
- [Theming Overview](/docs/histuid/theming) - Getting started with themes

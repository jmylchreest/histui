---
title: Theming Overview
description: Customize histuid notification appearance with CSS
sidebar_position: 1
---

# Theming

histuid uses GTK4 CSS for styling notification popups.
You can customize colors, fonts, borders, shadows, and more.

## Theme File Location

Create or edit your theme file:

```bash
mkdir -p ~/.config/histui/themes
touch ~/.config/histui/themes/mytheme.css
```

Then reference it in your config:

```toml
# ~/.config/histui/histuid.toml
[theme]
name = "mytheme"
```

## Theme Resolution Order

histuid checks for themes in this order:

1. **User themes directory**: `~/.config/histui/themes/`
2. **Bundled themes**: Embedded in the binary

This allows you to override bundled themes by placing a file with the same name in your themes directory.

## Bundled Themes

histuid ships with these bundled themes:

| Theme     | Description                                      |
|-----------|--------------------------------------------------|
| `default` | Libadwaita-style with system colors              |
| `minimal` | Clean, distraction-free notifications            |
| `dark`    | High-contrast dark theme (Catppuccin-inspired)   |
| `light`   | Clean white theme with soft shadows              |

## Quick Example

```css
/* Dark theme with rounded corners */
.notification-popup {
  background-color: #1e1e2e;
  color: #cdd6f4;
  border-radius: 12px;
  padding: 12px;
}

.notification-summary {
  font-weight: bold;
  font-size: 14px;
}

.notification-body {
  font-size: 12px;
  opacity: 0.9;
}
```

## Light/Dark Mode

histuid supports automatic light/dark mode switching based on system preference:

```toml
# ~/.config/histui/histuid.toml
[theme]
name = "default"
color_scheme = "system"  # "system", "light", or "dark"
```

| Value    | Behavior                                           |
|----------|----------------------------------------------------|
| `system` | Follows system preference (libadwaita StyleManager)|
| `light`  | Always use light mode                              |
| `dark`   | Always use dark mode                               |

The `.light` or `.dark` class is applied to `.notification-popup` based on the color scheme.

## Hot Reload

Theme changes are automatically reloaded when the CSS file is modified.
You can edit your theme in real-time without restarting the daemon.

## Next Steps

- [CSS Reference](/docs/histuid/theming/css-reference) - All CSS selectors and classes
- [Theme Examples](/docs/histuid/theming/examples) - Ready-to-use themes

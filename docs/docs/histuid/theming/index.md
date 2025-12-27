---
title: Theming Overview
description: Customize histuid notification appearance with CSS
sidebar_position: 1
---

# Theming

histuid uses GTK4 CSS for styling notification popups.
You can customize colors, fonts, borders, shadows, and more.

## Theme Packs

Themes are self-contained packs with styling, layout, and optional audio:

```
~/.config/histui/themes/mytheme/
├── theme.css           # Required: CSS styling
├── layout.xml          # Optional: Widget layout
├── manifest.toml       # Optional: Metadata and audio config
└── sounds/             # Optional: Audio files
    └── notify.wav
```

Create a theme and reference it in your config:

```bash
mkdir -p ~/.config/histui/themes/mytheme
```

```toml
# ~/.config/histui/histuid.toml
[theme]
name = "mytheme"
```

## Theme Resolution Order

histuid checks for themes in this order:

1. **User themes directory**: `~/.config/histui/themes/`
2. **Bundled themes**: Embedded in the binary

This allows you to override bundled themes by placing a directory with the same name in your themes directory.

## Bundled Themes

histuid ships with these bundled themes:

| Theme       | Description                                      |
|-------------|--------------------------------------------------|
| `default`   | Libadwaita-style with 90% opacity, 48px icons    |
| `minimal`   | Summary and body only, no icons, compact         |
| `compact`   | Smaller icons (32px), condensed layout           |
| `detailed`  | Full layout with timestamp                       |
| `catppuccin`| Catppuccin Mocha/Latte color scheme              |

## Quick Example

```css
/* Dark theme with rounded corners */
.notification-popup {
  background-color: alpha(#1e1e2e, 0.9);
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

Theme changes are automatically reloaded when theme files are modified.
You can edit your theme in real-time without restarting the daemon.

## Next Steps

- [CSS Reference](/docs/histuid/theming/css-reference) - All CSS selectors and classes
- [Theme Examples](/docs/histuid/theming/examples) - Ready-to-use themes
- [Advanced Theming](/docs/histuid/theming/advanced) - Theme pack structure, layouts, manifests, and icons

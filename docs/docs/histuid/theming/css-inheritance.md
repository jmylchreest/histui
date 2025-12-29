---
title: CSS Imports and Inheritance
description: How CSS @import resolution, inheritance, and the cascade work in histuid themes
sidebar_position: 5
---

# CSS Imports and Inheritance

histuid themes use standard CSS `@import` statements for inheritance. This enables partial themes that override only specific styles while inheriting everything else from a base theme.

## How @import Works

When histuid loads a theme, it processes all `@import` statements by:

1. Resolving the import path to a CSS file
2. Recursively processing any `@import` statements in that file
3. Inlining the resolved CSS in place of the `@import`

This happens at load time, so the final CSS applied to notifications is a single resolved stylesheet.

## Import Resolution Order

When you write `@import "something.css"`, histuid searches for the file in this order:

1. **Relative to current file** - The directory containing the CSS file with the import
2. **User themes directory** - `~/.config/histui/themes/`
3. **Embedded themes** - Built into the histuid binary

The first match wins. This allows you to override embedded themes by placing files in your user themes directory.

## Supported Import Formats

histuid recognizes several `@import` syntaxes:

```css
/* Standard CSS imports */
@import "filename.css";
@import 'filename.css';
@import url("filename.css");
@import url('filename.css');

/* All of these are equivalent */
@import "default.css";        /* Import embedded 'default' theme */
@import "default/theme.css";  /* Same - imports 'default' theme */
```

## Import Path Examples

### Importing Embedded Themes

The simplest case - import a bundled theme as your base:

```css
/* Import the default theme */
@import "default.css";

/* Or import other bundled themes */
@import "minimal.css";
@import "catppuccin.css";
@import "compact.css";
```

### Importing User Themes

Import themes from `~/.config/histui/themes/`:

```css
/* Import another user theme */
@import "my-base-theme/theme.css";

/* Import a single-file user theme */
@import "my-colors.css";
```

### Importing Relative Files

When building a theme pack with multiple CSS files:

```css
/* In mytheme/theme.css */
@import "./colors.css";        /* mytheme/colors.css */
@import "./partials/base.css"; /* mytheme/partials/base.css */
```

### Importing Shared Partials

Place shared CSS files directly in the themes directory for reuse:

```
~/.config/histui/themes/
├── _colors.css          # Shared partial (underscore prefix)
├── _fonts.css           # Another shared partial
├── theme-a/
│   └── theme.css        # @import "_colors.css";
└── theme-b/
    └── theme.css        # @import "_colors.css";
```

```css
/* In theme-a/theme.css */
@import "default.css";
@import "_colors.css";   /* Loads ~/.config/histui/themes/_colors.css */
```

:::tip Underscore Convention
Files starting with `_` (like `_colors.css`) are treated as partials - they're meant to be imported, not used as standalone themes. They won't appear in theme listings.
:::

## CSS Cascade and Specificity

CSS rules are applied in cascade order. Later rules override earlier ones (assuming equal specificity):

```css
@import "default.css";  /* Base styles loaded first */

/* Your overrides come after, so they win */
.notification-popup {
    background-color: #2d2d44;  /* Overrides default's background */
}
```

### Specificity Rules

GTK4 CSS follows standard CSS specificity:

1. **Element selectors**: `window`, `box` (lowest specificity)
2. **Class selectors**: `.notification-popup`, `.urgency-critical`
3. **Compound selectors**: `.notification-popup.urgency-critical` (higher specificity)
4. **Important declarations**: `color: red !important;` (highest, avoid if possible)

```css
/* Low specificity - easily overridden */
.notification-popup {
    color: white;
}

/* Higher specificity - only applies to critical notifications */
.notification-popup.urgency-critical {
    color: red;  /* This doesn't override the above, it's a different selector */
}

/* Same specificity as first rule - later rule wins */
.notification-popup {
    color: blue;  /* Overrides white */
}
```

## Circular Import Prevention

histuid tracks which files have been imported and prevents circular imports:

```css
/* theme-a.css */
@import "theme-b.css";

/* theme-b.css */
@import "theme-a.css";  /* Prevented - produces a comment instead */
```

When a circular import is detected, histuid inserts a comment:
```css
/* circular import prevented: theme-a.css */
```

## Import Debugging

When imports are processed, histuid adds comments showing the resolution:

```css
/* imported: default.css */
/* ... default theme CSS ... */

/* imported (user): _colors.css */
/* ... user colors CSS ... */

/* imported (embedded): minimal.css */
/* ... embedded minimal theme CSS ... */
```

If an import fails:
```css
/* import failed: nonexistent.css - file not found */
```

## Complete Example: Partial Theme

Here's a complete example of a partial theme that inherits from default:

```css
/* ~/.config/histui/themes/my-theme.css */

/* Start with the default theme */
@import "default.css";

/* Override CSS custom properties (use * not :root for GTK CSS) */
* {
    --histui-font-family: "JetBrains Mono", monospace;
    --histui-font-size: 13px;
}

/* Override specific styles */
.notification-popup {
    background-color: rgba(30, 30, 46, 0.95);
    border-radius: 8px;
    border: 2px solid #45475a;
}

/* Add per-app styling */
.notification-popup.app-discord {
    border-left: 4px solid #5865F2;
}

.notification-popup.app-slack {
    border-left: 4px solid #4A154B;
}

/* Override urgency colors */
.notification-popup.urgency-critical {
    background-color: rgba(68, 45, 45, 0.95);
    border-color: #f38ba8;
}
```

## Directory Theme with Multiple Files

For complex themes, use a directory structure:

```
~/.config/histui/themes/corporate/
├── theme.css
├── colors.css
├── typography.css
└── components/
    ├── header.css
    ├── body.css
    └── actions.css
```

**theme.css** (entry point):
```css
/* Import base theme */
@import "default.css";

/* Import local partials */
@import "./colors.css";
@import "./typography.css";
@import "./components/header.css";
@import "./components/body.css";
@import "./components/actions.css";

/* Any final overrides */
.notification-popup {
    margin: 8px;
}
```

## Dynamic Color Integration

External tools can write CSS files that your theme imports:

```css
/* ~/.config/histui/themes/dynamic/theme.css */
@import "default.css";
@import "pywal-colors.css";  /* Generated by pywal */

.notification-popup {
    background-color: var(--background);
    color: var(--foreground);
    border-color: var(--color4);
}
```

When the external tool regenerates `pywal-colors.css`, histuid's hot-reload automatically picks up the changes.

## GTK4 CSS Variables

histuid themes can use libadwaita CSS variables for system integration:

| Variable | Purpose |
|----------|---------|
| `@window_bg_color` | Window background |
| `@window_fg_color` | Window foreground (text) |
| `@borders` | Border color |
| `@accent_bg_color` | Accent background |
| `@accent_fg_color` | Accent foreground |
| `@accent_color` | Primary accent color |
| `@error_color` | Error/danger color |
| `@success_color` | Success color |

Using these ensures your theme respects the user's system theme:

```css
.notification-popup {
    background-color: alpha(@window_bg_color, 0.8);
    color: @window_fg_color;
    border-color: @borders;
}

.notification-popup.urgency-critical {
    border-color: @error_color;
}
```

## CSS Custom Properties

Themes should define custom properties for easy overriding. Note that GTK CSS uses `*` (universal selector) instead of `:root` for global variables:

```css
* {
    --histui-font-family: inherit;
    --histui-font-size: 14px;
}

.notification-popup {
    font-family: var(--histui-font-family);
    font-size: var(--histui-font-size);
}
```

Users can override these via CLI flags: `histuid --font "Ubuntu" --font-size 16`

## See Also

- [CSS Reference](/docs/histuid/theming/css-reference) - All CSS selectors and classes
- [Extending Themes](/docs/histuid/theming/extending) - Creating partial themes
- [Layout Reference](/docs/histuid/theming/layout-reference) - Widget layout configuration

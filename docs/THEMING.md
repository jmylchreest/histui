# Theming Guide

This guide explains how to customize the appearance of histui notifications using CSS themes and templates.

## Overview

histui supports two types of customization:

1. **CSS Themes** - Control colors, fonts, spacing, borders, shadows, and visual styling
2. **Output Templates** - Control text formatting for CLI output (dmenu, plain, etc.)

## CSS Themes

### Theme Resolution Order

histui checks for themes in this order:

1. **User themes directory**: `~/.config/histui/themes/`
2. **Bundled themes**: Embedded in the binary

This allows you to override bundled themes by placing a file with the same name in your themes directory.

### Bundled Themes

histui ships with these bundled themes:

| Theme     | Description                                      |
|-----------|--------------------------------------------------|
| `default` | Libadwaita-style with system colors              |
| `minimal` | Clean, distraction-free notifications            |
| `dark`    | High-contrast dark theme (Catppuccin-inspired)   |
| `light`   | Clean white theme with soft shadows              |

### Creating a Custom Theme

Create a CSS file in `~/.config/histui/themes/`:

```bash
mkdir -p ~/.config/histui/themes
touch ~/.config/histui/themes/mytheme.css
```

Then set it in your config:

```toml
# ~/.config/histui/config.toml
[daemon.display]
theme = "mytheme"
```

### Design Tokens

histui uses a design token system with `:root` level CSS variables that provide a consistent color palette. These tokens are defined in `_tokens.css` and are imported by all bundled themes.

**Token Categories:**

| Category      | Variables                                           |
|---------------|-----------------------------------------------------|
| Text colors   | `--text`, `--text-dim`, `--text-muted`, `--text-inverse` |
| Background    | `--bg`, `--bg-muted`, `--bg-accent`, `--bg-hover`   |
| Borders       | `--border`, `--border-muted`                        |
| Accent        | `--accent`, `--accent-hover`, `--accent-fg`         |
| Status        | `--error`, `--error-bg`, `--success`, `--success-bg`, `--warning`, `--warning-bg` |
| Shadows       | `--shadow`, `--shadow-lg`                           |

**Default Token Values:**

```css
/* Light mode (applied to :root) */
:root {
    --text: #1a1a2e;
    --text-dim: rgba(26, 26, 46, 0.7);
    --text-muted: rgba(26, 26, 46, 0.5);
    --text-inverse: #ffffff;

    --bg: #ffffff;
    --bg-muted: #f5f5f7;
    --bg-accent: #e8e8ed;
    --bg-hover: rgba(0, 0, 0, 0.05);

    --border: rgba(0, 0, 0, 0.12);
    --border-muted: rgba(0, 0, 0, 0.06);

    --accent: #3b82f6;
    --accent-hover: #2563eb;
    --accent-fg: #ffffff;

    --error: #dc2626;
    --error-bg: rgba(220, 38, 38, 0.1);
    --success: #22c55e;
    --success-bg: rgba(34, 197, 94, 0.1);
    --warning: #f59e0b;
    --warning-bg: rgba(245, 158, 11, 0.1);

    --shadow: rgba(0, 0, 0, 0.12);
    --shadow-lg: rgba(0, 0, 0, 0.18);
}

/* Dark mode (applied when .dark class is present) */
.dark {
    --text: #cdd6f4;
    --text-dim: rgba(205, 214, 244, 0.7);
    --text-muted: rgba(205, 214, 244, 0.5);
    --text-inverse: #1e1e2e;

    --bg: #1e1e2e;
    --bg-muted: #313244;
    --bg-accent: #45475a;
    --bg-hover: rgba(255, 255, 255, 0.05);

    --border: #45475a;
    --border-muted: #313244;

    --accent: #89b4fa;
    --accent-hover: #b4befe;
    --accent-fg: #1e1e2e;

    --error: #f38ba8;
    --error-bg: rgba(243, 139, 168, 0.15);
    --success: #a6e3a1;
    --success-bg: rgba(166, 227, 161, 0.15);
    --warning: #f9e2af;
    --warning-bg: rgba(249, 226, 175, 0.15);

    --shadow: rgba(0, 0, 0, 0.3);
    --shadow-lg: rgba(0, 0, 0, 0.45);
}
```

**Custom Color Scheme:**

To create a custom color scheme, override the tokens:

```css
/* ~/.config/histui/themes/mytheme.css */
@import "_tokens.css";  /* Import base tokens */

/* Override specific tokens for your palette */
:root {
    --accent: #8b5cf6;        /* Purple accent */
    --accent-hover: #7c3aed;
}

.dark {
    --bg: #0f0f1a;            /* Darker background */
    --accent: #a78bfa;        /* Lighter purple for dark mode */
}

/* Rest of your theme styles... */
.notification-popup {
    background-color: var(--bg);
    color: var(--text);
}
```

### CSS @import Support

histui supports CSS `@import` statements for modular theme organization. Imports are processed and inlined at load time.

**Supported Formats:**

```css
@import "filename.css";
@import 'filename.css';
@import url("filename.css");
@import url('filename.css');
```

**Import Resolution:**

1. **Relative paths** - Resolved relative to the importing file
2. **Embedded partials** - Files starting with `_` are checked in bundled themes
3. **Embedded themes** - Falls back to bundled theme files

**Creating Custom Partials:**

Organize your theme with partials (files starting with `_`):

```
~/.config/histui/themes/
├── mytheme.css           # Main theme file
├── _colors.css           # Your custom color tokens
└── _components.css       # Component-specific styles
```

```css
/* mytheme.css */
@import "_colors.css";
@import "_components.css";

.notification-popup {
    background-color: var(--my-bg);
}
```

```css
/* _colors.css */
:root {
    --my-bg: #1a1a2e;
    --my-fg: #ffffff;
    --my-accent: #ff6b6b;
}

.dark {
    --my-bg: #0d0d1a;
    --my-fg: #e0e0e0;
}
```

**Circular Import Protection:**

histui automatically prevents circular imports. If a file attempts to import itself or create a circular dependency, it's replaced with a comment:

```css
/* circular import prevented: _colors.css */
```

### Light/Dark Mode

histui supports automatic light/dark mode switching based on system preference, similar to Tailwind CSS.

**Configuration:**

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

**How It Works:**

1. The `.light` or `.dark` class is applied to `.notification-popup` based on the color scheme
2. Design tokens in `_tokens.css` define colors for both `:root` (light) and `.dark` selectors
3. Elements use `var(--token-name)` to automatically pick the right color

```css
/* Tokens defined at :root apply to light mode */
:root {
    --bg: #ffffff;
    --text: #1a1a2e;
}

/* .dark overrides tokens for dark mode */
.dark {
    --bg: #1e1e2e;
    --text: #cdd6f4;
}

/* Elements just reference tokens - mode switching is automatic */
.notification-popup {
    background-color: var(--bg);
    color: var(--text);
}
```

### CSS Variable Inheritance

The token system follows CSS cascade rules. Variables defined at `:root` are inherited by all elements. The `.dark` class overrides these when applied.

**Inheritance Chain:**

```
:root                     <- Base tokens (light mode)
  └── .dark               <- Dark mode overrides
        └── .notification-popup
              └── .notification-header
                    └── .notification-summary (inherits --text)
```

**Quick theming example:**

```css
/* Override just the accent color for both modes */
:root { --accent: #8b5cf6; }
.dark { --accent: #a78bfa; }
```

**Complete Token Reference:**

See the [Design Tokens](#design-tokens) section for all available tokens and their default values.

### Class Hierarchy Tree

```
.notification-popup                     <- Root container
│   ├── .light                         <- Light mode (defines light --notif-* vars)
│   ├── .dark                          <- Dark mode (defines dark --notif-* vars)
│   ├── .urgency-low                   <- Urgency modifier
│   ├── .urgency-normal                <- Urgency modifier
│   ├── .urgency-critical              <- Urgency modifier (uses --notif-error)
│   ├── .translucent                   <- State (overrides --notif-bg opacity)
│   ├── .app-{name}                    <- Per-app styling hook
│   └── .category-{name}               <- Per-category styling hook
│
├── .notification-header               <- Horizontal container
│   ├── .notification-icon             <- App icon (48x48)
│   ├── .notification-summary          <- Title (uses --notif-fg)
│   ├── .notification-appname          <- App name (uses --notif-fg-dim)
│   ├── .notification-timestamp        <- Time (uses --notif-fg-muted)
│   ├── .notification-stack-count      <- Badge (uses --notif-accent)
│   └── .notification-close            <- Close button
│
├── .notification-body                 <- Body text (uses --notif-fg)
│   └── link                           <- Hyperlinks (uses --notif-accent)
│
├── .notification-progress             <- Progress bar
│   └── trough > progress              <- Fill (uses --notif-accent)
│
├── .notification-image                <- Embedded image
│
└── .notification-actions              <- Action buttons container
    └── .notification-action           <- Individual button
```

### State Classes Applied to Root

These classes are added to `.notification-popup` based on notification content:

```
.notification-popup
    ├── .has-body         <- Has body text
    ├── .has-icon         <- Has app icon
    ├── .has-actions      <- Has action buttons
    ├── .has-progress     <- Has progress bar
    │   ├── .progress-minimal   <- 0-24%
    │   ├── .progress-low       <- 25-49%
    │   ├── .progress-medium    <- 50-74%
    │   ├── .progress-high      <- 75-99%
    │   └── .progress-complete  <- 100%
    ├── .is-resident      <- Won't auto-close after action
    └── .is-transient     <- Won't persist to history
```

### CSS Class Reference

The notification popup uses these CSS classes:

#### Container Classes

| Class                    | Element                        |
|--------------------------|--------------------------------|
| `.notification-popup`    | Main popup container           |
| `.notification-header`   | Header row (icon + text)       |
| `.notification-content`  | Content area                   |
| `.notification-actions`  | Action buttons container       |

#### Text Classes

| Class                    | Element                        |
|--------------------------|--------------------------------|
| `.notification-summary`  | Title/summary text             |
| `.notification-body`     | Body text                      |
| `.notification-appname`  | Application name label         |
| `.notification-timestamp`| Time label                     |

#### Widget Classes

| Class                      | Element                      |
|----------------------------|------------------------------|
| `.notification-icon`       | Application icon             |
| `.notification-close`      | Close button (X)             |
| `.notification-action`     | Individual action button     |
| `.notification-progress`   | Progress bar                 |
| `.notification-image`      | Embedded image               |
| `.notification-stack-count`| Stacked notification badge   |

#### Urgency Classes

These classes are applied to `.notification-popup`:

| Class              | When Applied                   |
|--------------------|--------------------------------|
| `.urgency-low`     | Low urgency notifications      |
| `.urgency-normal`  | Normal urgency (default)       |
| `.urgency-critical`| Critical/high urgency          |

#### Per-App Classes

Dynamic classes based on application name (sanitized to valid CSS):

| Class              | Example Apps                   |
|--------------------|--------------------------------|
| `.app-discord`     | Discord                        |
| `.app-firefox`     | Firefox                        |
| `.app-slack`       | Slack                          |
| `.app-spotify`     | Spotify                        |
| `.app-vs-code`     | VS Code (spaces become hyphens)|

#### Category Classes

Dynamic classes based on notification category from the [freedesktop.org Desktop Notifications Specification](https://specifications.freedesktop.org/notification-spec/notification-spec-latest.html#categories):

| Class                         | Category                              |
|-------------------------------|---------------------------------------|
| `.category-device`            | Device events                         |
| `.category-device-added`      | Device was added                      |
| `.category-device-removed`    | Device was removed                    |
| `.category-device-error`      | Device error                          |
| `.category-email`             | Email notification                    |
| `.category-email-arrived`     | New email arrived                     |
| `.category-email-bounced`     | Email bounced                         |
| `.category-im`                | Instant message                       |
| `.category-im-received`       | IM received                           |
| `.category-im-error`          | IM error                              |
| `.category-network`           | Network event                         |
| `.category-network-connected` | Network connected                     |
| `.category-network-disconnected` | Network disconnected               |
| `.category-network-error`     | Network error                         |
| `.category-presence`          | Presence change (online/offline)      |
| `.category-presence-online`   | Contact online                        |
| `.category-presence-offline`  | Contact offline                       |
| `.category-transfer`          | File transfer                         |
| `.category-transfer-complete` | Transfer complete                     |
| `.category-transfer-error`    | Transfer error                        |

**Note:** Categories are set by the sending application via the `category` hint. Not all apps set categories. See the [freedesktop.org spec](https://specifications.freedesktop.org/notification-spec/notification-spec-latest.html#hints) for all available hints.

#### State Classes

| Class              | When Applied                   |
|--------------------|--------------------------------|
| `.has-body`        | Notification has body text     |
| `.has-icon`        | Notification has an icon       |
| `.has-actions`     | Notification has action buttons|
| `.has-progress`    | Notification has progress bar  |
| `.is-resident`     | Resident notification          |
| `.is-transient`    | Transient notification         |

#### Progress Classes

When `.has-progress` is applied, one of these is also added:

| Class               | When Applied                  |
|---------------------|-------------------------------|
| `.progress-minimal` | Progress 0-24%                |
| `.progress-low`     | Progress 25-49%               |
| `.progress-medium`  | Progress 50-74%               |
| `.progress-high`    | Progress 75-99%               |
| `.progress-complete`| Progress 100%                 |

### Example Theme

```css
/* Custom dark theme with accent colors */

.notification-popup {
    background-color: #1a1b26;
    border-radius: 16px;
    border: 2px solid #3b4261;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.6);
    padding: 16px;
    margin: 10px;
}

.notification-summary {
    font-weight: bold;
    font-size: 1.15em;
    color: #c0caf5;
}

.notification-body {
    color: #a9b1d6;
    font-size: 0.95em;
    margin-top: 8px;
}

.notification-appname {
    color: #565f89;
    font-size: 0.8em;
}

/* Critical notifications get red border */
.notification-popup.urgency-critical {
    border-color: #f7768e;
    box-shadow: 0 0 20px rgba(247, 118, 142, 0.3);
}

.notification-popup.urgency-critical .notification-summary {
    color: #f7768e;
}

/* Action buttons */
.notification-action {
    background-color: #3b4261;
    color: #c0caf5;
    border-radius: 8px;
    padding: 8px 16px;
}

.notification-action:hover {
    background-color: #7aa2f7;
    color: #1a1b26;
}

/* Stack count badge */
.notification-stack-count {
    background-color: #7aa2f7;
    color: #1a1b26;
    border-radius: 12px;
    padding: 3px 10px;
    font-weight: bold;
}
```

### Per-App Styling Example

Use per-app classes to customize notifications from specific applications:

```css
/* Discord - purple accent */
.notification-popup.app-discord {
    border-left: 4px solid #5865F2;
}

.notification-popup.app-discord .notification-icon {
    filter: none; /* Keep Discord's icon colors */
}

/* Slack - green accent */
.notification-popup.app-slack {
    border-left: 4px solid #4A154B;
}

/* Spotify - green accent */
.notification-popup.app-spotify {
    border-left: 4px solid #1DB954;
}

/* Email notifications - minimal style */
.notification-popup.category-email {
    opacity: 0.9;
}

.notification-popup.category-email .notification-icon {
    -gtk-icon-size: 32px;
}

/* Hide body for transient notifications */
.notification-popup.is-transient .notification-body {
    display: none;
}

/* Style progress notifications */
.notification-popup.has-progress.progress-complete {
    border-color: #4ade80;
}
```

### GTK4/Libadwaita CSS Variables

The default theme uses libadwaita CSS variables for system integration:

| Variable              | Purpose                    |
|-----------------------|----------------------------|
| `@window_bg_color`    | Background color           |
| `@window_fg_color`    | Foreground/text color      |
| `@borders`            | Border color               |
| `@accent_bg_color`    | Accent background          |
| `@accent_fg_color`    | Accent foreground          |
| `@accent_color`       | Primary accent             |
| `@error_color`        | Error/critical color       |

Using these variables ensures your theme respects the user's system theme (light/dark).

### Hot Reload

Theme changes are automatically reloaded when the CSS file is modified. You can edit your theme in real-time without restarting the daemon.

## Layout Templates

Layout templates control the structure and arrangement of elements within notification popups using an HTML-like XML format.

### Template Resolution Order

histui checks for layout templates in this order:

1. **User templates directory**: `~/.config/histui/layouts/`
2. **Bundled templates**: Embedded in the binary

### Bundled Layouts

histui ships with these bundled layouts:

| Layout     | Description                                      |
|------------|--------------------------------------------------|
| `default`  | Full layout with icon, summary, appname, body, actions |
| `compact`  | Simplified layout without appname                |
| `minimal`  | Just summary and body                            |
| `detailed` | Full layout with timestamp                       |

### Configuring Layout

Set the layout template in your daemon config:

```toml
# ~/.config/histui/histuid.toml
[layout]
template = "compact"
```

### Creating a Custom Layout

Create an XML file in `~/.config/histui/layouts/`:

```bash
mkdir -p ~/.config/histui/layouts
touch ~/.config/histui/layouts/custom.xml
```

### Layout Syntax

Layouts use a simple XML format with these elements:

| Element        | Description                           | Container |
|----------------|---------------------------------------|-----------|
| `<popup>`      | Root element (required)               | Yes       |
| `<header>`     | Horizontal header row                 | Yes       |
| `<box>`        | Generic container (vertical/horizontal)| Yes      |
| `<icon>`       | Application icon                      | No        |
| `<summary>`    | Notification title                    | No        |
| `<appname>`    | Application name                      | No        |
| `<timestamp>`  | Relative time (e.g., "5m ago")        | No        |
| `<body>`       | Notification body text                | No        |
| `<progress>`   | Progress bar (if notification has progress) | No  |
| `<image>`      | Embedded image (if notification has image) | No   |
| `<actions>`    | Action buttons container              | No        |
| `<stack-count>`| Badge showing stacked notification count | No     |
| `<close>`      | Close button                          | No        |

### Box Attributes

The `<box>` element supports these attributes:

| Attribute     | Values                    | Default    |
|---------------|---------------------------|------------|
| `orientation` | `vertical`, `horizontal`  | `vertical` |

### Example Layouts

**Default layout:**
```xml
<popup>
  <header>
    <icon />
    <box orientation="vertical">
      <summary />
      <appname />
    </box>
    <stack-count />
    <close />
  </header>
  <body />
  <progress />
  <image />
  <actions />
</popup>
```

**Compact layout (no app name):**
```xml
<popup>
  <header>
    <icon />
    <summary />
    <stack-count />
    <close />
  </header>
  <body />
  <actions />
</popup>
```

**Detailed layout with timestamp:**
```xml
<popup>
  <header>
    <icon />
    <box orientation="vertical">
      <summary />
      <box orientation="horizontal">
        <appname />
        <timestamp />
      </box>
    </box>
    <stack-count />
    <close />
  </header>
  <body />
  <progress />
  <image />
  <actions />
</popup>
```

**Minimal layout:**
```xml
<popup>
  <summary />
  <body />
</popup>
```

### Conditional Elements

Some elements only appear when the notification has the relevant data:

| Element     | Condition                                    |
|-------------|----------------------------------------------|
| `<body>`    | Only if notification has body text           |
| `<progress>`| Only if notification has progress hint       |
| `<image>`   | Only if notification has image-path hint     |
| `<actions>` | Only if notification has actions             |

## Output Templates

The CLI commands (`histui get`) support custom output templates using Go template syntax.

### Template Syntax

Templates use Go's `text/template` syntax with double curly braces:

```
{{.Field}}              - Access a field
{{.Notification.Field}} - Access notification field
{{.Index}}              - Current item index (1-based)
{{.RelativeTime}}       - Human-readable time (e.g., "5m")
```

### Available Fields

#### Top-Level Fields

| Field            | Type   | Description                    |
|------------------|--------|--------------------------------|
| `.Index`         | int    | 1-based index in the list      |
| `.RelativeTime`  | string | Human-readable relative time   |
| `.Notification`  | object | The notification object        |

#### Notification Fields

| Field                        | Type   | Description                    |
|------------------------------|--------|--------------------------------|
| `.Notification.HistuiID`     | string | Unique notification ID (ULID)  |
| `.Notification.Summary`      | string | Title/summary text             |
| `.Notification.Body`         | string | Body text                      |
| `.Notification.AppName`      | string | Application name               |
| `.Notification.AppIcon`      | string | Icon name                      |
| `.Notification.Timestamp`    | int64  | Unix timestamp                 |
| `.Notification.Urgency`      | int    | Urgency level (0/1/2)          |
| `.Notification.UrgencyName`  | string | Urgency name (low/normal/critical) |
| `.Notification.Category`     | string | Notification category          |
| `.Notification.HistuiDismissedAt` | *time | When dismissed (nil if not) |
| `.Notification.HistuiSeenAt` | *time  | When marked seen (nil if not)  |

### Template Functions

| Function                    | Description                          |
|-----------------------------|--------------------------------------|
| `truncate STRING MAXLEN`    | Truncate string with ellipsis        |
| `reltime TIMESTAMP`         | Convert timestamp to relative time   |
| `urgencyIcon URGENCY`       | Convert urgency to icon (L/-/!)      |

### Example Templates

**Compact format:**
```
{{.Index}}|{{.Notification.AppName}}|{{.Notification.Summary}}
```

**With urgency icon:**
```
{{urgencyIcon .Notification.Urgency}} {{.Notification.Summary}} ({{.Notification.AppName}})
```

**JSON-like with truncation:**
```
[{{.RelativeTime}}] {{.Notification.AppName}}: {{truncate .Notification.Summary 40}}
```

### Using Templates

Set a custom template in your config:

```toml
# ~/.config/histui/config.toml
[templates]
dmenu = "{{.Index}} | {{.RelativeTime}} | {{.Notification.AppName}} | {{.Notification.Summary}}"
```

Or use the `--template` flag:

```bash
histui get --template "{{.Notification.Summary}}: {{truncate .Notification.Body 50}}"
```

## Filter Expressions

The CLI supports expression-based filtering using a simple syntax.

### Filter Syntax

```
field=value          Equal to
field!=value         Not equal to
field~value          Contains (case-insensitive)
field~=regex         Regex match
field>value          Greater than
field<value          Less than
field>=value         Greater or equal
field<=value         Less or equal
```

### Available Filter Fields

| Field       | Type    | Description                       |
|-------------|---------|-----------------------------------|
| `app`       | string  | Application name                  |
| `summary`   | string  | Notification title                |
| `body`      | string  | Notification body text            |
| `urgency`   | string  | Urgency level (low/normal/critical) |
| `category`  | string  | Notification category             |
| `dismissed` | bool    | Whether dismissed (true/false)    |
| `seen`      | bool    | Whether marked seen (true/false)  |
| `timestamp` | duration| Time since notification (e.g., 1h, 7d) |

### Filter Examples

```bash
# Filter by app name
histui get --filter "app=discord"

# Multiple conditions (AND logic)
histui get --filter "app=slack,urgency=critical"

# Contains search
histui get --filter "body~meeting"

# Regex matching
histui get --filter "summary~=(?i)error|warning"

# Time-based filtering
histui get --filter "timestamp<1h"        # Last hour
histui get --filter "timestamp>7d"        # Older than 7 days

# State filtering
histui get --filter "dismissed=false"
histui get --filter "seen=true"
```

## Compositor Blur and Transparency

histui supports transparent notification backgrounds that work with compositor blur effects. This requires both histui configuration and compositor configuration.

### Configuring Opacity

Set the opacity in your daemon config:

```toml
# ~/.config/histui/histuid.toml
[display]
opacity = 0.85  # 0.0 (fully transparent) to 1.0 (fully opaque)
```

When opacity is less than 1.0, histui adds the `.translucent` CSS class to notification popups, allowing you to style transparent notifications differently.

### CSS for Transparent Backgrounds

Use RGBA colors or CSS opacity for transparent effects:

```css
/* Style for translucent notifications */
.notification-popup.translucent {
    background-color: rgba(30, 30, 46, 0.85);
    backdrop-filter: blur(10px);  /* Note: may not work in all GTK configurations */
}

/* Alternative: use alpha channel directly */
.notification-popup {
    background-color: alpha(@window_bg_color, 0.9);
}
```

### Hyprland Configuration

For blur effects with Hyprland, add window rules:

```conf
# ~/.config/hypr/hyprland.conf

# Enable blur for histui notifications
windowrulev2 = opacity 0.9 0.9, class:^(histui)$
windowrulev2 = blur, class:^(histui)$

# Configure blur settings
decoration {
    blur {
        enabled = true
        size = 8
        passes = 2
        new_optimizations = true
    }
}
```

### Sway Configuration

For sway, transparency is handled at the application level:

```conf
# ~/.config/sway/config

# No specific blur support in sway, but opacity works
for_window [app_id="histui"] opacity 0.9
```

### Notes

- Blur is a compositor feature, not a GTK feature. The `backdrop-filter` CSS property may not work.
- The GTK layer-shell popup inherits compositor effects when opacity < 1.0.
- Test with your specific compositor to ensure desired appearance.

## What's Currently Possible

### Supported

- Full CSS styling of all popup elements
- Custom colors, fonts, sizes, borders, shadows
- Urgency-based styling variations
- Per-app styling (`.app-discord`, `.app-slack`, etc.)
- Category-based styling (`.category-email`, `.category-im`, etc.)
- State-based styling (`.has-body`, `.has-actions`, `.is-transient`, etc.)
- Progress bar styling (`.has-progress`, `.progress-complete`, etc.)
- Transparent backgrounds with compositor blur
- XML layout templates for custom popup structure
- Hot-reload of theme changes
- Custom output templates for CLI
- Expression-based filtering
- Libadwaita CSS variable integration
- Multiple bundled themes and layouts to use as starting points
- Design token system with `:root` level CSS variables
- CSS `@import` support for modular theme organization
- Automatic light/dark mode switching based on system preference

### Current Limitations

These features are planned but not yet implemented:

- **SVG masks/shaped windows** - Non-rectangular popups
- **Animation control** - Custom enter/exit animations
- **Image backgrounds** - Background images in CSS

## Tips

1. **Start from a bundled theme** - Copy and modify rather than starting from scratch
2. **Use system colors** - Libadwaita variables adapt to light/dark mode
3. **Test with hot-reload** - Save your CSS file to see changes instantly
4. **Check urgency states** - Ensure critical notifications are visible
5. **Consider accessibility** - Maintain readable contrast ratios

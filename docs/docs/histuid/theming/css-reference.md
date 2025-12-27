---
title: CSS Reference
description: Complete CSS selector and property reference
sidebar_position: 2
---

# CSS Reference

Complete reference for histuid CSS selectors and GTK4 CSS properties.

## Class Hierarchy

```
.notification-popup                     <- Root container
│   ├── .light                         <- Light mode
│   ├── .dark                          <- Dark mode
│   ├── .urgency-low                   <- Low urgency
│   ├── .urgency-normal                <- Normal urgency
│   ├── .urgency-critical              <- Critical urgency
│   ├── .translucent                   <- Transparent background
│   ├── .app-{name}                    <- Per-app styling hook
│   └── .category-{name}               <- Per-category styling hook
│
├── .notification-header               <- Horizontal container
│   ├── .notification-icon             <- App icon (48x48)
│   ├── .notification-summary          <- Title
│   ├── .notification-appname          <- App name
│   ├── .notification-timestamp        <- Time
│   ├── .notification-stack-count      <- Badge
│   └── .notification-close            <- Close button
│
├── .notification-body                 <- Body text
│   └── link                           <- Hyperlinks
│
├── .notification-progress             <- Progress bar
│   └── trough > progress              <- Fill
│
├── .notification-image                <- Embedded image
│
└── .notification-actions              <- Action buttons container
    └── .notification-action           <- Individual button
```

## Container Classes

| Class                    | Element                        |
|--------------------------|--------------------------------|
| `.notification-popup`    | Main popup container           |
| `.notification-header`   | Header row (icon + text)       |
| `.notification-content`  | Content area                   |
| `.notification-actions`  | Action buttons container       |

## Text Classes

| Class                    | Element                        |
|--------------------------|--------------------------------|
| `.notification-summary`  | Title/summary text             |
| `.notification-body`     | Body text                      |
| `.notification-appname`  | Application name label         |
| `.notification-timestamp`| Time label                     |

## Widget Classes

| Class                      | Element                      |
|----------------------------|------------------------------|
| `.notification-icon`       | Application icon             |
| `.notification-close`      | Close button (X)             |
| `.notification-action`     | Individual action button     |
| `.notification-progress`   | Progress bar                 |
| `.notification-image`      | Embedded image               |
| `.notification-stack-count`| Stacked notification badge   |

## Urgency Classes

Applied to `.notification-popup`:

| Class              | When Applied                   |
|--------------------|--------------------------------|
| `.urgency-low`     | Low urgency notifications      |
| `.urgency-normal`  | Normal urgency (default)       |
| `.urgency-critical`| Critical/high urgency          |

## Per-App Classes

Dynamic classes based on application name (sanitized to valid CSS):

| Class              | Example Apps                   |
|--------------------|--------------------------------|
| `.app-discord`     | Discord                        |
| `.app-firefox`     | Firefox                        |
| `.app-slack`       | Slack                          |
| `.app-spotify`     | Spotify                        |
| `.app-vs-code`     | VS Code (spaces become hyphens)|

Example usage:

```css
/* Discord - purple accent */
.notification-popup.app-discord {
    border-left: 4px solid #5865F2;
}

/* Slack - brand color */
.notification-popup.app-slack {
    border-left: 4px solid #4A154B;
}
```

## Category Classes

Based on the [Desktop Notifications Specification](https://specifications.freedesktop.org/notification-spec/):

| Class                         | Category                              |
|-------------------------------|---------------------------------------|
| `.category-device`            | Device events                         |
| `.category-device-added`      | Device was added                      |
| `.category-device-removed`    | Device was removed                    |
| `.category-email`             | Email notification                    |
| `.category-email-arrived`     | New email arrived                     |
| `.category-im`                | Instant message                       |
| `.category-im-received`       | IM received                           |
| `.category-network`           | Network event                         |
| `.category-network-connected` | Network connected                     |
| `.category-transfer`          | File transfer                         |
| `.category-transfer-complete` | Transfer complete                     |

## State Classes

Applied to `.notification-popup`:

| Class              | When Applied                   |
|--------------------|--------------------------------|
| `.has-body`        | Notification has body text     |
| `.has-icon`        | Notification has an icon       |
| `.has-actions`     | Notification has action buttons|
| `.has-progress`    | Notification has progress bar  |
| `.is-resident`     | Resident notification          |
| `.is-transient`    | Transient notification         |

## Progress Classes

When `.has-progress` is applied, one of these is also added:

| Class               | When Applied                  |
|---------------------|-------------------------------|
| `.progress-minimal` | Progress 0-24%                |
| `.progress-low`     | Progress 25-49%               |
| `.progress-medium`  | Progress 50-74%               |
| `.progress-high`    | Progress 75-99%               |
| `.progress-complete`| Progress 100%                 |

## Stack Position Classes

When multiple notifications are displayed, these classes control how they visually connect:

| Class               | When Applied                  |
|---------------------|-------------------------------|
| `.stack-single`     | Only one notification visible |
| `.stack-first`      | First in stack (top)          |
| `.stack-middle`     | Middle notification           |
| `.stack-last`       | Last in stack (bottom)        |

The default theme uses these to create a unified stack appearance:
- **stack-first**: Rounded top corners, flat bottom
- **stack-middle**: All corners flat, thin separator line
- **stack-last**: Flat top, rounded bottom corners

### Compositor Integration

Some compositors apply their own corner rounding to layer-shell surfaces, which can interfere with stack styling.

**Hyprland:**
Hyprland applies `decoration:rounding` globally to layer surfaces. There is no per-surface layer rule to disable this. Options:
- Set `decoration { rounding = 0 }` globally (affects all windows)
- Use `display.gap` in histuid config to space notifications so rounding looks intentional

**Sway:**
Respects the layer-shell surface styling. No additional configuration needed.

**Other Compositors:**
Consult your compositor's documentation for controlling rounding on layer-shell surfaces with namespace `histui-notification`.

## GTK4/Libadwaita CSS Variables

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

Using these variables ensures your theme respects the user's system theme.

## Common CSS Properties

### Colors

```css
.notification-popup {
  background-color: #1e1e2e;
  color: #cdd6f4;
  border-color: #45475a;
}
```

### Borders

```css
.notification-popup {
  border: 1px solid #45475a;
  border-radius: 12px;
}
```

### Spacing

```css
.notification-popup {
  padding: 12px;
  margin: 4px;
}

.notification-content {
  margin-left: 12px;
}
```

### Typography

```css
.notification-summary {
  font-weight: bold;
  font-size: 14px;
}

.notification-body {
  font-size: 12px;
  font-family: sans-serif;
}
```

### Shadows

```css
.notification-popup {
  box-shadow: 0 4px 6px rgba(0, 0, 0, 0.3);
}
```

## CSS Animations

GTK4 supports CSS animations via `@keyframes`. Example pulsing effect:

```css
@keyframes pulse {
    0%, 100% { text-shadow: 0 0 4px @error_color; }
    50% { text-shadow: 0 0 12px @error_color; }
}

.notification-popup.urgency-critical .notification-summary {
    animation: pulse 2s ease-in-out infinite;
}
```

See [Advanced Theming](/docs/histuid/theming/advanced) for more animation examples.

## Font Variables

Themes can use CSS custom properties for fonts:

```css
:root {
    --histui-font-family: "Inter", sans-serif;
    --histui-font-size: 14px;
}

.notification-popup {
    font-family: var(--histui-font-family);
    font-size: var(--histui-font-size);
}
```

These can be overridden via CLI: `histuid --font "Ubuntu" --font-size 16`

## See Also

- [Advanced Theming](/docs/histuid/theming/advanced) - Audio, fonts, animations
- [GTK4 CSS Properties](https://docs.gtk.org/gtk4/css-properties.html) - Full GTK4 reference
- [Theme Examples](/docs/histuid/theming/examples) - Complete themes

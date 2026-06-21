---
title: Theme Examples
description: Ready-to-use theme configurations
sidebar_position: 3
---

# Theme Examples

Copy these themes to `~/.config/histui/theme.css`.

## Catppuccin Mocha

A soothing pastel theme.

```css
/* Catppuccin Mocha */
.notification {
  background-color: #1e1e2e;
  color: #cdd6f4;
  border: 1px solid #45475a;
  border-radius: 12px;
  padding: 12px;
  box-shadow: 0 4px 6px rgba(0, 0, 0, 0.3);
}

.notification-summary {
  color: #cdd6f4;
  font-weight: bold;
  font-size: 14px;
}

.notification-body {
  color: #bac2de;
  font-size: 12px;
}

.notification.critical {
  border-left: 4px solid #f38ba8;
}

.notification.low {
  opacity: 0.8;
}

.notification-actions button {
  background-color: #45475a;
  color: #cdd6f4;
  border-radius: 6px;
  padding: 4px 12px;
}

.notification-actions button:hover {
  background-color: #585b70;
}
```

## Nord

Arctic, north-bluish color palette.

```css
/* Nord */
.notification {
  background-color: #2e3440;
  color: #eceff4;
  border: 1px solid #4c566a;
  border-radius: 8px;
  padding: 12px;
}

.notification-summary {
  color: #eceff4;
  font-weight: bold;
}

.notification-body {
  color: #d8dee9;
}

.notification.critical {
  border-left: 4px solid #bf616a;
}

.notification-actions button {
  background-color: #4c566a;
  color: #eceff4;
}
```

## Dracula

Dark theme with vibrant accents.

```css
/* Dracula */
.notification {
  background-color: #282a36;
  color: #f8f8f2;
  border: 1px solid #44475a;
  border-radius: 10px;
  padding: 12px;
}

.notification-summary {
  color: #f8f8f2;
  font-weight: bold;
}

.notification-body {
  color: #6272a4;
}

.notification.critical {
  border-left: 4px solid #ff5555;
}

.notification.low {
  border-left: 4px solid #6272a4;
}

.notification-actions button {
  background-color: #44475a;
  color: #f8f8f2;
}

.notification-actions button:hover {
  background-color: #bd93f9;
  color: #282a36;
}
```

## Minimal Light

Clean, minimal light theme.

```css
/* Minimal Light */
.notification {
  background-color: #ffffff;
  color: #1f2937;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  padding: 14px;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.notification-summary {
  color: #111827;
  font-weight: 600;
  font-size: 14px;
}

.notification-body {
  color: #4b5563;
  font-size: 13px;
}

.notification.critical {
  border-left: 3px solid #ef4444;
}

.notification-actions button {
  background-color: #f3f4f6;
  color: #374151;
  border: 1px solid #d1d5db;
}
```

## See Also

- [CSS Reference](/docs/histuid/theming/css-reference) - All available selectors
- [GTK4 CSS Properties](https://docs.gtk.org/gtk4/css-properties.html) - Full property reference

---
title: Introduction
description: A highly configurable notification daemon and history browser for Wayland
sidebar_position: 1
slug: /
---

# histui

**histui** (pronounced *his-too-ee*) is a notification system for Linux Wayland desktops.
It captures, displays, and lets you browse your notification history.

:::tip How to pronounce it
The name is a play on "history" + "TUI":
- **histui** → *his-too-ee*
- **histuid** (the daemon) → *his-too-ee-dee*
:::

## What You Can Do

### Display Notifications

**histuid** is a GTK4-based notification daemon for Wayland compositors. It:

- **CSS theming with hot reload** - edit themes live without restarting
- **Clickable URLs and deep links** - jump straight to the source app
- **Image previews** and progress bars with Pango markup support
- **Smart icon resolution** - app aliases with Nerd Font fallbacks for 350+ apps
- **Audio alerts** by urgency with repeat options
- **Notification stacking** with smooth animations
- Supports Hyprland, Sway, river, and other layer-shell compositors

### Browse Your History

**histui** is a CLI and TUI for querying notification history. You can:

- View what you missed while away
- Filter by app, urgency, or time range
- Output in JSON for scripts or dmenu for launchers
- Show notification counts in Waybar or Polybar

### Run Alongside Other Daemons

Want to keep dunst or mako for display but use histui for history? **Monitor mode** listens to an existing notification daemon and records all notifications to history - without displaying popups or taking over the D-Bus name.

## Quick Start

```bash
# Install (Arch Linux)
yay -S histui-bin

# Start the daemon
histuid

# Query your history
histui get --app discord --since 1h
```

:::note Wayland Required
histuid requires a Wayland compositor with layer-shell support.
The histui CLI works on both X11 and Wayland.
:::

## Next Steps

<div className="row">
  <div className="col col--4">
    <h3>🚀 Get Started</h3>
    <p>Install and run histuid in 5 minutes</p>
    <a href="/docs/quickstart">→ Quickstart Guide</a>
  </div>
  <div className="col col--4">
    <h3>🎨 Customize</h3>
    <p>Match notifications to your theme</p>
    <a href="/docs/histuid/theming">→ Theming Guide</a>
  </div>
  <div className="col col--4">
    <h3>🔍 Browse History</h3>
    <p>Find and filter past notifications</p>
    <a href="/docs/histui/commands/get">→ CLI Commands</a>
  </div>
</div>

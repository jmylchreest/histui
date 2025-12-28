---
title: histuid Daemon Quickstart
description: Install and configure the histuid notification daemon
sidebar_position: 3
---

# histuid Daemon Quickstart

Configure histuid to capture and display desktop notifications.

:::info Wayland Only
histuid requires a Wayland compositor with layer-shell support (Hyprland, Sway, river, etc.).
It does not support X11.
:::

:::tip Installation
See the [Installation Guide](/docs/installation) for all installation methods including AUR, pre-built binaries, and building from source.
:::

## Starting the Daemon

```bash
histuid
```

histuid will start capturing notifications and displaying popups.

## Compositor Setup

### Hyprland

Add these rules to your `hyprland.conf`:

```ini
# Optional: Enable blur for translucent notifications
layerrule = blur, histui-notification
layerrule = ignorealpha 0.3, histui-notification
```

:::note Stack Styling Limitation
Hyprland applies global `decoration:rounding` to layer-shell surfaces and there is no layer rule to disable it per-surface. If you want connected/unified notification stacks without gaps between corners, you can either:
- Set `decoration { rounding = 0 }` globally (affects all windows)
- Use `display.gap` in histuid config to add spacing between notifications so the rounding looks intentional
:::

### Sway

No additional configuration needed. Sway respects the layer-shell surface styling.

## Configuration

histuid works with sensible defaults.
Create a configuration file for customization:

```bash
mkdir -p ~/.config/histui
touch ~/.config/histui/histuid.toml
```

See [Configuration Reference](/docs/histuid/configuration) for all options.

## Next Steps

- [Desktop Integration](/docs/histuid/integration) - Auto-start with Hyprland, Sway, systemd
- [Configuration](/docs/histuid/configuration) - All configuration options
- [Theming](/docs/histuid/theming) - Customize notification appearance
- [Monitor Mode](/docs/histuid/monitor-mode) - Run alongside dunst

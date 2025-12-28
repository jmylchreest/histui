---
title: Desktop Integration
description: Integrate histuid with your desktop environment
sidebar_position: 5
---

# Desktop Integration

Configure histuid to start automatically and integrate with your desktop environment.

:::info Wayland Only
histuid requires Wayland and uses the layer-shell protocol for notification positioning.
It does not support X11.
:::

## Hyprland

### Auto-start

Add to your `~/.config/hypr/hyprland.conf`:

```ini
exec-once = histuid
```

### Waybar Integration

Add a custom module to show notification status in Waybar.

In `~/.config/waybar/config`:

```jsonc
{
    "modules-center": ["clock", "custom/histui"],

    "custom/histui": {
        "exec": "histui status",
        "return-type": "json",
        "interval": 5,
        "format": "{icon} {text}",
        "format-icons": {
            "empty": "",      // No notifications
            "dnd": "󰂛",        // Do Not Disturb
            "low": "󱅫",        // Low priority
            "normal": "󱅫",     // Normal priority
            "critical": "󰂚"    // Critical notification
        },
        "on-click": "histui dnd toggle",
        "on-click-right": "hyprctl dispatch exec '[float;size 900 600;center] kitty --class histui-float -e histui tui'",
        "on-click-middle": "histui get --format dmenu --since 24h | fuzzel --dmenu -p 'Notifications'"
    }
}
```

**Click actions:**
- **Left click**: Toggle Do Not Disturb mode
- **Right click** (2-finger tap): Open the TUI browser
- **Middle click** (3-finger tap): Browse recent notifications in fuzzel/dmenu

In `~/.config/waybar/style.css`:

```css
#custom-histui {
    padding: 0 10px;
}

/* Do Not Disturb active */
#custom-histui.dnd {
    color: #f38ba8;
}

/* Has unread notifications */
#custom-histui.has-notifications {
    color: #a6e3a1;
}

/* Critical notification pending */
#custom-histui.critical {
    color: #f38ba8;
    animation: pulse 1s ease-in-out infinite;
}

@keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.5; }
}
```

The `histui status` command outputs Waybar-compatible JSON:

```json
{
    "text": "3",
    "alt": "normal",
    "tooltip": "3 active\nDisplayed: 2\nWaiting: 1",
    "class": "has-notifications normal"
}
```

The `alt` field is used by Waybar to select the icon from `format-icons`. Possible values:
- `empty` - No notifications
- `dnd` - Do Not Disturb mode enabled
- `low`, `normal`, `critical` - Highest urgency among active notifications

:::tip Using Walker instead of Fuzzel
Replace `fuzzel --dmenu` with `walker --dmenu` if you use Walker as your launcher.
:::

## Sway

### Auto-start

Add to your `~/.config/sway/config`:

```ini
exec histuid
```

### Waybar

Same configuration as Hyprland above.

## systemd

For desktop environments without native auto-start, use the systemd user service.

### Installation

```bash
# Copy the service file
mkdir -p ~/.config/systemd/user
cp /usr/lib/systemd/user/histuid.service ~/.config/systemd/user/

# Or create manually
cat > ~/.config/systemd/user/histuid.service << 'EOF'
[Unit]
Description=histuid notification daemon
PartOf=graphical-session.target
After=graphical-session.target

[Service]
Type=simple
ExecStart=/usr/bin/histuid
Restart=on-failure
RestartSec=5

[Install]
WantedBy=graphical-session.target
EOF
```

### Enable and Start

```bash
# Enable on login
systemctl --user enable histuid

# Start now
systemctl --user start histuid

# Check status
systemctl --user status histuid

# View logs
journalctl --user -u histuid -f
```

## Other Wayland Compositors

For other Wayland compositors (river, dwl, etc.), add histuid to your startup configuration:

```bash
histuid &
```

## Verifying Installation

Test that notifications are working:

```bash
# Send a test notification
notify-send "Test" "histuid is working!"

# Check history
histui get
```

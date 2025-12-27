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

```json
{
    "modules-right": ["custom/notifications"],
    "custom/notifications": {
        "exec": "histui status --format json",
        "return-type": "json",
        "interval": 5,
        "on-click": "histui tui"
    }
}
```

In `~/.config/waybar/style.css`:

```css
#custom-notifications {
    padding: 0 10px;
}

#custom-notifications.dnd {
    color: #f38ba8;
}

#custom-notifications.has-notifications {
    color: #a6e3a1;
}
```

The status command outputs:

```json
{
    "text": "3",
    "tooltip": "3 notifications",
    "class": "has-notifications"
}
```

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

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
        "exec": "histui status --detailed",
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

#### Detailed Tooltip

Use `--detailed` for a comprehensive tooltip with:
- DnD status
- Pending/Missed/Dismissed counts with urgency breakdown
- Top 5 applications by notification count

Example tooltip:
```
DnD: disabled

Pending: 2
  Critical: 0
  Normal: 2
  Low: 0
Missed: 5
  Critical: 1
  Normal: 3
  Low: 1
Dismissed: 42
Tracked: 156

Top 5 Applications:
  1. Discord: 28
  2. Firefox: 22
  3. Slack: 18
  4. Signal: 12
  5. Thunderbird: 8
```

#### Advanced Click Actions

Replay missed notifications with on-click bindings:

```jsonc
{
    "custom/histui": {
        "exec": "histui status --detailed",
        "return-type": "json",
        "interval": 5,
        "format": "{icon} {text}",
        "format-icons": {
            "empty": "",
            "dnd": "󰂛",
            "low": "󱅫",
            "normal": "󱅫",
            "critical": "󰂚"
        },
        // Toggle DnD
        "on-click": "histui dnd toggle",
        // Open TUI
        "on-click-right": "hyprctl dispatch exec '[float;size 900 600;center] kitty --class histui-float -e histui tui'",
        // Replay missed normal notifications (most recent 5)
        "on-click-middle": "histui replay --filter 'seen=false,dismissed=false,urgency=normal' --limit 5 --all",
        // Scroll up: replay missed criticals
        "on-scroll-up": "histui replay --filter 'dismissed=false,urgency=critical' --limit 3 --all",
        // Scroll down: dismiss all
        "on-scroll-down": "histui set --filter 'dismissed=false' --dismiss --all"
    }
}
```

**Workflow bindings explained:**
- **Middle click**: Replay the 5 most recent missed normal-urgency notifications
- **Scroll up**: Replay any undismissed critical notifications (up to 3)
- **Scroll down**: Dismiss all active notifications

#### Filter Expressions

Use `--filter` for powerful notification selection:

| Filter | Description |
|--------|-------------|
| `seen=false` | Not yet seen (missed) |
| `dismissed=false` | Not dismissed |
| `urgency=critical` | Critical urgency only |
| `urgency>=normal` | Normal or critical |
| `app=discord` | From Discord only |
| `age<1h` | From the last hour |

Combine filters with commas (AND logic):
```bash
# Missed critical notifications
histui get --filter "seen=false,dismissed=false,urgency=critical"

# Undismissed from Slack in the last hour
histui get --filter "dismissed=false,app=slack,age<1h"
```

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
    "tooltip": "DnD: disabled\n\nPending: 2\n  Critical: 0\n  ...",
    "class": "has-notifications normal"
}
```

The `alt` field is used by Waybar to select the icon from `format-icons`. Possible values:
- `empty` - No notifications
- `dnd` - Do Not Disturb mode enabled
- `low`, `normal`, `critical` - Highest urgency among active notifications

:::tip Customizing the Text
Waybar's `format` controls what appears next to the icon. Use `{text}` for the count, or create custom formats:
```jsonc
"format": "{icon}",           // Icon only
"format": "{icon} {text}",    // Icon and count
"format": "󰂚 {text}"          // Custom icon with count
```
:::

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

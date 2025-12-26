# Waybar Integration

Example configurations for integrating histui with Waybar.

## Module Configuration

Add to your `~/.config/waybar/config.jsonc`:

```jsonc
{
  "modules-right": [
    "custom/notifications",
    // ... other modules
  ],

  "custom/notifications": {
    "exec": "histui status --all --since 24h",
    "interval": 5,
    "return-type": "json",
    "format": "{icon} {}",
    "format-icons": {
      "low": "󰎟",
      "normal": "󰍡",
      "critical": "󱈸",
      "empty": "󰍥"
    },
    // Left click: toggle dunst pause
    "on-click": "dunstctl set-paused toggle",
    // Middle click: open floating TUI
    "on-click-middle": "hyprctl dispatch exec '[float;size 900 600;center] kitty --class histui-float -e histui'",
    // Right click: dmenu picker (copies body to clipboard)
    "on-click-right": "histui get | walker --dmenu -p 'Notifications' | cut -d'|' -f1 | xargs histui get --field body | wl-copy"
  }
}
```

## Styling

Add to your `~/.config/waybar/style.css`:

```css
#custom-notifications {
  padding: 0 8px;
}

#custom-notifications.critical {
  color: #f38ba8;
}

#custom-notifications.normal {
  color: #a6e3a1;
}

#custom-notifications.low {
  color: #6c7086;
}
```

## Alternative Launchers

### Fuzzel

```jsonc
"on-click-right": "histui get | fuzzel -d -p 'Notifications: ' | cut -d'|' -f1 | xargs histui get --field body | wl-copy"
```

### Rofi

```jsonc
"on-click-right": "histui get | rofi -dmenu -p 'Notifications' | cut -d'|' -f1 | xargs histui get --field body | wl-copy"
```

### Wofi

```jsonc
"on-click-right": "histui get | wofi --dmenu -p 'Notifications' | cut -d'|' -f1 | xargs histui get --field body | wl-copy"
```

## Hyprland Window Rules

For the floating TUI window, add to your `~/.config/hypr/hyprland.conf`:

```conf
windowrulev2 = float, class:^(histui-float)$
windowrulev2 = size 900 600, class:^(histui-float)$
windowrulev2 = center, class:^(histui-float)$
```

## Sway Configuration

For Sway users, use `swaymsg` instead of `hyprctl`:

```jsonc
"on-click-middle": "swaymsg exec 'kitty --class histui-float -e histui'"
```

And add to your Sway config:

```conf
for_window [app_id="histui-float"] floating enable, resize set 900 600, move position center
```

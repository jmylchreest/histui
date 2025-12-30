---
title: Configuration
description: histuid configuration file reference
sidebar_position: 1
---

# Configuration

histuid is configured via `~/.config/histui/histuid.toml`.

## Minimal Configuration

histuid works with no configuration file.
All settings have sensible defaults.

## Configuration File Location

```
~/.config/histui/histuid.toml
```

The configuration file is automatically loaded from this location if it exists.

## Full Reference

### log_level

Controls logging verbosity.

| Value | Description |
|-------|-------------|
| `debug` | Verbose debugging output |
| `info` | Normal operational messages (default) |
| `warn` | Warnings only |
| `error` | Errors only |

```toml
log_level = "info"
```

**Command-line override:**

```bash
histuid --log-level debug
```

### [display]

Controls notification popup position and behavior.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `position` | string | "top-right" | Popup position on screen |
| `offset_x` | int | 10 | Pixels from horizontal screen edge |
| `offset_y` | int | 10 | Pixels from vertical screen edge |
| `max_visible` | int | 5 | Maximum simultaneous popups (1-20) |
| `monitor` | int | 0 | Display monitor (0 = all, 1+ = specific monitor) |
| `new_on_top` | bool | false | New notifications appear at top of stack |
| `image_data_preview_size` | bytesize | "100 KiB" | Minimum size for showing image-data in body |

**Valid Positions:**

- `top-right` (default)
- `top-left`
- `top-center`
- `bottom-right`
- `bottom-left`
- `bottom-center`

:::note
Notification width and gap are controlled by the theme layout, not configuration.
See [Layout Reference](/docs/histuid/theming/layout-reference) for details.
:::

**Image Data Preview:**

The `image_data_preview_size` setting controls whether `image-data` from notifications is displayed in the popup body. Many messaging apps (Signal, Discord) send profile pictures via `image-data` which can clutter notifications.

| Value | Meaning |
|-------|---------|
| `"never"` or `"-1"` | Never display image-data |
| `"always"` or `"0"` | Always display image-data |
| Size threshold (e.g., `"100 KiB"`) | Only display if data size >= threshold |

The default `100 KiB` filters out profile pictures (typically 64-128px = ~64KB raw RGBA) while showing larger content like album art (300px+ = ~360KB+ raw).

:::tip
The `image-path` hint is **always** displayed since explicit file paths indicate intentional content images.
:::

**Byte Size Format:**

Byte sizes can be specified as:
- `"100"` - 100 bytes
- `"100 KB"` or `"100KB"` - 100,000 bytes (SI, base 1000)
- `"100 KiB"` - 102,400 bytes (IEC, base 1024)
- `"1 MB"` - 1,000,000 bytes
- `"1 MiB"` - 1,048,576 bytes

See [Icon Aliases](/docs/histuid/theming/icon-aliases#icons-vs-images-in-notifications) for more details on how icons and images are handled.

### [timeouts]

Controls how long notifications are displayed by urgency level.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `low` | duration | "0" | Timeout for low urgency (default: honor client) |
| `normal` | duration | "0" | Timeout for normal urgency (default: honor client) |
| `critical` | duration | "never" | Timeout for critical urgency |
| `fallback` | duration | "10s" | Fallback timeout when client says "server decides" |

**Timeout Values:**

| Value | Meaning |
|-------|---------|
| `"0"` or `"0s"` | Honor the client's requested timeout |
| `"never"` or `"-1"` | Notification never expires (must be dismissed) |
| Positive duration (e.g., `"5s"`, `"30s"`, `"2m"`) | Override with this timeout |

**Duration Format:**

Durations can be specified as:
- `"5s"` - 5 seconds
- `"500ms"` - 500 milliseconds
- `"2m"` - 2 minutes
- `"1h30m"` - 1 hour 30 minutes
- `"5000"` - 5000 milliseconds (integer)

**Client Timeout Behavior:**

When set to `"0"` (honor client), the daemon respects what the sending application requested:
- If the client requests `-1` (server decides), the daemon uses the `fallback` timeout (default 10s, critical always uses 0/never)
- If the client requests `0` (never expire), the notification stays until dismissed
- If the client requests a positive value, that timeout is used

### [history]

Controls notification history and retention.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `max_notifications` | int | 500 | Maximum notifications to keep (0 = unlimited) |
| `store_images` | bool | true | Store notification images for replay |

**Auto-Pruning Behavior:**

The daemon automatically prunes old notifications to stay within limits:

- **Buffer threshold**: Pruning triggers when count exceeds max by 10% (e.g., at 550 for max 500)
- **Debounce**: At most one prune operation every 5 minutes
- **Cascade cleanup**: Images for pruned notifications are automatically deleted
- **Startup prune**: A prune runs on daemon startup to clean up any excess

This design prevents constant disk I/O while keeping history within reasonable bounds.

### [behavior]

Controls daemon behavior.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `stack_duplicates` | bool | true | Combine identical notifications |
| `show_count` | bool | true | Show "(2)" for stacked duplicates |
| `pause_on_hover` | bool | true | Pause timeout when mouse hovers |
| `history_length` | int | 100 | Max notifications in session memory |

### [dnd]

Controls Do Not Disturb mode.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `enabled` | bool | false | Initial DnD state on daemon startup |
| `suppress_critical` | bool | false | Also suppress critical notifications in DnD mode |

**DnD Behavior:**

- When DnD is enabled, notification popups and sounds are suppressed
- Notifications are still persisted to history for later review
- By default (`suppress_critical = false`), critical notifications bypass DnD and are still shown
- Set `suppress_critical = true` to also silence critical notifications when DnD is active
- Use `histui dnd on/off/toggle` to control DnD state

### [mouse]

Controls mouse button actions on notification popups.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `left` | string | "dismiss" | Left-click action |
| `middle` | string | "do-action" | Middle-click action |
| `right` | string | "close-all" | Right-click action |

**Available Actions:**

| Action | Description |
|--------|-------------|
| `dismiss` | Close this notification |
| `do-action` | Execute the "default" action if present (see below) |
| `close-all` | Close all visible notifications |
| `context-menu` | Show context menu (if supported) |
| `none` | Do nothing |

### Understanding Notification Actions

D-Bus notifications can include **actions** - interactive buttons or click behaviors. Actions are key/label pairs sent by the application:

- **Key**: An identifier sent back to the app when triggered (e.g., `"reply"`, `"default"`)
- **Label**: Human-readable text shown to the user (e.g., `"Reply"`, `"Open"`)

**Special keys:**

| Key | Meaning |
|-----|---------|
| `"default"` | Action triggered by clicking the notification body (not a button) |
| Other keys | Displayed as action buttons in the notification |

**How `do-action` works:**

When you configure a mouse button to `do-action`, clicking the notification:
1. Looks for an action with key `"default"`
2. If found, sends an `ActionInvoked` D-Bus signal back to the originating application
3. The application handles it (e.g., focusing its window, opening a conversation)

**Common examples:**

| Application | Default Action | Effect |
|-------------|----------------|--------|
| kitty/alacritty | Focus terminal window | `["default", ""]` |
| Discord | Open conversation | `["default", ""]` |
| Slack | Open thread | `["default", "Open"]` |
| Firefox | Focus tab | `["default", ""]` |

**Visual indicator:**

Notifications with a "default" action display a subtle corner indicator (when using a theme that includes `<default-action-indicator />`). This signals to users that the notification is "clickable."

See the [CSS Reference](/docs/histuid/theming/css-reference#action-state-classes) for styling options and the [Layout Reference](/docs/histuid/theming/layout-reference#default-action-indicator-attributes) for theme customization.

**Viewing actions in histui:**

The histui TUI detail view shows all actions with their keys:

```
Extensions:
  Actions:
    default (no label) [click body to invoke]
    reply: Reply
```

### [audio]

Controls notification sounds.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `enabled` | bool | true | Enable notification sounds |
| `volume` | int | 80 | Global volume (0-100) |

**Audio Behavior:**

- Sounds are defined in theme manifests (see [Theming](/docs/histuid/theming))
- Only one sound plays at a time (newer sounds are skipped unless critical)
- Critical notifications can interrupt any playing sound
- Per-theme volumes are multiplied with the global volume

**Supported Formats:**

| Format | Extension | Notes |
|--------|-----------|-------|
| WAV | `.wav` | PCM only (16-bit signed little-endian). IEEE Float format is **not** supported |
| Ogg Vorbis | `.ogg` | Fully supported |
| MP3 | `.mp3` | Fully supported |

**Command-line Flags:**

```bash
# Disable all sounds (overrides theme and config)
histuid --no-audio

# Override global volume (0.0 to 1.0)
histuid --volume 0.5
```

## Example Configuration

```toml
# Logging: debug, info, warn, error
log_level = "info"

[display]
position = "top-right"
max_visible = 5
offset_x = 10
offset_y = 10
image_data_preview_size = "100 KiB"  # Filter small images like profile pics

[timeouts]
# Honor whatever the application requests (default for low/normal)
low = "0"
normal = "0"

# Critical notifications never expire (default)
critical = "never"

# Fallback timeout when client says "server decides" (-1)
fallback = "10s"

[behavior]
stack_duplicates = true
pause_on_hover = true

[dnd]
enabled = false           # Start with DnD off
suppress_critical = false # Critical notifications bypass DnD (default)

[mouse]
left = "dismiss"
middle = "do-action"
right = "close-all"

[audio]
enabled = true
volume = 80

[history]
max_notifications = 500   # 0 = unlimited
store_images = true       # Required for replay with images
```

## Icon Aliases

Icon aliases are configured in a separate file for easy sharing:

```
~/.config/histui/icon-aliases.toml
```

This maps application names to standard icon names:

```toml
# ~/.config/histui/icon-aliases.toml

[aliases]
zapzap = "whatsapp"
telegram-desktop = "telegram"
firefox-esr = "firefox"
vesktop = "discord"
signal-desktop = "signal"
```

Built-in aliases are provided for common apps. User aliases take precedence.
See [Advanced Theming](/docs/histuid/theming/advanced#icon-configuration) for more details.

## See Also

- [Theming](/docs/histuid/theming) - Customize appearance
- [Monitor Mode](/docs/histuid/monitor-mode) - Run with dunst

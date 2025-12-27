# histui Wishlist / TODO

Features and enhancements to consider for future development.

## Notification Rules Engine

### Per-Application Rules
Ability to define rules based on the sending application:

```toml
[[rules]]
match = { app_name = "discord" }
action = { suppress = true }  # Hide Discord notifications entirely

[[rules]]
match = { app_name = "slack" }
action = { urgency = "low", timeout = 3000 }  # Reduce urgency, short timeout

[[rules]]
match = { app_name = "signal" }
action = { persist = true, urgency = "critical" }  # Always require manual dismissal
```

### Content-Based Rules
Match notification content (summary, body) with regex or substring:

```toml
[[rules]]
match = { body_contains = "claire" }
action = { urgency = "critical", persist = true, bypass_dnd = true }

[[rules]]
match = { summary_regex = "(?i)error|failed|critical" }
action = { urgency = "critical" }

[[rules]]
match = { body_regex = "meeting (started|in 5)" }
action = { sound = "~/sounds/meeting.wav" }
```

### Rule Actions
- `suppress`: Don't show popup at all (still logged to history)
- `hide`: Don't show popup or log to history
- `persist`: Require manual dismissal (never timeout)
- `urgency`: Override urgency level (low/normal/critical)
- `timeout`: Override timeout in milliseconds
- `sound`: Override notification sound
- `bypass_dnd`: Show even when DnD is enabled
- `mute_sound`: Suppress sound for this notification

### Rule Matching Fields
- `app_name`: Exact match or regex on application name
- `app_name_contains`: Substring match on app name
- `summary`: Exact match or regex on summary
- `summary_contains`: Substring match on summary
- `body`: Exact match or regex on body
- `body_contains`: Substring match on body
- `category`: D-Bus notification category
- `urgency`: Match specific urgency level
- `desktop_entry`: Match desktop entry hint

---

## DnD (Do Not Disturb) Rules

### Time-Based DnD
Schedule DnD automatically:

```toml
[dnd.schedule]
enabled = true
start = "22:00"  # 10 PM
end = "08:00"    # 8 AM
days = ["mon", "tue", "wed", "thu", "fri"]  # Weekdays only
```

### Window/Application DnD Rules
Auto-enable DnD based on focused window or running applications:

```toml
[[dnd.rules]]
name = "fullscreen"
trigger = "fullscreen_app"  # Any fullscreen application
action = { enable_dnd = true, reason = "fullscreen application detected" }

[[dnd.rules]]
name = "zoom-meeting"
trigger = { app_running = "zoom" }
action = { enable_dnd = true, reason = "Zoom meeting in progress" }

[[dnd.rules]]
name = "gaming"
trigger = { window_class_regex = "steam_app_.*" }
action = { enable_dnd = true, reason = "gaming session" }

[[dnd.rules]]
name = "screenshare"
trigger = "screenshare_active"  # Requires integration with Wayland compositor
action = { enable_dnd = true, reason = "screen sharing" }
```

### DnD Trigger Types (enum)
- `user` - Manual toggle via CLI, TUI, or GUI
- `rule` - Triggered by a DnD rule (with rule name)
- `schedule` - Time-based schedule
- `system` - System event (fullscreen, screenshare, etc.)

---

## Popup Behavior Enhancements

### Persist/Pin Mode
Notifications with `persist = true` or the D-Bus "resident" hint:
- Never auto-timeout (ExpireTimeout = 0)
- Require explicit user dismissal
- Show a visual indicator (e.g., pin icon)
- Always shown even when DnD is active (if `bypass_dnd = true`)

### Stacking Behavior
- `stack_duplicates`: Combine identical notifications with count badge
- `group_by_app`: Group notifications from same app
- `max_visible_per_app`: Limit popups per application

### Popup Positioning
- Multi-monitor support with per-monitor positioning
- Follow focused window/mouse
- Avoid fullscreen windows

---

## Audio Enhancements

### Per-Rule Sounds
Override notification sounds based on rules:

```toml
[[rules]]
match = { app_name = "calendar" }
action = { sound = "~/sounds/calendar-reminder.wav" }
```

### Sound Themes
Support XDG sound themes and freedesktop.org sound naming:

```toml
[audio]
theme = "freedesktop"  # Use system sound theme
fallback = "~/sounds/default.wav"
```

---

## TUI Enhancements

- Inline actions: Execute notification actions from TUI
- Bulk operations: Select multiple notifications for dismiss/delete
- Search/filter by app, content, date range
- Notification preview with full body text
- Rule editor: Create/edit notification rules from TUI

---

## Integration Features

### Waybar Module Enhancements
- Show notification preview on hover
- Right-click context menu
- Scroll to cycle through notifications

### External Triggers
- HTTP/REST API for external control
- D-Bus interface for scripting
- Integration with Sway/Hyprland IPC for window/workspace events

---

## Technical Improvements

### GTK Window Tracking
Currently, the mapping between D-Bus notification IDs and GTK popup windows is managed by:
- `DisplayStateManager`: Maps D-Bus ID <-> histui ULID
- `Manager.popups`: Maps D-Bus ID -> `PopupState` (contains `Popup` GTK window)

Consider adding:
- Direct ID storage on the `Popup` struct for debugging
- Window naming with notification ID for compositor rules
- Layer-shell namespace per notification for window manager integration

### Performance
- Lazy loading of notification icons
- Virtual scrolling in TUI for large history
- Indexed search on history store

### Testing
- D-Bus integration tests with mock bus
- GTK widget tests
- Theme/CSS validation tests

---

## Shared Libraries

### Time Formatting (humanize)
Extract relative time formatting to shared library:
- `RelativeTime(t time.Time) string` - "5 minutes ago"
- `RelativeTimeShort(t time.Time) string` - "5m ago"
- `FormatDuration(d time.Duration) string` - "2h 15m"

Already implemented in tvarr's `pkg/format/format.go` - candidate for extraction.

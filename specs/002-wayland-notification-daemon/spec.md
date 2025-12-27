# Feature Specification: histuid - Wayland Notification Daemon

**Feature Branch**: `002-wayland-notification-daemon`
**Created**: 2025-12-27
**Status**: Draft
**Input**: User description: "Wayland notification daemon with CSS theming, DBus integration, shared history with histui, rich web-based popup rendering, audio support, and Do Not Disturb mode with window rules"

## Overview

histuid is a Wayland-native notification daemon that serves as a streaming input adapter for histui. It receives notifications via D-Bus, displays them as styled popup windows using GTK4/libadwaita on Wayland layer-shell surfaces, and persists them to the shared histui history store.

**Key architecture points**:
- **Shared state**: histuid and histui share the same history store and notification model
- **File watching**: histuid watches for changes to config, themes, audio files, and the history store for hot-reload
- **Preloading**: Active theme CSS and audio files are preloaded into memory for instant response
- **Unified CLI**: The existing `histui` CLI is extended with `set` and `dnd` commands - no separate control binary needed

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Receive and Display Notifications (Priority: P1)

As a Hyprland user, I want to receive desktop notifications that appear as styled popups on my screen, so I can be notified of events from applications without switching context.

**Why this priority**: This is the core functionality - receiving D-Bus notifications and displaying them. Without this, nothing else works.

**Independent Test**: Can be tested by running histuid and sending a notification via `notify-send`. The notification should appear as a popup on screen.

**Acceptance Scenarios**:

1. **Given** histuid is running as the notification daemon, **When** an application sends a notification via D-Bus, **Then** a styled popup appears on screen at the configured position
2. **Given** a notification is displayed, **When** the configured timeout elapses, **Then** the popup automatically disappears
3. **Given** a critical notification is received, **When** it is displayed, **Then** it uses critical urgency styling and remains visible until dismissed
4. **Given** multiple notifications arrive quickly, **When** they are displayed, **Then** they stack according to configuration (max visible limit, stacking behavior)

---

### User Story 2 - Shared History with histui (Priority: P2)

As a user of both histui and histuid, I want notifications received by histuid to be immediately available in histui's history, so I can browse and manage all my notifications in one place.

**Why this priority**: The shared history is the key integration point that makes histuid more than just another notification daemon. It enables the unified notification management experience.

**Independent Test**: Can be tested by sending a notification, then immediately querying `histui get` to verify the notification appears in history.

**Acceptance Scenarios**:

1. **Given** histuid receives a notification, **When** it is displayed, **Then** it is simultaneously persisted to the histui history store
2. **Given** a notification exists in histuid, **When** I dismiss it via histui TUI, **Then** the popup disappears from histuid immediately
3. **Given** I have histui TUI open, **When** a new notification arrives in histuid, **Then** it appears in the histui list in real-time
4. **Given** histuid is not running, **When** I start histui, **Then** I can still browse all previously persisted notifications

---

### User Story 3 - CSS Theming and Rich Rendering (Priority: P3)

As a user who customizes my desktop appearance, I want to style notification popups using CSS themes with support for urgency-based colors and rich content (animated images, icons), so notifications match my desktop aesthetic.

**Why this priority**: Visual customization is a key differentiator. GTK4/libadwaita with native CSS theming enables rich, modern styling that integrates with the desktop aesthetic.

**Independent Test**: Can be tested by creating a custom CSS theme, configuring histuid to use it, and verifying notifications render with the theme's styles.

**Acceptance Scenarios**:

1. **Given** I have a custom CSS theme in my themes folder, **When** I configure histuid to use it, **Then** notifications render using that theme's styles
2. **Given** a notification with an animated GIF icon, **When** it is displayed, **Then** the animation plays in the popup
3. **Given** different urgency levels, **When** notifications are displayed, **Then** each urgency uses its configured color scheme via CSS class inheritance
4. **Given** a theme defines symbolic icon styles, **When** a notification with a symbolic icon is displayed, **Then** the icon renders using the theme's icon colors

---

### User Story 4 - Audio Notifications (Priority: P4)

As a user, I want to hear distinct sounds for notifications based on urgency, so I can be alerted to important notifications even when not looking at the screen.

**Why this priority**: Audio feedback enhances notification awareness without visual attention. Essential for accessibility and hands-busy scenarios.

**Independent Test**: Can be tested by configuring sounds for each urgency level, sending notifications of each type, and verifying the correct sound plays.

**Acceptance Scenarios**:

1. **Given** audio is configured for normal urgency, **When** a normal notification arrives, **Then** the configured sound plays
2. **Given** different sounds for low/normal/critical urgency, **When** notifications of each type arrive, **Then** the appropriate sound plays for each
3. **Given** Do Not Disturb mode is enabled, **When** a notification arrives, **Then** no sound plays regardless of urgency
4. **Given** audio is disabled in configuration, **When** a notification arrives, **Then** no sound plays

---

### User Story 5 - Do Not Disturb Mode (Priority: P5)

As a user who needs focus time, I want to enable Do Not Disturb mode to suppress notification popups and sounds while still capturing notifications for later review.

**Why this priority**: Focus mode is essential for productivity. Notifications are still persisted but don't interrupt the user.

**Independent Test**: Can be tested by enabling DnD mode, sending notifications, and verifying they appear in history but don't produce popups or sounds.

**Acceptance Scenarios**:

1. **Given** DnD mode is enabled, **When** a notification arrives, **Then** it is persisted to history but no popup or sound is produced
2. **Given** DnD mode is enabled, **When** a critical notification arrives, **Then** it follows the configured critical-bypass setting (bypass or respect DnD)
3. **Given** DnD mode is disabled, **When** a notification arrives, **Then** normal popup and sound behavior resumes
4. **Given** DnD mode is enabled via waybar toggle, **When** I check histui, **Then** the status shows DnD is active

---

### User Story 6 - Interactive Notification Actions (Priority: P6)

As a user, I want to interact with notifications by clicking to dismiss, middle-clicking for default action, or using keyboard shortcuts, so I can respond to notifications efficiently.

**Why this priority**: Interactivity enables quick responses without switching to histui. Mouse and keyboard support covers different user preferences.

**Independent Test**: Can be tested by clicking on a notification popup and verifying it dismisses (or performs configured action).

**Acceptance Scenarios**:

1. **Given** a notification is displayed, **When** I click on it, **Then** it performs the configured click action (default: dismiss)
2. **Given** a notification with a default action, **When** I middle-click on it, **Then** the default action is invoked
3. **Given** a notification is displayed, **When** I hover and press the dismiss key, **Then** the notification is dismissed
4. **Given** a notification is dismissed, **When** I check histui, **Then** the notification shows as dismissed with timestamp

---

### User Story 7 - Automatic DnD Based on Window Focus (Priority: P7 - Future)

As a user, I want notifications to be automatically suppressed when specific windows are focused (like fullscreen video or gaming), so I'm not interrupted during immersive activities.

**Why this priority**: Advanced feature that requires Hyprland IPC integration. Marked as future enhancement to avoid scope creep.

**Independent Test**: Can be tested by configuring a window rule, focusing that window, and verifying DnD mode activates automatically.

**Acceptance Scenarios**:

1. **Given** a window rule is configured for "mpv" class, **When** an mpv window gains focus, **Then** DnD mode is automatically enabled
2. **Given** automatic DnD is active due to window focus, **When** focus moves to a different window, **Then** DnD mode is automatically disabled
3. **Given** manual DnD is enabled, **When** a window rule would also enable DnD, **Then** manual DnD takes precedence (remains enabled when focus changes)

---

### Edge Cases

- What happens when histuid starts but no histui history store exists? (Create it automatically)
- What happens when the D-Bus notification service is already claimed by another daemon? (Exit with clear error message)
- How does histuid handle notifications with extremely long body text (10000+ characters)? (Truncate with scroll/expand option)
- What happens when the theme CSS file is invalid or missing? (Fall back to embedded default theme)
- How does histuid behave when Wayland compositor is not running (SSH session)? (Exit with clear error message)
- What happens when audio playback fails (missing sound file, audio server unavailable)? (Log warning, continue without sound)
- How does histuid handle rapid-fire notifications (100+ per second)? (Rate limit display, persist all to history)
- What happens when the history store file is locked by histui? (Use file locking with retry, or shared lock for reads)
- How does histuid handle notifications with embedded images (D-Bus image-data hint)? (Decode and display via GdkPixbufAnimation)
- What happens when screen resolution changes while notifications are displayed? (Reposition popups to valid screen coordinates)

## Requirements *(mandatory)*

### Functional Requirements

**D-Bus Integration**:
- **FR-001**: System MUST implement the org.freedesktop.Notifications D-Bus interface
- **FR-002**: System MUST support all standard notification hints (urgency, category, icon, actions, etc.)
- **FR-003**: System MUST support notification actions and invoke them when triggered
- **FR-004**: System MUST support notification replacement (replaces_id)
- **FR-005**: System MUST emit NotificationClosed signals with appropriate reason codes
- **FR-006**: System MUST support GetCapabilities to advertise supported features
- **FR-007**: System MUST support GetServerInformation to identify the daemon

**Wayland Display**:
- **FR-008**: System MUST create borderless popup windows using Wayland layer-shell protocol
- **FR-009**: System MUST position popups according to configuration (corner, offset)
- **FR-010**: System MUST support stacking multiple notifications vertically
- **FR-011**: System MUST respect maximum visible notification limit
- **FR-012**: System MUST use GTK4/libadwaita widgets with native CSS theming for popup content
- **FR-013**: System MUST support configurable popup dimensions (width, max-height)

**History Integration**:
- **FR-014**: System MUST persist all received notifications to the histui history store immediately
- **FR-015**: System MUST use the same persistence format and location as histui
- **FR-016**: System MUST share the same notification model (ULID, states, timestamps) as histui
- **FR-017**: System MUST update notification states when user interacts with popups (dismiss, etc.)
- **FR-018**: System MUST reflect external state changes in displayed popups immediately (e.g., dismiss via histui closes popup)

**Theming**:
- **FR-019**: System MUST load CSS themes from `~/.config/histui/themes/` directory
- **FR-020**: System MUST support a default theme embedded in the binary
- **FR-021**: System MUST apply urgency-specific CSS classes (urgency-low, urgency-normal, urgency-critical)
- **FR-022**: System MUST support CSS variables for color customization
- **FR-023**: System MUST support symbolic icon rendering with theme-defined colors
- **FR-024**: System MUST support animated images (GIF, APNG, WebP) in notification content
- **FR-025**: System MUST support custom fonts including symbolic/icon fonts (loaded via CSS @font-face)

**Timeouts and Behavior**:
- **FR-026**: System MUST support per-urgency timeout configuration
- **FR-027**: System MUST support "never expire" for critical notifications (timeout = 0)
- **FR-028**: System MUST support pausing timeout on mouse hover
- **FR-029**: System MUST support duplicate notification stacking (combine identical notifications with count)

**Audio**:
- **FR-030**: System MUST support per-urgency sound configuration
- **FR-031**: System MUST support common audio formats (WAV, OGG, MP3)
- **FR-032**: System MUST suppress sounds when Do Not Disturb is enabled
- **FR-033**: System MUST support volume configuration for notification sounds

**Do Not Disturb**:
- **FR-034**: System MUST support DnD mode toggle (via histui CLI updating shared state)
- **FR-035**: System MUST persist notifications to history even when DnD is active
- **FR-036**: System MUST support critical notification bypass option for DnD
- **FR-037**: System MUST expose DnD state for status queries

**Mouse Interaction**:
- **FR-038**: System MUST support configurable mouse button actions (left, middle, right click)
- **FR-038a**: System MUST display action buttons only on hover or focus (not always visible)
- **FR-039**: System MUST support close-all action to dismiss all visible notifications
- **FR-040**: System MUST support click-through option for non-interactive display mode

**Configuration & Hot-Reload**:
- **FR-041**: System MUST use TOML configuration file at `~/.config/histui/histuid.toml`
- **FR-042**: System MUST watch configuration file for changes and hot-reload automatically
- **FR-043**: System MUST watch theme CSS files and hot-reload on change
- **FR-044**: System MUST watch audio files and hot-reload on change
- **FR-045**: System MUST preload active theme and audio files into memory for instant playback
- **FR-046**: System MUST provide sensible defaults for all configuration options

**histui CLI Extensions**:
- **FR-047**: `histui get --format ids` MUST output bare ULIDs, one per line
- **FR-048**: `histui get --filter` MUST support rich filter expressions (see Filter Syntax below)
- **FR-049**: `histui set` MUST accept a single ULID as positional argument
- **FR-050**: `histui set --stdin` MUST extract ULIDs from input (bare ULID or scan for ULID pattern in line)
- **FR-051**: `histui set --stdin --format json` MUST accept JSON with `histui_id` field
- **FR-052**: `histui set` MUST support `--dismiss`, `--undismiss`, `--seen`, `--delete` flags
- **FR-053**: `histui dnd` MUST support `on`, `off`, `toggle` subcommands

**Integration**:
- **FR-054**: System MUST work with Hyprland compositor's layer-shell implementation
- **FR-055**: System MUST watch the shared history store for external state changes
- **FR-056**: System MUST provide example waybar configuration

### Key Entities

- **Notification**: Extended from histui's model with additional display state:
  - All fields from histui Notification model (ULID, timestamps, urgency, content, etc.)
  - DisplayState: pending, visible, expired, dismissed
  - PopupID: window identifier for the popup
  - TimeoutRemaining: milliseconds until auto-dismiss
  - Actions: list of available actions from D-Bus notification

- **Theme**: CSS-based styling configuration:
  - Name: theme identifier (filename without extension)
  - Path: filesystem path to CSS file
  - Variables: CSS custom properties for color schemes
  - UrgencyClasses: CSS class mappings for urgency levels

- **Configuration**: Runtime settings:
  - Display: position, dimensions, max-visible, gap
  - Timeouts: per-urgency timeout values in milliseconds
  - Audio: per-urgency sound file paths, volume level
  - DnD: enabled state, critical bypass flag
  - Theme: active theme name
  - Mouse: per-button action mappings

- **WindowRule** (Future Enhancement): Automatic DnD trigger:
  - MatchType: class, title, app_id
  - Pattern: regex or glob pattern to match
  - Action: enable-dnd when matched window is focused

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Notifications appear on screen within 100ms of D-Bus signal receipt
- **SC-002**: Notifications persist to history within 50ms of display
- **SC-003**: State changes from histui reflect in displayed popups within 200ms
- **SC-004**: System supports 50 simultaneous notifications without performance degradation
- **SC-005**: CSS theme changes apply without daemon restart (via reload signal)
- **SC-006**: Audio playback begins within 50ms of notification arrival
- **SC-007**: Memory usage remains under 100MB with 1000 notifications in session
- **SC-008**: Daemon starts and claims D-Bus name within 1 second
- **SC-009**: Popup animations render smoothly (60fps) on standard hardware
- **SC-010**: DnD mode toggle reflects in waybar status within 500ms

## Assumptions

- User is running a Wayland compositor with layer-shell support (Hyprland, Sway, etc.)
- User has D-Bus session bus available
- User has write access to XDG config and data directories
- GTK4 and libadwaita runtime libraries are available (gtk4, libadwaita, gtk4-layer-shell)
- PipeWire or PulseAudio is available for audio playback (audio is optional feature)
- histui internal packages are available for import (shared Go module)
- CSS theming knowledge is expected for custom theme creation
- The freedesktop.org notification specification (v1.2) is the authoritative reference
- Hyprland is the primary target compositor; other Wayland compositors are secondary

## Configuration Example

```toml
# ~/.config/histui/histuid.toml

[display]
position = "top-right"      # top-left, top-right, bottom-left, bottom-right, top-center, bottom-center
offset_x = 10               # Pixels from screen edge
offset_y = 10
width = 350                 # Popup width in pixels
max_height = 200            # Maximum popup height
max_visible = 5             # Maximum simultaneous popups
gap = 5                     # Gap between stacked popups
monitor = 0                 # Monitor index (0 = primary)

[timeouts]
low = 5000                  # Milliseconds (0 = never expire)
normal = 10000
critical = 0                # Critical never expires by default

[behavior]
stack_duplicates = true     # Combine identical notifications
show_count = true           # Show "(2)" for stacked duplicates
pause_on_hover = true       # Pause timeout when mouse hovers
history_length = 100        # Max notifications in session memory

[audio]
enabled = true
volume = 80                 # 0-100

[audio.sounds]
low = ""                    # No sound for low urgency
normal = "~/.config/histui/sounds/notification.wav"
critical = "~/.config/histui/sounds/critical.wav"

[theme]
name = "default"            # Theme name from themes directory
# Themes are CSS files in ~/.config/histui/themes/

[dnd]
enabled = false
critical_bypass = true      # Show critical even in DnD mode

[mouse]
left = "dismiss"            # dismiss, do-action, close-all, context-menu
middle = "do-action"
right = "close-all"

# Future: Window rules for automatic DnD
# [[window_rules]]
# match = "class"
# pattern = "^mpv$"
# action = "enable-dnd"
```

## Theme CSS Example

```css
/* ~/.config/histui/themes/default.css */

:root {
  --bg-color: #1e1e2e;
  --fg-color: #cdd6f4;
  --border-color: #45475a;
  --border-radius: 8px;

  /* Urgency colors */
  --low-bg: #1e1e2e;
  --low-accent: #a6e3a1;
  --normal-bg: #1e1e2e;
  --normal-accent: #89b4fa;
  --critical-bg: #1e1e2e;
  --critical-accent: #f38ba8;
}

.notification {
  background: var(--bg-color);
  color: var(--fg-color);
  border: 1px solid var(--border-color);
  border-radius: var(--border-radius);
  padding: 12px;
  font-family: "Inter", sans-serif;
}

.notification.urgency-low {
  border-left: 3px solid var(--low-accent);
}

.notification.urgency-normal {
  border-left: 3px solid var(--normal-accent);
}

.notification.urgency-critical {
  border-left: 3px solid var(--critical-accent);
  animation: pulse 1s ease-in-out infinite;
}

.notification .icon {
  width: 48px;
  height: 48px;
  margin-right: 12px;
}

.notification .summary {
  font-weight: 600;
  font-size: 14px;
}

.notification .body {
  font-size: 12px;
  opacity: 0.9;
  margin-top: 4px;
}

.notification .app-name {
  font-size: 10px;
  opacity: 0.7;
}

.notification .timestamp {
  font-size: 10px;
  opacity: 0.5;
}

/* Symbolic icon coloring */
.notification .icon.symbolic {
  filter: brightness(0) saturate(100%) invert(85%);
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.8; }
}
```

## Waybar Integration Example

```json
{
    "custom/notification": {
        "exec": "histui status",
        "return-type": "json",
        "interval": 1,
        "format": "{icon}",
        "format-icons": {
            "enabled": "󰂚",
            "paused": "󰂛",
            "paused-pending": "󰂛 ({})",
            "unavailable": "󰂭"
        },
        "on-click": "histui",
        "on-click-right": "histui dnd toggle"
    }
}
```

## Command Structure

histui extends its existing command structure with state modification:

```bash
# Existing commands
histui                  # TUI browser
histui get [flags]      # Query notifications
histui status           # Waybar JSON (extended to show DnD state)
histui prune [flags]    # Clean up history

# New commands
histui set <ulid> [flags]   # Modify notification state
histui dnd [on|off|toggle]  # Control Do Not Disturb mode
```

### `histui set` - Modify Notification State

```bash
# Single notification by ULID
histui set <ulid> --dismiss      # Mark notification as dismissed
histui set <ulid> --undismiss    # Restore dismissed notification
histui set <ulid> --seen         # Mark as seen
histui set <ulid> --delete       # Permanently delete from history

# Multiple notifications via stdin
histui set --stdin --dismiss     # Read ULIDs from stdin (one per line)
histui set --stdin --delete      # Bulk delete
```

**New format option for piping**: `--format ids` outputs just ULIDs, one per line:

```bash
# Simple pipeline - dismiss all slack notifications older than 1 day
histui get --app-filter slack --since 1d --format ids | histui set --stdin --dismiss

# Delete all low urgency notifications
histui get --urgency low --format ids | histui set --stdin --delete

# Interactive: select with fuzzel, then dismiss
histui get --format dmenu | fuzzel -d | histui set --stdin --dismiss
```

For the dmenu pipeline to work, `histui set --stdin` extracts ULIDs from input lines:
- If line is a bare ULID (26 chars, base32), use directly
- Otherwise, scan line for ULID pattern and extract it
- This allows dmenu format output to pipe directly without `cut`

**JSON format** for jq pipelines:

`histui get --format json` outputs newline-delimited JSON (NDJSON) - one JSON object per line, making it easy to stream through jq:

```bash
# Filter with jq, pipe back - jq outputs are directly compatible with set
histui get --format json | jq 'select(.urgency == 0)' | histui set --stdin --format json --dismiss

# Complex jq filtering
histui get --format json | jq 'select(.app_name | test("slack|discord"))' | histui set --stdin --format json --seen

# Count by app
histui get --format json | jq -s 'group_by(.app_name) | map({app: .[0].app_name, count: length})'
```

The JSON format uses the same field names as the internal model (`histui_id`, `app_name`, `summary`, `body`, `urgency`, `timestamp`, etc.), so jq filters are intuitive.

`histui set --stdin --format json` accepts:
- NDJSON (one object per line) - matches `get` output
- JSON array
- Just needs `histui_id` field to identify the notification

When histuid is running and watching the shared store, state changes are reflected immediately in popup display.

### `histui dnd` - Do Not Disturb

```bash
histui dnd              # Show current DnD state
histui dnd on           # Enable DnD
histui dnd off          # Disable DnD
histui dnd toggle       # Toggle DnD state
```

DnD state is persisted to the shared state file. histuid watches this and responds accordingly.

The `histui status` command is extended to include DnD state, maintaining backwards compatibility with the existing waybar integration.

### Filter Syntax

The `--filter` flag accepts a rich expression for filtering notifications:

```bash
histui get --filter "app=slack,urgency>=normal,time<1h"
histui get --filter "app~discord|slack,dismissed=false"
```

**Field operators**:

| Field | Operators | Examples |
|-------|-----------|----------|
| `app` | `=`, `~` (regex) | `app=slack`, `app~^(discord\|slack)$` |
| `summary` | `=`, `~` (regex) | `summary~error`, `summary=Download Complete` |
| `body` | `=`, `~` (regex) | `body~https://` |
| `urgency` | `=`, `>`, `>=`, `<`, `<=` | `urgency=critical`, `urgency>low`, `urgency>=normal` |
| `time` | `>`, `>=`, `<`, `<=` | `time<1h`, `time>=30m`, `time<2d` |
| `seen` | `=` | `seen=true`, `seen=false` |
| `dismissed` | `=` | `dismissed=true`, `dismissed=false` |
| `source` | `=` | `source=histuid`, `source=dunst` |

**Combining conditions**:
- Comma (`,`) = AND: `app=slack,urgency=critical`
- Pipe in regex = OR for same field: `app~slack|discord`

**Time duration format**: `5m`, `1h`, `2d`, `1w` (minutes, hours, days, weeks)

**Examples**:

```bash
# All critical notifications from the last hour
histui get --filter "urgency=critical,time<1h" --format ids | histui set --stdin --seen

# Dismiss all discord notifications older than 1 day
histui get --filter "app~discord,time>=1d" --format ids | histui set --stdin --dismiss

# Show unseen notifications from slack or teams
histui get --filter "app~slack|teams,seen=false"

# Delete all dismissed notifications older than a week
histui get --filter "dismissed=true,time>=1w" --format ids | histui set --stdin --delete
```

## Clarifications

This section documents design decisions made during specification review.

| Question | Decision | Rationale |
|----------|----------|-----------|
| How should action buttons be displayed in the popup UI? | On hover/focus only | Keeps the notification compact by default while still providing easy access to actions. Matches the behavior of modern notification systems like GNOME. |
| What rendering technology for popup content? | GTK4/libadwaita (not WebKit) | Reduced attack surface (no browser engine), native CSS theming support, better desktop integration, lower memory footprint. |

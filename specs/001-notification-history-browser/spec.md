# Feature Specification: histui - Notification History Browser

**Feature Branch**: `001-notification-history-browser`
**Created**: 2025-12-26
**Updated**: 2025-12-26
**Status**: Draft
**Project**: histui (history + TUI)
**Input**: User description: "A notification history browser tool for Hyprland/Waybar with pluggable input adapters (dunst, stdin), centralized history store with persistence, and multiple output modes (TUI, dmenu list, waybar status)"

## Command Structure

histui uses subcommands for different operations:

```bash
histui                  # Default: launches TUI (same as histui tui)
histui get [flags]      # Query and output notifications
histui status           # Waybar JSON status output
histui tui              # Interactive TUI browser
histui prune [flags]    # Clean up old notifications
```

**Default behavior**: Running `histui` with no subcommand launches the TUI.

### `histui get` - Query Notifications

Outputs notifications with flexible formatting. Behavior depends on stdin:
- **No stdin**: Output all notifications (filtered by flags)
- **With stdin**: Look up the specific notification matching the input line

**Field flags:**
```bash
--app, -a           Include app name
--title, -t         Include summary/title
--body, -b          Include body text
--timestamp, -T     Include timestamp (ISO 8601)
--time-relative     Include relative time (e.g., "5m ago")
--ulid              Include ULID (for reliable piping)
--all               All fields
```

**Format presets:**
```bash
--format dmenu      # app | summary - body_truncated | relative_time
--format json       # JSON array
--format "{{.Field}}"  # Go template
```

**Filtering:**
```bash
--since 48h         # Only notifications from last 48h (default)
--since 1h          # Last hour
--since 7d          # Last week
--since 0           # All time (no filter)
--app-filter NAME   # Filter by app name
--urgency LEVEL     # Filter by urgency (low, normal, critical)
--limit N           # Maximum notifications to return
```

**Sorting:**
```bash
--sort timestamp:desc   # Newest first (default)
--sort timestamp:asc    # Oldest first
--sort app:asc          # Alphabetical by app
--sort urgency:desc     # Critical first
```

### `histui status` - Waybar Status

Outputs JSON for waybar custom module integration.

### `histui tui` - Interactive Browser

Full-screen terminal UI with keyboard navigation, search, and clipboard support.

#### TUI Design

**Philosophy**: Minimal, keyboard-driven interface. No chrome, no distractions. Focus on content.

**Main View** (notification list):
```
┌─────────────────────────────────────────────────────────────────────────────┐
│ histui                                                    5 notifications   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│ ▶ firefox          Download Complete                              2m ago   │
│   slack            New message from @john                         15m ago  │
│   discord          Server notification                            1h ago   │
│   spotify          Now playing: Artist - Song                     2h ago   │
│   kitty            Build finished                                 3h ago   │
│                                                                             │
│                                                                             │
│                                                                             │
│                                                                             │
│                                                                             │
│                                                                             │
│                                                                             │
├─────────────────────────────────────────────────────────────────────────────┤
│ j/k:nav  /:search  Enter:view  p:print  y:copy  d:delete  q:quit            │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Search Mode** (after pressing `/`):
```
┌─────────────────────────────────────────────────────────────────────────────┐
│ histui                                                    2 matches         │
├─────────────────────────────────────────────────────────────────────────────┤
│ /fire█                                                                      │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│ ▶ firefox          Download Complete                              2m ago   │
│   firefox          Page loaded: GitHub                            5h ago   │
│                                                                             │
│                                                                             │
│                                                                             │
│                                                                             │
│                                                                             │
│                                                                             │
│                                                                             │
│                                                                             │
│                                                                             │
├─────────────────────────────────────────────────────────────────────────────┤
│ Enter:select  Esc:cancel  Ctrl-U:clear                                      │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Detail View** (after pressing Enter):
```
┌─────────────────────────────────────────────────────────────────────────────┐
│ histui › firefox                                          2 minutes ago    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────┐                                                                │
│  │  (icon) │   Download Complete                                           │
│  │         │   ──────────────────                                           │
│  └─────────┘   myfile.zip has finished downloading.                        │
│                                                                             │
│                Click to open download folder.                              │
│                                                                             │
│  ─────────────────────────────────────────────────────────────────────────  │
│                                                                             │
│  App:       firefox                                                         │
│  Time:      2025-12-26 14:32:15                                            │
│  Urgency:   normal                                                          │
│  ULID:      01HQGXK5P0000000000000000                                      │
│                                                                             │
├─────────────────────────────────────────────────────────────────────────────┤
│ Esc:back  Enter:print  y:copy body  Y:copy all  o:open URL  q:quit          │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Keybindings**:

| Key | Mode | Action |
|-----|------|--------|
| `j` / `↓` | List | Move selection down |
| `k` / `↑` | List | Move selection up |
| `g` / `Home` | List | Go to first item |
| `G` / `End` | List | Go to last item |
| `Ctrl-d` | List | Page down |
| `Ctrl-u` | List | Page up |
| `/` | List | Enter search mode (fuzzy filter) |
| `Enter` | List | View notification detail |
| `p` | List | Print to stdout and exit (for piping) |
| `y` | List | Copy selected notification body |
| `d` | List | Delete selected notification |
| `q` | List | Quit (no output) |
| | | |
| `Esc` | Search | Cancel search, return to full list |
| `Enter` | Search | Confirm filter, select first match |
| `Ctrl-u` | Search | Clear search input |
| | | |
| `Esc` | Detail | Return to list |
| `Enter` | Detail | Print to stdout and exit (for piping) |
| `y` | Detail | Copy notification body |
| `Y` | Detail | Copy all notification details |
| `o` | Detail | Open URL in body (if present) |
| `q` | Detail | Quit (no output) |

**Pipeline Usage**:

The TUI can be used in pipelines like `fzf`:

```bash
# Browse notifications, select one, copy body to clipboard
histui | wl-copy

# Browse and pipe to another tool
histui | xargs -I {} notify-send "Selected: {}"

# With custom output template
histui --output-template '{{.Body}}' | wl-copy
```

When `Enter` is pressed (in detail view) or `p` is pressed (in list view), the selected notification is printed to stdout using the configured template, then TUI exits. Pressing `q` exits without output.

**Image Rendering**:

Icons are rendered in the detail view using the Kitty graphics protocol (with fallback to Sixel, iTerm2, or no image). The icon area is approximately 64x64 pixels.

- **Library**: `blacktop/go-termimg` - Modern Go library with universal protocol support
- **Protocols supported**: Kitty (preferred), Sixel, iTerm2, halfblocks fallback
- **Kitty 0.45+**: Supports animated PNG/WebP in addition to existing animated GIF support
- **Graceful degradation**: If no image protocol is available, icon area shows `[icon]` placeholder

**Search Behavior**:
- Fuzzy matching across app name, summary, and body
- Real-time filtering as you type
- Matches highlighted in results
- Empty search returns to full list

### `histui prune` - Clean Up History

Removes old notifications from the store.

```bash
histui prune                    # Remove older than 48h (default)
histui prune --older-than 7d    # Custom age threshold
histui prune --keep 100         # Keep at most N notifications
histui prune --dry-run          # Preview what would be removed
```

---

## Configuration File

Location: `~/.config/histui/config.toml` (follows XDG spec)

```toml
# ~/.config/histui/config.toml

# Default filtering
[filter]
since = "48h"           # Default time filter (0 = all time)
limit = 0               # Max notifications (0 = unlimited)

# Default sorting
[sort]
field = "timestamp"     # timestamp, app, urgency
order = "desc"          # asc, desc

# Prune defaults
[prune]
older_than = "48h"      # Default age threshold
keep = 0                # Max to keep (0 = unlimited)

# Output templates (Go text/template syntax)
[templates]
# Used by: histui get --format <name>
dmenu = "{{.AppName}} | {{.Summary}} - {{.BodyTruncated 50}} | {{.RelativeTime}}"
full = "{{.Timestamp | formatTime}} {{.AppName}}: {{.Summary}}\n{{.Body}}"
body = "{{.Body}}"
json = ""               # Empty = use built-in JSON marshaling

# TUI output template (used when pressing Enter/p to print)
tui_output = "{{.Timestamp | formatTime}} {{.AppName}}: {{.Summary}}\n{{.Body}}"

# Custom templates
[templates.custom]
slack = "{{.Summary}}: {{.Body}}"
minimal = "{{.AppName}}: {{.Summary}}"

# TUI settings
[tui]
show_icons = true       # Render notification icons (if terminal supports)
icon_size = 64          # Icon size in pixels
show_help = true        # Show keybind hints in footer

# Clipboard (TUI mode only)
[clipboard]
command = "wl-copy"     # Clipboard command (auto-detected if empty)
```

### Template Functions

Available functions in templates:

| Function | Description | Example |
|----------|-------------|---------|
| `formatTime` | Format timestamp | `{{.Timestamp \| formatTime}}` → `2025-12-26 14:32:15` |
| `formatTimeRFC3339` | RFC3339 format | `{{.Timestamp \| formatTimeRFC3339}}` → `2025-12-26T14:32:15Z` |
| `relativeTime` | Human relative | `{{.RelativeTime}}` → `5m ago` |
| `truncate N` | Truncate string | `{{.Body \| truncate 50}}` → `First 50 chars...` |
| `escapeJSON` | JSON escape | `{{.Body \| escapeJSON}}` |
| `upper` / `lower` | Case conversion | `{{.AppName \| upper}}` → `FIREFOX` |

### Template Fields

Available fields on Notification:

```go
.HistuiID         // ULID string
.HistuiSource     // "dunst", "stdin", "dbus"
.AppName          // Application name
.Summary          // Notification title
.Body             // Notification body
.Timestamp        // Unix timestamp (use with formatTime)
.Urgency          // 0, 1, 2
.UrgencyName      // "low", "normal", "critical"
.Category         // Freedesktop category
.IconPath         // Path to icon file
.RelativeTime     // Computed: "5m ago", "2h ago"
.BodyTruncated N  // Method: truncated body
```

### Command-Line Override

CLI flags override config file settings:

```bash
# Override template for this invocation
histui get --format '{{.AppName}}: {{.Body}}'

# Override TUI output
histui --output-template '{{.Body}}' | wl-copy
```

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Quick Notification Lookup via Waybar (Priority: P1)

As a Hyprland user, I want to click a notification icon in my waybar and quickly browse my recent notifications using a fuzzy finder (fuzzel/walker/dmenu), so I can find and act on notifications I may have missed.

**Why this priority**: This is the primary use case that solves the immediate problem of reviewing missed notifications in a fast, keyboard-driven workflow. It integrates with the existing waybar setup and replaces the current shell script solution.

**Independent Test**: Can be fully tested by running the tool with dmenu-style output and piping to fuzzel/walker. Delivers value as a standalone notification viewer.

**Acceptance Scenarios**:

1. **Given** histui is installed and waybar is configured, **When** I click the notification icon, **Then** a fuzzy finder appears showing my recent notification history
2. **Given** I have 50 notifications in history, **When** I type a search term in fuzzel, **Then** the list filters to matching notifications
3. **Given** I select a notification from the list, **When** I press Enter, **Then** the full notification details are piped to my clipboard via shell pipeline

**Example waybar integration:**
```bash
# On click: show notifications in fuzzel, copy selected body to clipboard
histui get --format dmenu --ulid | fuzzel -d | histui get --body | wl-copy
```

---

### User Story 2 - Interactive TUI Browser (Priority: P2)

As a power user, I want an interactive terminal interface to browse, search, and manage my notification history with keyboard navigation, so I can review notifications in detail without relying on external tools.

**Why this priority**: Provides a richer experience for users who want detailed notification viewing. Builds on the core parsing logic from P1.

**Independent Test**: Can be tested by launching the TUI mode directly and navigating notifications with keyboard. Delivers value as a standalone notification manager.

**Acceptance Scenarios**:

1. **Given** I launch `histui tui`, **When** the interface loads, **Then** I see a scrollable list of notifications with app name, summary, timestamp, and relative time
2. **Given** I am viewing the notification list, **When** I press `/` and type a search term, **Then** the list filters to matching notifications
3. **Given** I have a notification selected, **When** I press Enter, **Then** I see the full notification body in a detail view
4. **Given** I am viewing notification details, **When** I press `y`, **Then** the notification body is copied to my clipboard
5. **Given** I am viewing the list, **When** I press `q`, **Then** the TUI exits cleanly

---

### User Story 3 - Waybar Status Integration (Priority: P3)

As a Hyprland user, I want the waybar notification icon to show the current notification status (enabled/paused) and pending count, so I have visual feedback about my notification state.

**Why this priority**: Enhances the existing waybar integration with a more robust status indicator. Can leverage the same tool for both status and history browsing.

**Independent Test**: Can be tested by running the status output mode and verifying JSON output format matches waybar custom module requirements.

**Acceptance Scenarios**:

1. **Given** notifications are enabled, **When** waybar polls `histui status`, **Then** it receives JSON with enabled icon and appropriate tooltip
2. **Given** notifications are paused with 5 pending, **When** waybar polls `histui status`, **Then** it receives JSON showing paused state with "(5)" count
3. **Given** dunst is not running, **When** waybar polls `histui status`, **Then** it receives JSON with an error/unavailable state

---

### User Story 4 - Filtering and Sorting (Priority: P4)

As a user with many notifications, I want to filter by app name, urgency, or time range, and sort by different criteria, so I can quickly find specific notifications.

**Why this priority**: Enhances usability for power users with high notification volumes. Builds on core functionality.

**Independent Test**: Can be tested by running with filter/sort flags and verifying output matches expected criteria.

**Acceptance Scenarios**:

1. **Given** I have notifications from multiple apps, **When** I run `histui get --app-filter kitty`, **Then** only kitty notifications appear
2. **Given** I have notifications of different urgencies, **When** I run `histui get --urgency critical`, **Then** only critical notifications appear
3. **Given** I have 100 notifications, **When** I run `histui get --limit 10`, **Then** only the 10 most recent appear
4. **Given** I want oldest first, **When** I run `histui get --sort timestamp:asc`, **Then** notifications appear oldest to newest
5. **Given** I have notifications spanning a week, **When** I run `histui get --since 1h`, **Then** only notifications from the last hour appear

---

### User Story 5 - Persistent History Across Sessions (Priority: P5)

As a user, I want my notification history to persist across system reboots and histui restarts, so I can review notifications from previous sessions.

**Why this priority**: Enables long-term history browsing beyond what the notification daemon stores. Differentiates histui from simple wrapper scripts.

**Independent Test**: Can be tested by importing notifications, restarting histui, and verifying notifications are still available.

**Acceptance Scenarios**:

1. **Given** I have imported notifications from dunst, **When** I restart histui, **Then** my previously imported notifications are still visible
2. **Given** I have persistence enabled, **When** new notifications are imported, **Then** they are saved to disk automatically
3. **Given** I have 1000 notifications persisted, **When** I start histui, **Then** it loads within 1 second

---

### User Story 6 - Multiple Input Sources (Priority: P6)

As a user, I want to import notifications from different sources (dunst, piped JSON), so I can use histui with various notification daemons or custom scripts.

**Why this priority**: Foundation for future extensibility. Enables users with custom setups to use histui.

**Independent Test**: Can be tested by piping JSON to histui stdin and verifying notifications appear.

**Acceptance Scenarios**:

1. **Given** I run `dunstctl history | histui --source stdin`, **When** the import completes, **Then** notifications are added to my history store
2. **Given** I run `histui --source dunst`, **When** histui starts, **Then** it executes dunstctl and imports the history
3. **Given** I import from multiple sources, **When** I view the history, **Then** I can see which source each notification came from

---

### User Story 7 - History Maintenance (Priority: P7)

As a user, I want old notifications to be automatically filtered out and optionally pruned from storage, so my history stays manageable.

**Why this priority**: Prevents unbounded growth of history storage. Keeps UI responsive with reasonable notification counts.

**Independent Test**: Can be tested by running prune command and verifying old notifications are removed.

**Acceptance Scenarios**:

1. **Given** I have notifications older than 48 hours, **When** I run `histui get`, **Then** old notifications are filtered out by default
2. **Given** I want to see all history, **When** I run `histui get --since 0`, **Then** all notifications appear regardless of age
3. **Given** I run `histui prune --dry-run`, **When** the command completes, **Then** I see a preview of what would be removed
4. **Given** I run `histui prune --older-than 7d`, **When** the command completes, **Then** notifications older than 7 days are removed from storage

---

### Edge Cases

- What happens when dunstctl is not installed or not in PATH?
- What happens when dunstctl history returns empty results?
- What happens when dunstctl history returns malformed JSON?
- How does the tool handle notifications with very long body text (1000+ characters)?
- What happens when notification timestamps are in the future (system clock issues)?
- How does the tool behave when clipboard tools (wl-copy) are not available? (TUI mode only)
- What happens when the persistence file is corrupted?
- How does the tool handle duplicate notifications from the same source?
- What happens when disk is full and persistence fails?
- What happens when `histui get` receives a line that doesn't match any notification?
- How does `histui get` handle ambiguous matches (multiple notifications with identical display)?

## Requirements *(mandatory)*

### Functional Requirements

**Commands**:
- **FR-001**: System MUST provide a `get` subcommand for querying and outputting notifications
- **FR-002**: System MUST provide a `status` subcommand for waybar JSON output
- **FR-003**: System MUST provide a `tui` subcommand for interactive terminal UI
- **FR-004**: System MUST provide a `prune` subcommand for cleaning up old notifications
- **FR-004a**: Running `histui` with no subcommand MUST default to `tui` mode

**Input Adapters**:
- **FR-005**: System MUST support a dunst input adapter that executes `dunstctl history` and parses the JSON output
- **FR-006**: System MUST support a stdin input adapter that reads JSON from standard input
- **FR-007**: System MUST handle both legacy and current dunstctl output formats
- **FR-008**: System MUST auto-detect available notification daemon when no explicit source is specified
- **FR-009**: System MUST track the source of each notification (dunst, stdin, etc.)

**History Store**:
- **FR-010**: System MUST maintain an in-memory cache of all imported notifications
- **FR-011**: System MUST support optional disk persistence of notification history
- **FR-012**: System MUST hydrate in-memory cache from disk on startup (if persistence enabled)
- **FR-013**: System MUST persist new notifications to disk as they are imported (if persistence enabled)
- **FR-014**: System MUST store metadata with each notification: source, import timestamp, original timestamp, ULID
- **FR-015**: System MUST follow XDG spec for persistence location (`~/.local/share/histui/`)

**Get Command - Output**:
- **FR-016**: `get` command MUST support field flags: `--app`, `--title`, `--body`, `--timestamp`, `--time-relative`, `--ulid`
- **FR-017**: `get` command MUST support `--format dmenu` preset for fuzzy finder integration
- **FR-018**: `get` command MUST support `--format json` for JSON array output
- **FR-019**: `get` command MUST support Go template format strings via `--format "{{.Field}}"`
- **FR-020**: `get` command MUST display human-readable relative timestamps (e.g., "5m ago", "2h ago", "1d ago")

**Get Command - Filtering**:
- **FR-021**: `get` command MUST support `--since` flag with duration (default: 48h)
- **FR-022**: `get` command MUST support `--since 0` to disable time filtering
- **FR-023**: `get` command MUST support `--app-filter` for filtering by app name
- **FR-024**: `get` command MUST support `--urgency` for filtering by urgency level
- **FR-025**: `get` command MUST support `--limit` for limiting output count

**Get Command - Sorting**:
- **FR-026**: `get` command MUST support `--sort field:order` syntax
- **FR-027**: `get` command MUST default to `--sort timestamp:desc` (newest first)
- **FR-028**: `get` command MUST support sorting by: timestamp, app, urgency

**Get Command - Lookup**:
- **FR-029**: When `get` receives input on stdin, it MUST look up the matching notification
- **FR-030**: Lookup MUST match by ULID if present in the input line
- **FR-031**: Lookup MUST match by content (app, summary, time) if ULID not present
- **FR-032**: Lookup MUST return the most recent match if multiple notifications match

**Prune Command**:
- **FR-033**: `prune` command MUST remove notifications older than threshold (default: 48h)
- **FR-034**: `prune` command MUST support `--older-than` flag with duration
- **FR-035**: `prune` command MUST support `--keep N` to retain at most N notifications
- **FR-036**: `prune` command MUST support `--dry-run` to preview changes
- **FR-036a**: Pruning logic MUST be implemented as a reusable store utility (for use by command and future D-Bus stream ingest)

**TUI Mode**:
- **FR-037**: TUI MUST display scrollable list with app name, summary, relative time
- **FR-038**: TUI MUST support `/` for search/filter
- **FR-039**: TUI MUST support Enter to view notification detail
- **FR-040**: TUI MUST support `y` to copy notification body to clipboard
- **FR-041**: TUI MUST support `q` to quit

**Status Mode**:
- **FR-042**: `status` command MUST output waybar-compatible JSON
- **FR-043**: `status` command MUST include: text, alt, tooltip, class fields

**Display & Formatting**:
- **FR-044**: System MUST sanitize notification content to prevent display issues (strip control characters, collapse whitespace)

**Error Handling**:
- **FR-045**: System MUST handle missing or unavailable dunstctl gracefully with clear error messages
- **FR-046**: System MUST handle corrupted persistence files gracefully (backup and recreate)

**Integration**:
- **FR-047**: System MUST provide example waybar configuration for integration
- **FR-048**: System MUST provide example fuzzel/walker pipeline commands

### Key Entities

- **Notification**: A single notification entry with:
  - ULID (generated by histui, sortable)
  - App name
  - Summary
  - Body
  - Original timestamp (from daemon)
  - Import timestamp (when histui received it)
  - Urgency level (0=low, 1=normal, 2=critical)
  - Category
  - Icon path
  - URLs
  - Source identifier (dunst, stdin, dbus, etc.)
  - Extensions (daemon-specific fields)

- **HistoryStore**: Central notification repository with:
  - In-memory cache (slice/map of notifications)
  - Disk persistence layer (JSONL file)
  - Change notification channel (for reactive TUI updates)
  - Filter/sort/search operations
  - Prune utility (reusable for command and automatic ingest cleanup)

- **InputAdapter**: Interface for notification sources:
  - `Import()` method to fetch and inject notifications into store
  - Source identifier for tracking
  - Implementations: DunstAdapter, StdinAdapter

- **OutputAdapter**: Interface for presentation formats:
  - `Render(notifications)` method to produce output
  - Implementations: GetFormatter, StatusFormatter, TUIFormatter

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can view their notification history within 1 second of invoking the tool
- **SC-002**: `get --format dmenu` output is compatible with fuzzel, walker, dmenu, and rofi without modification
- **SC-003**: TUI mode supports keyboard navigation of 100+ notifications without perceptible lag
- **SC-004**: `status` command produces valid JSON that waybar custom modules consume without errors
- **SC-005**: Users can copy notification content to clipboard with a single keypress in TUI mode
- **SC-006**: `get` command completes in under 100ms with typical notification volumes (under 100 notifications)
- **SC-007**: All user-facing error messages clearly indicate the problem and suggest remediation
- **SC-008**: Persistence file loads 1000 notifications in under 1 second
- **SC-009**: Pipeline `histui get --format dmenu --ulid | fuzzel -d | histui get --body` works reliably

## Assumptions

- User has at least one supported notification daemon installed (dunst for v1.0)
- User has `dunstctl` command available in PATH (for dunst adapter)
- User has Wayland clipboard tool (`wl-copy`) available for TUI clipboard operations
- User is running Hyprland or similar Wayland compositor with waybar
- Notifications are stored in dunst's internal history (dunst configuration enables history)
- The tool will be invoked via command line, waybar custom module, or keyboard shortcut
- User has write access to XDG data directory for persistence
- JSONL is acceptable format for persistence (human-readable, append-friendly)
- Default 48-hour history window is acceptable for most users

## Integration Examples

### Waybar Configuration

```json
{
    "custom/notification": {
        "exec": "histui status",
        "return-type": "json",
        "interval": 5,
        "format": "{icon}",
        "format-icons": {
            "enabled": "󰂚",
            "paused": "󰂛",
            "paused-*": "󰂛 ({})",
            "unavailable": "󰂭"
        },
        "on-click": "histui get --format dmenu --ulid | fuzzel -d | histui get --body | wl-copy",
        "on-click-right": "dunstctl set-paused toggle"
    }
}
```

### Fuzzel Pipeline

```bash
# Basic: select notification, copy body
histui get --format dmenu --ulid | fuzzel -d | histui get --body | wl-copy

# Show full details of selected notification
histui get --format dmenu --ulid | fuzzel -d | histui get --timestamp --app --title --body

# Filter to critical only, then select
histui get --format dmenu --ulid --urgency critical | fuzzel -d | histui get --body | wl-copy
```

### Walker Integration

```bash
# Using walker instead of fuzzel
histui get --format dmenu --ulid | walker -d | histui get --body | wl-copy
```

### Dmenu/Rofi (X11)

```bash
# With dmenu
histui get --format dmenu --ulid | dmenu -l 20 | histui get --body | xclip -selection clipboard

# With rofi
histui get --format dmenu --ulid | rofi -dmenu -p "Notifications" | histui get --body | xclip -selection clipboard
```

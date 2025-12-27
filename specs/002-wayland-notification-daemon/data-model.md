# Data Model: histuid - Wayland Notification Daemon

**Date**: 2025-12-27
**Feature**: histuid - Wayland Notification Daemon

## Overview

This document defines the data structures for histuid. The daemon extends the existing histui notification model with display state and adds new models for configuration, theming, and shared state.

## Entity Relationship

```
┌─────────────────┐       ┌─────────────────┐       ┌─────────────────┐
│   Notification  │───────│   DisplayState  │       │     Config      │
│   (existing)    │ 1:1   │   (new)         │       │   (daemon)      │
└─────────────────┘       └─────────────────┘       └─────────────────┘
        │                                                   │
        │                                                   │
        ▼                                                   ▼
┌─────────────────┐                               ┌─────────────────┐
│  SharedState    │                               │     Theme       │
│  (DnD, etc)     │                               │   (CSS)         │
└─────────────────┘                               └─────────────────┘
```

---

## 1. Notification (Extended)

**Location**: `internal/model/notification.go`

The existing `Notification` struct is used as-is. histuid adds display state as a separate in-memory structure that maps to the notification by ULID.

### Existing Fields (unchanged)

```go
type Notification struct {
    // histui metadata
    HistuiID          string `json:"histui_id"`
    HistuiSource      string `json:"histui_source"`     // "histuid" for daemon-received
    HistuiImportedAt  int64  `json:"histui_imported_at"`
    HistuiSeenAt      int64  `json:"histui_seen_at,omitempty"`
    HistuiActedAt     int64  `json:"histui_acted_at,omitempty"`
    HistuiDismissedAt int64  `json:"histui_dismissed_at,omitempty"`
    ContentHash       string `json:"content_hash,omitempty"`

    // Freedesktop standard fields
    ID            int    `json:"id"`                      // D-Bus notification ID
    AppName       string `json:"app_name"`
    Summary       string `json:"summary"`
    Body          string `json:"body"`
    Timestamp     int64  `json:"timestamp"`
    ExpireTimeout int    `json:"expire_timeout,omitempty"`

    // Urgency
    Urgency     int    `json:"urgency"`
    UrgencyName string `json:"urgency_name"`

    // Optional fields
    Category string `json:"category,omitempty"`
    IconPath string `json:"icon_path,omitempty"`

    // Extensions
    Extensions *Extensions `json:"extensions,omitempty"`
}
```

### New Extensions Fields

```go
type Extensions struct {
    // Existing dunst fields...

    // D-Bus notification fields (added by histuid)
    Actions     []Action `json:"actions,omitempty"`      // Available actions
    ImageData   []byte   `json:"image_data,omitempty"`   // Raw image data (if provided)
    SoundFile   string   `json:"sound_file,omitempty"`   // Requested sound file
    SoundName   string   `json:"sound_name,omitempty"`   // Named sound from spec
    DesktopEntry string  `json:"desktop_entry,omitempty"` // .desktop file name
    Resident    bool     `json:"resident,omitempty"`     // Don't auto-remove after action
    Transient   bool     `json:"transient,omitempty"`    // Don't persist
}

type Action struct {
    Key   string `json:"key"`
    Label string `json:"label"`
}
```

---

## 2. DisplayState (New)

**Location**: `internal/daemon/display_state.go`

In-memory only. Not persisted to disk.

```go
// DisplayState tracks the visual state of a notification in histuid.
// This is separate from the Notification struct because:
// 1. It's daemon-specific (not needed by histui CLI)
// 2. It's ephemeral (reset on daemon restart)
// 3. It changes frequently (timeouts, hover state)
type DisplayState struct {
    HistuiID         string        // Links to Notification.HistuiID
    DBusID           uint32        // D-Bus notification ID (for signals)

    // Display lifecycle
    Status           PopupStatus   // pending, visible, hiding, hidden
    CreatedAt        time.Time     // When notification was received
    DisplayedAt      time.Time     // When popup became visible
    ExpiresAt        time.Time     // When popup should auto-hide (0 = never)

    // Interaction
    Hovered          bool          // Mouse is over popup (pauses timeout)
    HoverPausedAt    time.Time     // When hover started (to resume timeout)

    // Popup reference
    PopupWindow      *gtk.Window   // GTK window reference (for closing)

    // Stacking (for duplicate detection)
    StackCount       int           // Number of identical notifications stacked
    StackedIDs       []string      // ULIDs of stacked notifications
}

type PopupStatus int

const (
    PopupStatusPending PopupStatus = iota  // Queued, not yet displayed
    PopupStatusVisible                      // Currently on screen
    PopupStatusHiding                       // Fade-out animation
    PopupStatusHidden                       // Off screen
)
```

### Validation Rules

- `HistuiID` must be a valid ULID (26 characters, base32)
- `DBusID` must be > 0 (D-Bus notification IDs start at 1)
- `ExpiresAt` is 0 (zero time) for critical notifications that never expire
- `StackCount` >= 1 (a single notification has count 1)

---

## 3. SharedState (New)

**Location**: `internal/store/state.go`

Shared between histui and histuid via a state file.

```go
// SharedState contains configuration that is shared between histui and histuid.
// This is persisted to ~/.local/share/histui/state.json
type SharedState struct {
    // Do Not Disturb
    DnDEnabled        bool   `json:"dnd_enabled"`
    DnDEnabledAt      int64  `json:"dnd_enabled_at,omitempty"`  // Unix timestamp
    DnDEnabledBy      string `json:"dnd_enabled_by,omitempty"`  // "cli", "waybar", "window-rule"

    // Statistics (optional, for waybar)
    LastNotificationAt int64 `json:"last_notification_at,omitempty"`

    // Version for compatibility
    SchemaVersion     int    `json:"schema_version"` // Currently 1
}
```

### State File Location

```
~/.local/share/histui/state.json
```

### State Transitions

```
DnD State Machine:
┌─────────────┐  dnd on   ┌─────────────┐
│  Disabled   │──────────▶│   Enabled   │
│             │◀──────────│             │
└─────────────┘  dnd off  └─────────────┘
       │                         │
       │      dnd toggle         │
       └─────────────────────────┘
```

---

## 4. DaemonConfig (New)

**Location**: `internal/config/daemon.go`

Configuration for histuid daemon loaded from TOML file.

```go
// DaemonConfig is the configuration for histuid.
// Loaded from ~/.config/histui/histuid.toml
type DaemonConfig struct {
    Display  DisplayConfig  `toml:"display"`
    Timeouts TimeoutConfig  `toml:"timeouts"`
    Behavior BehaviorConfig `toml:"behavior"`
    Audio    AudioConfig    `toml:"audio"`
    Theme    ThemeConfig    `toml:"theme"`
    DnD      DnDConfig      `toml:"dnd"`
    Mouse    MouseConfig    `toml:"mouse"`
}

type DisplayConfig struct {
    Position   string `toml:"position"`    // "top-right", "top-left", etc.
    OffsetX    int    `toml:"offset_x"`
    OffsetY    int    `toml:"offset_y"`
    Width      int    `toml:"width"`
    MaxHeight  int    `toml:"max_height"`
    MaxVisible int    `toml:"max_visible"`
    Gap        int    `toml:"gap"`
    Monitor    int    `toml:"monitor"`
}

type TimeoutConfig struct {
    Low      int `toml:"low"`      // Milliseconds, 0 = never
    Normal   int `toml:"normal"`
    Critical int `toml:"critical"`
}

type BehaviorConfig struct {
    StackDuplicates bool `toml:"stack_duplicates"`
    ShowCount       bool `toml:"show_count"`
    PauseOnHover    bool `toml:"pause_on_hover"`
    HistoryLength   int  `toml:"history_length"`
}

type AudioConfig struct {
    Enabled bool        `toml:"enabled"`
    Volume  int         `toml:"volume"` // 0-100
    Sounds  SoundConfig `toml:"sounds"`
}

type SoundConfig struct {
    Low      string `toml:"low"`
    Normal   string `toml:"normal"`
    Critical string `toml:"critical"`
}

type ThemeConfig struct {
    Name string `toml:"name"` // Theme name without .css extension
}

type DnDConfig struct {
    Enabled        bool `toml:"enabled"`        // Initial state
    CriticalBypass bool `toml:"critical_bypass"`
}

type MouseConfig struct {
    Left   string `toml:"left"`   // "dismiss", "do-action", "close-all", "context-menu"
    Middle string `toml:"middle"`
    Right  string `toml:"right"`
}
```

### Default Values

```go
func DefaultDaemonConfig() *DaemonConfig {
    return &DaemonConfig{
        Display: DisplayConfig{
            Position:   "top-right",
            OffsetX:    10,
            OffsetY:    10,
            Width:      350,
            MaxHeight:  200,
            MaxVisible: 5,
            Gap:        5,
            Monitor:    0,
        },
        Timeouts: TimeoutConfig{
            Low:      5000,
            Normal:   10000,
            Critical: 0, // Never expires
        },
        Behavior: BehaviorConfig{
            StackDuplicates: true,
            ShowCount:       true,
            PauseOnHover:    true,
            HistoryLength:   100,
        },
        Audio: AudioConfig{
            Enabled: true,
            Volume:  80,
        },
        Theme: ThemeConfig{
            Name: "default",
        },
        DnD: DnDConfig{
            Enabled:        false,
            CriticalBypass: true,
        },
        Mouse: MouseConfig{
            Left:   "dismiss",
            Middle: "do-action",
            Right:  "close-all",
        },
    }
}
```

### Validation Rules

- `Position` must be one of: "top-left", "top-right", "bottom-left", "bottom-right", "top-center", "bottom-center"
- `Width` must be between 100 and 1000
- `MaxVisible` must be between 1 and 20
- `Volume` must be between 0 and 100
- `Mouse.*` must be one of: "dismiss", "do-action", "close-all", "context-menu", "none"
- Sound file paths must be absolute or use `~` for home directory

---

## 5. Theme (New)

**Location**: `internal/theme/theme.go`

```go
// Theme represents a loaded CSS theme.
type Theme struct {
    Name    string // Theme identifier (filename without .css)
    Path    string // Full path to CSS file
    CSS     string // Loaded CSS content
    ModTime time.Time // For hot-reload detection
}

// ThemeManager handles theme loading and hot-reload.
type ThemeManager struct {
    themesDir    string
    activeTheme  *Theme
    defaultTheme *Theme // Embedded default
    mu           sync.RWMutex
}
```

### Theme Directory Structure

```
~/.config/histui/themes/
├── default.css
├── catppuccin.css
├── nord.css
└── custom/
    └── my-theme.css
```

### Required CSS Classes

Themes must define these classes:

| Class | Purpose |
|-------|---------|
| `.notification` | Base notification container |
| `.notification.urgency-low` | Low urgency styling |
| `.notification.urgency-normal` | Normal urgency styling |
| `.notification.urgency-critical` | Critical urgency styling |
| `.notification .icon` | Icon container |
| `.notification .icon.symbolic` | Symbolic icon (for color filtering) |
| `.notification .summary` | Title text |
| `.notification .body` | Body text |
| `.notification .app-name` | Application name |
| `.notification .timestamp` | Relative time |
| `.notification .actions` | Action buttons container |
| `.notification .action` | Individual action button |
| `.notification .stack-count` | "(2)" for stacked notifications |

---

## 6. Filter Expression (New)

**Location**: `internal/core/filter.go`

Extension to existing filter system for rich `--filter` flag.

```go
// FilterExpr represents a parsed filter expression.
type FilterExpr struct {
    Conditions []FilterCondition
}

type FilterCondition struct {
    Field    string       // app, summary, body, urgency, time, seen, dismissed, source
    Operator FilterOp     // =, ~, >, >=, <, <=
    Value    interface{}  // string, int, time.Duration, bool
}

type FilterOp int

const (
    FilterOpEq    FilterOp = iota // =
    FilterOpRegex                  // ~ (regex match)
    FilterOpGt                     // >
    FilterOpGte                    // >=
    FilterOpLt                     // <
    FilterOpLte                    // <=
)

// ParseFilter parses a filter string like "app=slack,urgency>=normal,time<1h"
func ParseFilter(expr string) (*FilterExpr, error)

// Match checks if a notification matches the filter.
func (f *FilterExpr) Match(n *model.Notification) bool
```

### Filter Syntax Grammar

```
filter     = condition ("," condition)*
condition  = field operator value
field      = "app" | "summary" | "body" | "urgency" | "time" | "seen" | "dismissed" | "source"
operator   = "=" | "~" | ">" | ">=" | "<" | "<="
value      = string | number | duration | boolean

duration   = number unit
unit       = "m" | "h" | "d" | "w"

boolean    = "true" | "false"
```

### Examples

| Filter | Meaning |
|--------|---------|
| `app=slack` | App name is exactly "slack" |
| `app~slack\|discord` | App name matches regex "slack\|discord" |
| `urgency=critical` | Critical urgency only |
| `urgency>=normal` | Normal or critical (1 or 2) |
| `time<1h` | Received within last hour |
| `time>=1d` | Received at least 1 day ago |
| `seen=false` | Not yet seen |
| `dismissed=true` | Already dismissed |
| `source=histuid` | From histuid daemon |

---

## 7. D-Bus Notification Types

**Location**: `internal/dbus/types.go`

Types for the org.freedesktop.Notifications interface.

```go
// DBusNotification represents an incoming D-Bus Notify call.
type DBusNotification struct {
    AppName       string
    ReplacesID    uint32
    AppIcon       string
    Summary       string
    Body          string
    Actions       []string            // Alternating key, label pairs
    Hints         map[string]dbus.Variant
    ExpireTimeout int32               // -1 = server default, 0 = never
}

// ParsedActions converts the D-Bus action array to structured form.
func (n *DBusNotification) ParsedActions() []Action {
    actions := make([]Action, 0, len(n.Actions)/2)
    for i := 0; i+1 < len(n.Actions); i += 2 {
        actions = append(actions, Action{
            Key:   n.Actions[i],
            Label: n.Actions[i+1],
        })
    }
    return actions
}

// Urgency extracts the urgency hint (default: Normal).
func (n *DBusNotification) Urgency() int {
    if v, ok := n.Hints["urgency"]; ok {
        if b, ok := v.Value().(byte); ok {
            return int(b)
        }
    }
    return model.UrgencyNormal
}

// Category extracts the category hint.
func (n *DBusNotification) Category() string {
    if v, ok := n.Hints["category"]; ok {
        if s, ok := v.Value().(string); ok {
            return s
        }
    }
    return ""
}
```

### Close Reason Constants

```go
const (
    CloseReasonExpired      uint32 = 1
    CloseReasonDismissed    uint32 = 2
    CloseReasonClosed       uint32 = 3
    CloseReasonUndefined    uint32 = 4
)
```

---

## Migration Notes

### Existing histui Compatibility

- No changes to the persisted Notification format
- SharedState file is new; histui will create it if missing
- DnD state defaults to disabled if state file doesn't exist
- New Extensions fields are optional and backward-compatible

### State File Creation

When histui starts and `~/.local/share/histui/state.json` doesn't exist:

```go
defaultState := SharedState{
    DnDEnabled:    false,
    SchemaVersion: 1,
}
```

### Filter Flag Compatibility

The `--filter` flag is new. Existing filter flags (`--app-filter`, `--urgency`, `--since`) continue to work for backward compatibility but can be combined with `--filter`.

# Feature Specification: D-Bus Interface for histuid

**Feature ID**: 004-dbus-interface
**Created**: 2025-12-29
**Status**: Draft
**Input**: Consolidate socket IPC to D-Bus, add real-time notification tracking, enable histui to show active popup status

## Overview

Replace the current Unix socket IPC between histui and histuid with a comprehensive D-Bus interface. This provides real-time event notifications, eliminates socket file management, and enables richer integration between the CLI/TUI and daemon.

## Motivation

1. **Real-time updates**: D-Bus signals allow histui to react instantly to notification events without polling
2. **Standardization**: D-Bus is the standard Linux IPC mechanism, enabling third-party tool integration
3. **Discoverability**: Interface is introspectable via `busctl`, `d-feet`, etc.
4. **Simplification**: Remove socket file lifecycle management, connection handling code
5. **Active popup tracking**: Enable histui to show which notifications currently have visible popups

## D-Bus Interface Design

### Service Name
```
org.freedesktop.histui.Daemon
```

### Object Path
```
/org/freedesktop/histui/Daemon
```

### Interface: `org.freedesktop.histui.Daemon`

#### Methods

##### Core (replaces socket IPC)

| Method | Signature | Description |
|--------|-----------|-------------|
| `Ping()` | `() -> (b success)` | Check if daemon is responsive |
| `GetDnD()` | `() -> (b enabled)` | Get Do Not Disturb state |
| `SetDnD(enabled)` | `(b) -> ()` | Set Do Not Disturb state |
| `ToggleDnD()` | `() -> (b new_state)` | Toggle DnD, return new state |
| `StopAudio()` | `() -> ()` | Stop currently playing notification sound |
| `CloseNotification(id)` | `(s) -> (b success)` | Close popup by histui ID |
| `CloseAllNotifications()` | `() -> (i count)` | Close all popups, return count closed |

##### Active Popup Tracking (NEW)

| Method | Signature | Description |
|--------|-----------|-------------|
| `GetActiveNotifications()` | `() -> (as ids)` | Get histui IDs of currently displayed popups |
| `IsNotificationActive(id)` | `(s) -> (b active)` | Check if a notification has an active popup |

##### Notification Replay (NEW)

| Method | Signature | Description |
|--------|-----------|-------------|
| `ReplayNotification(id)` | `(s) -> (b success)` | Show popup for a notification again |
| `ReplayNotifications(ids)` | `(as) -> (i count)` | Replay multiple notifications |

##### History Management (NEW)

| Method | Signature | Description |
|--------|-----------|-------------|
| `GetNotification(id)` | `(s) -> (s json)` | Get full notification data as JSON |
| `GetNotificationCount()` | `() -> (i total, i unread, i active)` | Get notification counts |
| `MarkRead(id)` | `(s) -> ()` | Mark notification as read |
| `MarkAllRead()` | `() -> (i count)` | Mark all as read, return count |
| `DeleteNotification(id)` | `(s) -> (b success)` | Delete from history |
| `PruneHistory(keep)` | `(i) -> (i deleted)` | Keep N most recent, return deleted count |
| `ClearHistory()` | `() -> (i deleted)` | Clear all history |

##### Status & Configuration (NEW)

| Method | Signature | Description |
|--------|-----------|-------------|
| `GetStatus()` | `() -> (s json)` | Get daemon status (version, uptime, stats) |
| `GetConfig()` | `() -> (s json)` | Get current runtime configuration |
| `ReloadConfig()` | `() -> (b success)` | Reload configuration from disk |

##### Theme Management (NEW)

| Method | Signature | Description |
|--------|-----------|-------------|
| `GetTheme()` | `() -> (s name)` | Get current theme name |
| `SetTheme(name)` | `(s) -> (b success)` | Switch theme at runtime |
| `ListThemes()` | `() -> (as names)` | List available themes |
| `ReloadTheme()` | `() -> ()` | Reload current theme from disk |

#### Signals

| Signal | Signature | Description |
|--------|-----------|-------------|
| `NotificationReceived(id, summary, app)` | `(sss)` | New notification received |
| `NotificationDisplayed(id)` | `(s)` | Popup shown for notification |
| `NotificationDismissed(id, reason)` | `(ss)` | Popup closed (reason: timeout, user, replaced, api) |
| `NotificationClosed(id)` | `(s)` | Notification removed from history |
| `DnDChanged(enabled)` | `(b)` | Do Not Disturb state changed |
| `HistoryChanged(action, id)` | `(ss)` | History modified (action: added, deleted, pruned) |
| `ConfigChanged()` | `()` | Configuration reloaded |
| `ThemeChanged(name)` | `(s)` | Theme changed |

#### Properties (optional, for convenience)

| Property | Type | Access | Description |
|----------|------|--------|-------------|
| `DnDEnabled` | `b` | RW | Do Not Disturb state |
| `ActiveCount` | `i` | RO | Number of active popups |
| `TotalCount` | `i` | RO | Total notifications in history |
| `Version` | `s` | RO | Daemon version |
| `Theme` | `s` | RW | Current theme name |

## User Scenarios & Testing

### User Story 1 - View Active Notifications in TUI (Priority: P1)

A user opens histui TUI and wants to see which notifications currently have visible popups on screen.

**Why this priority**: Core feature request - enables users to understand notification state at a glance.

**Acceptance Scenarios**:

1. **Given** histuid is running with active popups, **When** user opens histui TUI, **Then** notifications with active popups show "A" indicator in margin
2. **Given** a popup times out or is dismissed, **When** histui receives the signal, **Then** the "A" indicator is removed in real-time
3. **Given** a new notification arrives, **When** its popup is displayed, **Then** histui shows the "A" indicator without manual refresh

---

### User Story 2 - Dismiss Notification from TUI (Priority: P1)

A user wants to dismiss a notification in histui TUI and have the corresponding popup close in histuid.

**Why this priority**: Unified control - users expect dismissing in one interface to affect the other.

**Acceptance Scenarios**:

1. **Given** a notification has an active popup, **When** user dismisses it in histui TUI, **Then** the popup closes immediately
2. **Given** histuid is not running, **When** user dismisses in histui, **Then** the notification is marked dismissed locally without error
3. **Given** multiple notifications selected, **When** user bulk dismisses, **Then** all corresponding popups close

---

### User Story 3 - Filter by Active Status (Priority: P2)

A user wants to see only notifications that currently have active popups.

**Why this priority**: Useful for managing notification overload - see what's demanding attention now.

**Acceptance Scenarios**:

1. **Given** histui CLI, **When** user runs `histui get --filter active`, **Then** only notifications with active popups are shown
2. **Given** histui TUI, **When** user applies "active" filter, **Then** list shows only active notifications
3. **Given** no active popups, **When** filter is applied, **Then** empty list is shown with appropriate message

---

### User Story 4 - Replay Notification (Priority: P2)

A user wants to see a notification popup again after it has timed out.

**Why this priority**: Common use case - missed a notification and want to see it properly.

**Acceptance Scenarios**:

1. **Given** a dismissed notification, **When** user selects "replay" in histui, **Then** the popup appears on screen
2. **Given** notification has an image, **When** replayed, **Then** image is shown correctly
3. **Given** DnD is enabled, **When** replay is requested, **Then** popup still shows (explicit user action)

---

### User Story 5 - Real-time DnD Sync (Priority: P2)

A user toggles DnD in histui CLI and the TUI (if open) reflects the change immediately.

**Why this priority**: Consistent state across interfaces without manual refresh.

**Acceptance Scenarios**:

1. **Given** histui TUI is open, **When** `histui dnd on` is run in another terminal, **Then** TUI status updates immediately
2. **Given** multiple histui instances, **When** one changes DnD, **Then** all reflect new state via signal
3. **Given** histuid restarts, **When** histui reconnects, **Then** it gets correct DnD state

---

### User Story 6 - External Tool Integration (Priority: P3)

A developer wants to build a custom notification widget that shows active notification count.

**Why this priority**: Extensibility - D-Bus enables ecosystem tools.

**Acceptance Scenarios**:

1. **Given** D-Bus interface is running, **When** tool calls `GetNotificationCount()`, **Then** it receives current counts
2. **Given** tool subscribes to signals, **When** notification events occur, **Then** tool receives real-time updates
3. **Given** tool uses `busctl` or similar, **When** it introspects interface, **Then** all methods/signals are documented

---

### User Story 7 - Runtime Theme Switching (Priority: P3)

A user wants to change the notification theme without restarting histuid.

**Why this priority**: Quality of life - faster theme iteration during customization.

**Acceptance Scenarios**:

1. **Given** histuid is running, **When** `histui theme set catppuccin` is run, **Then** next notification uses new theme
2. **Given** theme file is modified, **When** `histui theme reload` is run, **Then** changes are applied
3. **Given** invalid theme name, **When** set is attempted, **Then** error is returned, current theme unchanged

## Implementation Notes

### histuid Changes

1. Register additional D-Bus interface `org.freedesktop.histui.Daemon` alongside existing `org.freedesktop.Notifications`
2. Track active popups in a map keyed by histui ID
3. Emit signals when popup lifecycle events occur
4. Remove socket server code (`internal/ipc/server.go`)

### histui Changes

1. Replace `internal/ipc/client.go` with D-Bus client
2. Subscribe to signals for real-time updates in TUI
3. Add "A" indicator column in notification list
4. Add `--filter active` option
5. Add `replay` command/action
6. Add `theme` subcommand

### Migration Path

1. Implement D-Bus interface in histuid (keep socket IPC temporarily)
2. Update histui to use D-Bus with socket fallback
3. Deprecate socket IPC
4. Remove socket IPC in next major version

## Files to Modify/Create

### New Files
- `internal/dbus/daemon_interface.go` - D-Bus interface implementation
- `internal/dbus/signals.go` - Signal emission helpers

### Modified Files
- `internal/daemon/daemon.go` - Register D-Bus interface, track active popups
- `internal/daemon/popup.go` - Emit display/dismiss signals
- `cmd/histui/commands/theme.go` - New theme subcommand
- `internal/tui/list.go` - Add active indicator column
- `internal/tui/model.go` - Subscribe to D-Bus signals

### Removed Files (after migration)
- `internal/ipc/client.go`
- `internal/ipc/server.go`
- `internal/ipc/protocol.go`

## D-Bus Introspection XML

```xml
<!DOCTYPE node PUBLIC "-//freedesktop//DTD D-BUS Object Introspection 1.0//EN"
  "http://www.freedesktop.org/standards/dbus/1.0/introspect.dtd">
<node>
  <interface name="org.freedesktop.histui.Daemon">
    <!-- Core -->
    <method name="Ping">
      <arg name="success" type="b" direction="out"/>
    </method>
    <method name="GetDnD">
      <arg name="enabled" type="b" direction="out"/>
    </method>
    <method name="SetDnD">
      <arg name="enabled" type="b" direction="in"/>
    </method>
    <method name="ToggleDnD">
      <arg name="new_state" type="b" direction="out"/>
    </method>
    <method name="StopAudio"/>
    <method name="CloseNotification">
      <arg name="id" type="s" direction="in"/>
      <arg name="success" type="b" direction="out"/>
    </method>
    <method name="CloseAllNotifications">
      <arg name="count" type="i" direction="out"/>
    </method>

    <!-- Active Tracking -->
    <method name="GetActiveNotifications">
      <arg name="ids" type="as" direction="out"/>
    </method>
    <method name="IsNotificationActive">
      <arg name="id" type="s" direction="in"/>
      <arg name="active" type="b" direction="out"/>
    </method>

    <!-- Replay -->
    <method name="ReplayNotification">
      <arg name="id" type="s" direction="in"/>
      <arg name="success" type="b" direction="out"/>
    </method>

    <!-- History -->
    <method name="GetNotification">
      <arg name="id" type="s" direction="in"/>
      <arg name="json" type="s" direction="out"/>
    </method>
    <method name="GetNotificationCount">
      <arg name="total" type="i" direction="out"/>
      <arg name="unread" type="i" direction="out"/>
      <arg name="active" type="i" direction="out"/>
    </method>
    <method name="MarkRead">
      <arg name="id" type="s" direction="in"/>
    </method>
    <method name="MarkAllRead">
      <arg name="count" type="i" direction="out"/>
    </method>
    <method name="DeleteNotification">
      <arg name="id" type="s" direction="in"/>
      <arg name="success" type="b" direction="out"/>
    </method>
    <method name="PruneHistory">
      <arg name="keep" type="i" direction="in"/>
      <arg name="deleted" type="i" direction="out"/>
    </method>
    <method name="ClearHistory">
      <arg name="deleted" type="i" direction="out"/>
    </method>

    <!-- Status -->
    <method name="GetStatus">
      <arg name="json" type="s" direction="out"/>
    </method>
    <method name="GetConfig">
      <arg name="json" type="s" direction="out"/>
    </method>
    <method name="ReloadConfig">
      <arg name="success" type="b" direction="out"/>
    </method>

    <!-- Theme -->
    <method name="GetTheme">
      <arg name="name" type="s" direction="out"/>
    </method>
    <method name="SetTheme">
      <arg name="name" type="s" direction="in"/>
      <arg name="success" type="b" direction="out"/>
    </method>
    <method name="ListThemes">
      <arg name="names" type="as" direction="out"/>
    </method>
    <method name="ReloadTheme"/>

    <!-- Signals -->
    <signal name="NotificationReceived">
      <arg name="id" type="s"/>
      <arg name="summary" type="s"/>
      <arg name="app_name" type="s"/>
    </signal>
    <signal name="NotificationDisplayed">
      <arg name="id" type="s"/>
    </signal>
    <signal name="NotificationDismissed">
      <arg name="id" type="s"/>
      <arg name="reason" type="s"/>
    </signal>
    <signal name="NotificationClosed">
      <arg name="id" type="s"/>
    </signal>
    <signal name="DnDChanged">
      <arg name="enabled" type="b"/>
    </signal>
    <signal name="HistoryChanged">
      <arg name="action" type="s"/>
      <arg name="id" type="s"/>
    </signal>
    <signal name="ConfigChanged"/>
    <signal name="ThemeChanged">
      <arg name="name" type="s"/>
    </signal>

    <!-- Properties -->
    <property name="DnDEnabled" type="b" access="readwrite"/>
    <property name="ActiveCount" type="i" access="read"/>
    <property name="TotalCount" type="i" access="read"/>
    <property name="Version" type="s" access="read"/>
    <property name="Theme" type="s" access="readwrite"/>
  </interface>
</node>
```

## Graceful Degradation

### D-Bus Not Available

histui must work without D-Bus (containers, minimal systems, SSH sessions):

| Feature | With D-Bus | Without D-Bus |
|---------|------------|---------------|
| Read history | Full | Full (SQLite only) |
| Active indicators | Yes | No (column hidden) |
| Dismiss popup | Yes | No-op (mark dismissed locally) |
| Real-time updates | Yes | No (manual refresh) |
| DnD control | Yes | No (warning shown) |
| Replay notification | Yes | No |
| Theme switching | Yes | No |

**Implementation**: Detect D-Bus availability at startup. If unavailable, disable D-Bus-dependent features and show notice in status bar.

### Stale History Detection

histui should detect when history may be stale:

**Notification Daemon Detection**:
```go
// Check who owns org.freedesktop.Notifications
owner := dbus.GetNameOwner("org.freedesktop.Notifications")
if owner == "" {
    // No notification daemon running
    status = "No daemon"
} else {
    // Get process name via /org/freedesktop/DBus GetConnectionUnixProcessID
    pid := dbus.GetConnectionUnixProcessID(owner)
    processName := readProcessName(pid)  // /proc/{pid}/comm
    if processName != "histuid" {
        status = fmt.Sprintf("Using %s (stale)", processName)
    }
}
```

**Status Indicators** (TUI status bar / CLI status):

| State | Indicator | Meaning |
|-------|-----------|---------|
| histuid running | `[histuid]` | Live - receiving notifications |
| histuid + monitor | `[histuid+mon]` | Live - also forwarding to another daemon |
| Other daemon | `[dunst] STALE` | History not updating |
| No daemon | `[--] STALE` | No notification daemon running |
| D-Bus unavailable | `[offline]` | D-Bus not available |

**histui status output**:
```
Notification Daemon: histuid (pid 12345)
D-Bus Interface:     org.freedesktop.histui.Daemon available
History:             Live (1,234 notifications)
DnD:                 Off
Active Popups:       2
```

vs stale:
```
Notification Daemon: dunst (pid 12345)
                     WARNING: histuid is not the active daemon
                     History may be stale - run histuid or use monitor mode
History:             Stale (1,234 notifications, last update 2h ago)
```

### Monitor Mode Awareness

When histuid runs in monitor mode (`histuid --monitor`), it forwards to another daemon but still captures history. Detection:

1. Check if `org.freedesktop.histui.Daemon` is available (histuid running)
2. Check if histuid's config has `monitor_mode = true`
3. Status: `[histuid+dunst]` - histuid capturing, dunst displaying

## Open Questions

1. Should we support D-Bus activation (auto-start histuid when called)?
2. Should GetNotification return full binary data (images) or just metadata?
3. Should we add batch operations for all single-item methods?
4. Should theme changes apply to existing popups or only new ones?
5. Should stale history show "last notification received" timestamp?
6. Should histui offer to start histuid if not running?

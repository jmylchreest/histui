# Data Model: histui - Notification History Browser

**Date**: 2025-12-26
**Feature**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md)
**Schema Reference**: [notification-schema.md](../../.specify/memory/notification-schema.md)

## Entity Relationship Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                      HistoryStore                            │
├─────────────────────────────────────────────────────────────┤
│ • notifications: []Notification                              │
│ • persistence: PersistenceLayer                             │
│ • changes: chan<- ChangeEvent                               │
├─────────────────────────────────────────────────────────────┤
│ + Add(n Notification)                                        │
│ + AddBatch(ns []Notification)                               │
│ + All() []Notification                                       │
│ + Filter(opts FilterOptions) []Notification                  │
│ + Lookup(input string) *Notification                        │
│ + Prune(opts PruneOptions) int                              │
│ + Subscribe() <-chan ChangeEvent                            │
│ + Close()                                                    │
└─────────────────────────────────────────────────────────────┘
                          │
                          │ stores
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                      Notification                            │
├─────────────────────────────────────────────────────────────┤
│ histui_id: ULID              │ Primary key (sortable)       │
│ histui_source: string        │ "dunst", "stdin", "dbus"     │
│ histui_imported_at: int64    │ Unix timestamp               │
│ id: int                      │ Original daemon ID           │
│ app_name: string             │ Application name             │
│ summary: string              │ Notification title           │
│ body: string                 │ Notification content         │
│ timestamp: int64             │ Unix timestamp               │
│ urgency: int                 │ 0=low, 1=normal, 2=critical  │
│ urgency_name: string         │ Human-readable urgency       │
│ category: string             │ Freedesktop category         │
│ icon_path: string            │ Path to icon file            │
│ extensions: Extensions       │ Daemon-specific data         │
└─────────────────────────────────────────────────────────────┘
                          │
                          │ contains
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                      Extensions                              │
├─────────────────────────────────────────────────────────────┤
│ stack_tag: string            │ x-dunst-stack-tag            │
│ progress: int                │ Progress bar 0-100, -1=none  │
│ message: string              │ Dunst formatted message      │
│ urls: string                 │ Extracted URLs               │
│ foreground: string           │ Text color                   │
│ background: string           │ Background color             │
└─────────────────────────────────────────────────────────────┘
```

---

## Core Entities

### Notification

The primary data entity representing a single notification from any source.

```go
package model

import (
    "time"
    "github.com/oklog/ulid/v2"
)

// Notification represents a single notification entry.
// This is the normalized format stored in the history and used by all adapters.
type Notification struct {
    // histui metadata (added by histui)
    HistuiID         ulid.ULID `json:"histui_id"`
    HistuiSource     string    `json:"histui_source"`
    HistuiImportedAt int64     `json:"histui_imported_at"`

    // Freedesktop standard fields
    ID            int    `json:"id"`
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

    // Daemon-specific extensions
    Extensions *Extensions `json:"extensions,omitempty"`
}

// Extensions holds daemon-specific fields that don't map to freedesktop spec.
type Extensions struct {
    // Dunst-specific
    StackTag   string `json:"stack_tag,omitempty"`
    Progress   int    `json:"progress,omitempty"`
    Message    string `json:"message,omitempty"`
    URLs       string `json:"urls,omitempty"`
    Foreground string `json:"foreground,omitempty"`
    Background string `json:"background,omitempty"`
}
```

#### Field Descriptions

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `histui_id` | ULID | Yes | Unique, sortable identifier generated by histui |
| `histui_source` | string | Yes | Adapter that provided this notification |
| `histui_imported_at` | int64 | Yes | Unix timestamp when histui received this |
| `id` | int | Yes | Original notification ID from daemon |
| `app_name` | string | Yes | Name of the sending application |
| `summary` | string | Yes | Brief notification title |
| `body` | string | No | Detailed notification content |
| `timestamp` | int64 | Yes | Unix timestamp of notification |
| `expire_timeout` | int | No | Timeout in milliseconds |
| `urgency` | int | Yes | 0=low, 1=normal, 2=critical |
| `urgency_name` | string | Yes | Human-readable urgency level |
| `category` | string | No | Freedesktop category |
| `icon_path` | string | No | Path to notification icon |
| `extensions` | Extensions | No | Daemon-specific data |

#### Validation Rules

1. `histui_id` must be a valid ULID
2. `histui_source` must be non-empty ("dunst", "stdin", "dbus")
3. `app_name` must be non-empty
4. `summary` must be non-empty
5. `urgency` must be 0, 1, or 2
6. `timestamp` must be > 0

#### Urgency Mapping

| Value | Name | Description |
|-------|------|-------------|
| 0 | low | Background information |
| 1 | normal | Standard notifications |
| 2 | critical | Requires attention |

---

### HistoryStore

Central repository for all notifications with in-memory cache and optional persistence.

```go
package store

import (
    "sync"
    "histui/internal/model"
)

// Store manages the notification history with thread-safe operations.
type Store struct {
    mu            sync.RWMutex
    notifications []model.Notification
    index         map[string]int // histui_id -> slice index

    persistence   Persistence
    persistPath   string
    persistEnable bool

    changes    chan ChangeEvent
    subscribers []chan<- ChangeEvent
}

// ChangeEvent signals that the store contents have changed.
type ChangeEvent struct {
    Type   ChangeType
    Count  int
    Source string
}

type ChangeType int

const (
    ChangeTypeAdd ChangeType = iota
    ChangeTypeClear
)

// FilterOptions specifies criteria for filtering notifications.
type FilterOptions struct {
    Since      time.Duration // Filter to notifications newer than now-since (0=all)
    AppFilter  string        // Exact match on app name
    Urgency    *int          // Filter by urgency level (nil=any)
    Limit      int           // Maximum results (0=unlimited)
    SortField  string        // Field to sort by: "timestamp", "app", "urgency"
    SortOrder  string        // "asc" or "desc" (default: "desc")
}

// PruneOptions configures the prune operation.
type PruneOptions struct {
    OlderThan time.Duration // Remove notifications older than this (default: 48h)
    Keep      int           // Keep at most N notifications (0=unlimited)
    DryRun    bool          // If true, return count without removing
}
```

#### Store Operations

| Operation | Description |
|-----------|-------------|
| `Add(n Notification)` | Add single notification, persist if enabled, notify subscribers |
| `AddBatch(ns []Notification)` | Add multiple notifications efficiently |
| `All() []Notification` | Return all notifications (newest first by default) |
| `Filter(opts FilterOptions)` | Return filtered/sorted subset |
| `Lookup(input string) *Notification` | Find notification by ULID or content match |
| `Prune(opts PruneOptions) int` | Remove old notifications, return count removed |
| `Count() int` | Return total notification count |
| `Subscribe() <-chan ChangeEvent` | Subscribe to change notifications |
| `Unsubscribe(ch <-chan ChangeEvent)` | Remove subscription |
| `Close()` | Cleanup resources, close channels |

#### State Diagram

```
                    ┌─────────────┐
                    │   Empty     │
                    └──────┬──────┘
                           │ Hydrate from disk
                           ▼
                    ┌─────────────┐
        Add/AddBatch│   Ready     │◄──────────┐
              ┌─────┤             │           │
              │     └──────┬──────┘           │
              │            │                  │
              ▼            │ Close            │
    ┌─────────────────┐    │                  │
    │ Notify          │    │                  │
    │ subscribers     │────┼──────────────────┘
    │ + persist       │    │
    └─────────────────┘    │
                           ▼
                    ┌─────────────┐
                    │   Closed    │
                    └─────────────┘
```

---

### PersistenceLayer

Handles JSONL file operations for durable storage.

```go
package store

import (
    "histui/internal/model"
)

// Persistence defines the interface for history storage.
type Persistence interface {
    // Load reads all notifications from storage.
    Load() ([]model.Notification, error)

    // Append adds a notification to storage.
    Append(n model.Notification) error

    // AppendBatch adds multiple notifications efficiently.
    AppendBatch(ns []model.Notification) error

    // Clear removes all stored notifications.
    Clear() error

    // Close releases resources.
    Close() error
}

// JSONLPersistence implements Persistence using JSONL files.
type JSONLPersistence struct {
    path string
    file *os.File
}
```

#### File Format

```jsonl
{"histui_schema_version":1,"created_at":1703577600}
{"histui_id":"01HQGXK5P0000000000000000","histui_source":"dunst","histui_imported_at":1703577600,"id":123,"app_name":"firefox","summary":"Download Complete","body":"myfile.zip","timestamp":1703577500,"urgency":1,"urgency_name":"normal"}
{"histui_id":"01HQGXK5P1000000000000001","histui_source":"dunst","histui_imported_at":1703577601,"id":124,"app_name":"slack","summary":"New Message","body":"Hello","timestamp":1703577501,"urgency":1,"urgency_name":"normal"}
```

#### File Location

```
~/.local/share/histui/history.jsonl
```

Follows XDG Base Directory specification. Falls back to `$HOME/.local/share` if `XDG_DATA_HOME` is not set.

---

## Adapter Interfaces

### InputAdapter

Interface for notification sources.

```go
package adapter

import (
    "context"
    "histui/internal/model"
)

// InputAdapter fetches notifications from a source.
type InputAdapter interface {
    // Name returns the adapter identifier.
    Name() string

    // Import fetches notifications and returns them.
    // This is a one-shot operation for import adapters.
    Import(ctx context.Context) ([]model.Notification, error)
}
```

#### Implementations

| Adapter | Source | Import Type |
|---------|--------|-------------|
| `DunstAdapter` | `dunstctl history` | One-shot |
| `StdinAdapter` | Standard input JSON | One-shot |
| `DBusAdapter` | D-Bus monitor (future) | Stream |

### OutputAdapter

Interface for presentation formats.

```go
package adapter

import (
    "io"
    "histui/internal/model"
)

// OutputAdapter formats notifications for display.
type OutputAdapter interface {
    // Name returns the adapter identifier.
    Name() string

    // Render formats notifications and writes to output.
    Render(w io.Writer, notifications []model.Notification) error
}

// InteractiveAdapter extends OutputAdapter for interactive modes.
type InteractiveAdapter interface {
    OutputAdapter

    // Run starts the interactive session.
    Run(store *store.Store) error
}
```

#### Implementations

| Adapter | Output | Type |
|---------|--------|------|
| `ListFormatter` | One-line-per-notification | OutputAdapter |
| `JSONFormatter` | Raw JSON array | OutputAdapter |
| `StatusFormatter` | Waybar-compatible JSON | OutputAdapter |
| `TUIFormatter` | Interactive BubbleTea | InteractiveAdapter |

---

## Output Formats

### List Mode Output

```
firefox | Download Complete - myfile.zip | 5m ago
slack | New Message - Hello from John | 2h ago
kitty | Build Complete - Success | 1d ago
```

Format: `{app_name} | {summary} - {body_truncated} | {relative_time}`

### Status Mode Output (Waybar)

```json
{"text":"","alt":"enabled","tooltip":"Notifications enabled\n5 in history","class":"enabled"}
```

### JSON Mode Output

```json
[
  {
    "histui_id": "01HQGXK5P0000000000000000",
    "histui_source": "dunst",
    "app_name": "firefox",
    "summary": "Download Complete",
    "body": "myfile.zip",
    "timestamp": 1703577500,
    "urgency": 1,
    "urgency_name": "normal"
  }
]
```

---

## Index Strategy

### In-Memory Index

```go
// Primary index: ULID -> notification
index map[ulid.ULID]*Notification

// For filtering
appIndex map[string][]ulid.ULID  // app_name -> notification IDs
```

### Query Patterns

| Query | Index Used | Complexity |
|-------|------------|------------|
| Get by ID | Primary | O(1) |
| Filter by app | appIndex | O(n) where n = matching |
| Filter by urgency | Full scan | O(total) |
| Latest N | Sorted slice | O(N) |
| Time range | Binary search on sorted | O(log n + k) |

---

## Deduplication

Notifications are deduplicated based on:
1. Same `app_name`
2. Same `summary`
3. Same `body`
4. Timestamp within 1 second window

```go
func (n *Notification) DedupeKey() string {
    return fmt.Sprintf("%s:%s:%s:%d",
        n.AppName,
        n.Summary,
        n.Body,
        n.Timestamp/1, // Round to second
    )
}
```

---

## Memory Budget

Estimated memory usage per notification:
- Base struct: ~200 bytes
- Average strings: ~300 bytes
- Extensions: ~100 bytes
- **Total: ~600 bytes per notification**

For 1,000 notifications: ~600 KB in-memory

---

## Schema Evolution

The JSONL file includes a schema version header:

```json
{"histui_schema_version":1,"created_at":1703577600}
```

Version migration:
- **v1**: Initial schema (this document)
- **v2+**: Add fields as needed, old fields remain for backward compatibility

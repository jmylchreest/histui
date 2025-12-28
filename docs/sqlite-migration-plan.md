# SQLite Migration Plan

## Overview

Migrate from JSONL + file-based storage to SQLite using `modernc.org/sqlite` (pure Go, no CGO).

**Goals:**
- Eliminate file handle synchronization issues between daemon and CLI
- Simplify codebase by removing in-memory store
- Remove tombstones (SQLite DELETE is sufficient)
- Remove complex file watchers for store changes
- Maintain or improve query performance

## Database Location

```
~/.local/share/histui/histui.db
```

Single file replaces:
- `history.jsonl` (notifications)
- `state.json` (DnD, audio control)
- `tombstones.json` (removed entirely)

**Keep as files:**
- `~/.config/histui/config.toml` (user config)
- `~/.config/histui/histuid.toml` (daemon config)
- Images (see design decision below)

## Schema Design

```sql
-- Enable WAL mode for concurrent access
PRAGMA journal_mode = WAL;
PRAGMA busy_timeout = 5000;
PRAGMA foreign_keys = ON;

-- Schema version tracking
CREATE TABLE schema_info (
    version INTEGER PRIMARY KEY,
    migrated_at INTEGER NOT NULL
);

-- Main notifications table
CREATE TABLE notifications (
    histui_id TEXT PRIMARY KEY,           -- ULID (26 chars)

    -- D-Bus fields
    dbus_id INTEGER,
    app_name TEXT NOT NULL,
    summary TEXT NOT NULL,
    body TEXT DEFAULT '',
    icon_path TEXT DEFAULT '',
    urgency INTEGER DEFAULT 1,            -- 0=low, 1=normal, 2=critical
    category TEXT DEFAULT '',

    -- Timestamps (Unix seconds)
    timestamp INTEGER NOT NULL,           -- Original notification time
    expire_timeout INTEGER DEFAULT 0,

    -- histui metadata
    source TEXT DEFAULT 'histuid',        -- histuid, histuid-monitor, dunst
    imported_at INTEGER NOT NULL,
    seen_at INTEGER DEFAULT 0,            -- 0 = not seen
    acted_at INTEGER DEFAULT 0,           -- 0 = no action taken
    dismissed_at INTEGER DEFAULT 0,       -- 0 = not dismissed
    replayed INTEGER DEFAULT 0,           -- boolean
    replayed_at INTEGER DEFAULT 0,

    -- Deduplication
    content_hash TEXT UNIQUE,             -- SHA256 for dedup

    -- Complex fields as JSON
    extensions TEXT,                      -- JSON: actions, progress, etc.
    original_hints TEXT                   -- JSON: raw D-Bus hints for replay
);

-- Indexes for common queries
CREATE INDEX idx_notifications_timestamp ON notifications(timestamp DESC);
CREATE INDEX idx_notifications_app ON notifications(app_name);
CREATE INDEX idx_notifications_urgency ON notifications(urgency);
CREATE INDEX idx_notifications_dismissed ON notifications(dismissed_at);
CREATE INDEX idx_notifications_seen ON notifications(seen_at);

-- Compound index for status queries (active = not dismissed)
CREATE INDEX idx_notifications_active ON notifications(dismissed_at, seen_at, urgency);

-- Images stored as BLOBs (simpler than filesystem)
CREATE TABLE notification_images (
    histui_id TEXT NOT NULL,
    ref_type TEXT NOT NULL,               -- 'icon' or 'image'
    data BLOB NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (histui_id, ref_type),
    FOREIGN KEY (histui_id) REFERENCES notifications(histui_id) ON DELETE CASCADE
);

-- Shared state (DnD, audio control, etc.)
CREATE TABLE shared_state (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,                  -- JSON for complex values
    updated_at INTEGER NOT NULL
);

-- Initial state entries
INSERT INTO shared_state (key, value, updated_at) VALUES
    ('dnd_enabled', 'false', 0),
    ('dnd_transition', '{}', 0),
    ('last_notification_at', '0', 0),
    ('stop_audio_requested_at', '0', 0);
```

## Design Decisions

### 1. Images as BLOBs vs Filesystem

**Decision: Store as BLOBs**

Pros:
- Cascade delete works automatically (FK constraint)
- Single backup (just copy .db file)
- No orphan cleanup needed
- Atomic with notification inserts

Cons:
- Slightly larger DB file
- Can't browse images directly in filesystem

For typical notification icons (32x32 PNG ~2KB), this is negligible.

### 2. No In-Memory Store

**Decision: Query SQLite directly**

The current architecture:
```
JSONL → Load → In-Memory Store → Query
                    ↓
              File Watcher → Reload
```

New architecture:
```
SQLite → Query directly
```

Benefits:
- No memory bloat with thousands of notifications
- No sync issues between memory and disk
- No file watcher complexity
- Both daemon and CLI see same data instantly

### 3. Remove Tombstones

**Decision: Just DELETE rows**

Tombstones existed to prevent re-import of deleted notifications during dunst import. With SQLite:
- UNIQUE constraint on `content_hash` prevents duplicates
- INSERT OR IGNORE handles conflicts cleanly
- If user deletes a notification, it's gone (expected behavior)

### 4. Shared State via Table

**Decision: Key-value table instead of JSON file**

Benefits:
- Atomic updates
- No file watching needed for DnD changes
- CLI and daemon share same connection
- Can poll/query instead of watch

For DnD changes, daemon can poll every second or use a simple notification mechanism.

## New Package Structure

```
internal/
├── db/
│   ├── db.go              # Database connection, migrations
│   ├── notifications.go   # Notification CRUD operations
│   ├── images.go          # Image storage operations
│   ├── state.go           # Shared state operations
│   └── queries.go         # Complex queries (counts, filters)
├── store/
│   └── (removed or minimal wrapper)
```

## API Design

### db.go - Core Database

```go
package db

import (
    "database/sql"
    _ "modernc.org/sqlite"
)

type DB struct {
    conn *sql.DB
    path string
}

// Open opens or creates the database with WAL mode
func Open(path string) (*DB, error)

// Close closes the database connection
func (d *DB) Close() error

// Migrate runs schema migrations
func (d *DB) Migrate() error
```

### notifications.go - Notification Operations

```go
// Add inserts a notification (ignores if content_hash exists)
func (d *DB) AddNotification(n *model.Notification) error

// AddBatch inserts multiple notifications efficiently
func (d *DB) AddBatch(ns []model.Notification) error

// Get retrieves a notification by ID
func (d *DB) GetNotification(histuiID string) (*model.Notification, error)

// Update updates a notification
func (d *DB) UpdateNotification(n *model.Notification) error

// Delete removes a notification (cascade deletes images)
func (d *DB) DeleteNotification(histuiID string) error

// Dismiss marks notification as dismissed
func (d *DB) DismissNotification(histuiID string) error

// MarkSeen marks notification as seen
func (d *DB) MarkSeen(histuiID string) error

// Filter returns notifications matching criteria
func (d *DB) Filter(opts FilterOptions) ([]model.Notification, error)

// All returns all notifications (newest first)
func (d *DB) All() ([]model.Notification, error)

// Count returns total notification count
func (d *DB) Count() (int, error)

// Prune removes oldest notifications beyond limit
func (d *DB) Prune(keepCount int) (int, error)

// Clear removes all notifications
func (d *DB) Clear() error
```

### queries.go - Status Queries

```go
// Counts for waybar status
type Counts struct {
    Displayed int  // seen but not dismissed
    History   int  // dismissed
    Waiting   int  // not seen, not dismissed
}

func (d *DB) GetCounts() (*Counts, error)

// DetailedCounts for --detailed status
type DetailedCounts struct {
    Pending         int
    PendingCritical int
    PendingNormal   int
    PendingLow      int
    Missed          int
    MissedCritical  int
    MissedNormal    int
    MissedLow       int
    Dismissed       int
    Total           int
    TopApps         []AppCount
}

func (d *DB) GetDetailedCounts() (*DetailedCounts, error)

// Efficient single query for all counts
const countQuery = `
SELECT
    SUM(CASE WHEN seen_at > 0 AND dismissed_at = 0 THEN 1 ELSE 0 END) as displayed,
    SUM(CASE WHEN dismissed_at > 0 THEN 1 ELSE 0 END) as history,
    SUM(CASE WHEN seen_at = 0 AND dismissed_at = 0 THEN 1 ELSE 0 END) as waiting
FROM notifications
`
```

### state.go - Shared State

```go
// GetDnDEnabled returns current DnD state
func (d *DB) GetDnDEnabled() (bool, error)

// SetDnDEnabled updates DnD state
func (d *DB) SetDnDEnabled(enabled bool, transition *DnDTransition) error

// GetLastNotificationAt returns timestamp of last notification
func (d *DB) GetLastNotificationAt() (int64, error)

// SetStopAudioRequested signals daemon to stop audio
func (d *DB) SetStopAudioRequested() error

// GetStopAudioRequestedAt returns audio stop signal timestamp
func (d *DB) GetStopAudioRequestedAt() (int64, error)
```

### images.go - Image Storage

```go
// SaveImage stores image data for a notification
func (d *DB) SaveImage(histuiID string, refType string, data []byte) error

// LoadImage retrieves image data
func (d *DB) LoadImage(histuiID string, refType string) ([]byte, error)

// LoadAllImages retrieves all images for a notification
func (d *DB) LoadAllImages(histuiID string) (map[string][]byte, error)

// Images are automatically deleted via CASCADE when notification is deleted
```

## Migration Strategy

### Phase 1: Add SQLite (Parallel)

1. Add `modernc.org/sqlite` dependency
2. Create `internal/db` package
3. Implement schema and migrations
4. Add JSONL → SQLite migration tool

### Phase 2: Dual-Write

1. Modify Store to write to both JSONL and SQLite
2. Verify data consistency
3. Test all queries against SQLite

### Phase 3: Switch Read Path

1. Change all reads to use SQLite
2. Keep JSONL writes for rollback safety
3. Test extensively

### Phase 4: Remove JSONL

1. Remove JSONL persistence code
2. Remove file watchers (StoreWatcher)
3. Remove in-memory store
4. Remove tombstones
5. Simplify Store interface to thin wrapper around DB

### Phase 5: Cleanup

1. Remove `internal/store/persistence.go`
2. Remove `internal/store/tombstones.go`
3. Simplify `internal/store/store.go` or remove entirely
4. Update tests

## Files to Modify/Remove

### Remove Entirely
- `internal/store/persistence.go` - JSONL implementation
- `internal/store/tombstones.go` - No longer needed
- `internal/store/watcher.go` - File watcher not needed
- `internal/daemon/hotreload.go` (StoreWatcher parts)

### Simplify
- `internal/store/store.go` - Thin wrapper or remove
- `internal/store/images.go` - Move to db package
- `internal/store/state.go` - Move to db package
- `internal/store/retention.go` - Simplify, just call db.Prune()

### Keep (Config Watchers)
- ConfigWatcher in `hotreload.go` - Still needed for theme/display hot-reload

### New Files
- `internal/db/db.go`
- `internal/db/notifications.go`
- `internal/db/images.go`
- `internal/db/state.go`
- `internal/db/queries.go`
- `internal/db/migrate.go`

## Daemon Changes

### Current Flow
```
1. Load JSONL into memory
2. Start file watcher
3. On notification: Add to memory + append to JSONL
4. On file change: ReloadIfNeeded()
5. Query: Read from memory
```

### New Flow
```
1. Open SQLite connection (WAL mode)
2. On notification: INSERT into SQLite
3. Query: SELECT from SQLite
4. No file watcher needed
```

### DnD Polling

Instead of file watching for DnD changes:
```go
// In daemon main loop, poll every second
ticker := time.NewTicker(1 * time.Second)
go func() {
    var lastDnD bool
    for range ticker.C {
        enabled, _ := db.GetDnDEnabled()
        if enabled != lastDnD {
            // Handle DnD state change
            lastDnD = enabled
        }
    }
}()
```

Or use SQLite's `PRAGMA data_version` to detect any changes.

## Testing Plan

1. **Unit tests** for all db package functions
2. **Migration test** - Import existing JSONL, verify all data
3. **Concurrency test** - Daemon + CLI simultaneous access
4. **Performance test** - 10K notifications, measure query times
5. **Integration test** - Full daemon + CLI workflow

## Rollback Plan

Keep JSONL export capability:
```go
func (d *DB) ExportJSONL(w io.Writer) error
```

If issues arise, can export back to JSONL and revert code.

## Timeline Estimate

- Phase 1 (SQLite core): ~2-3 hours
- Phase 2-3 (Dual write/read): ~2 hours
- Phase 4-5 (Cleanup): ~2-3 hours
- Testing: ~2 hours

Total: ~8-10 hours of work

## Open Questions

1. **State polling interval** - 1 second okay for DnD responsiveness?
2. **Image size limit** - Should we limit BLOB size? (Most icons are tiny)
3. **Vacuum schedule** - Auto-vacuum or manual after prune?
4. **Backup strategy** - Just copy .db file? SQLite online backup API?

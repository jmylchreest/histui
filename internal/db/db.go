// Package db provides SQLite-based storage for histui notifications.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// SchemaVersion is the current database schema version.
const SchemaVersion = 1

// DB wraps a SQLite database connection.
type DB struct {
	conn *sql.DB
	path string
	mu   sync.RWMutex
}

// Open opens or creates the database at the given path.
// If path is empty, uses the default location (~/.local/share/histui/histui.db).
func Open(path string) (*DB, error) {
	if path == "" {
		dataDir, err := defaultDataDir()
		if err != nil {
			return nil, fmt.Errorf("get data dir: %w", err)
		}
		path = filepath.Join(dataDir, "histui.db")
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	// Open with WAL mode and other pragmas via query string
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON&_synchronous=NORMAL", path)

	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Verify connection
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	db := &DB{
		conn: conn,
		path: path,
	}

	// Run migrations
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

// Close closes the database connection.
func (d *DB) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.conn != nil {
		// Checkpoint WAL before closing
		_, _ = d.conn.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
		return d.conn.Close()
	}
	return nil
}

// Path returns the database file path.
func (d *DB) Path() string {
	return d.path
}

// DataDir returns the directory containing the database file.
func (d *DB) DataDir() string {
	return filepath.Dir(d.path)
}

// LastUpdatedPath returns the path to the .lastupdated file.
// This file is touched by histuid when notifications change.
func LastUpdatedPath() (string, error) {
	dataDir, err := defaultDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, ".lastupdated"), nil
}

// TouchLastUpdated writes the current unix timestamp to the .lastupdated file.
// This is called by histuid to signal histui that notifications have changed.
func TouchLastUpdated() error {
	path, err := LastUpdatedPath()
	if err != nil {
		return err
	}

	// Write current unix timestamp
	timestamp := fmt.Sprintf("%d", unixNow())
	return os.WriteFile(path, []byte(timestamp), 0644)
}

// unixNow returns the current unix timestamp.
// Extracted for testing.
var unixNow = func() int64 {
	return timeNow().Unix()
}

// timeNow returns the current time. Can be overridden in tests.
var timeNow = func() time.Time {
	return time.Now()
}

// defaultDataDir returns the default data directory.
func defaultDataDir() (string, error) {
	// Use XDG_DATA_HOME if set, otherwise ~/.local/share
	if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
		return filepath.Join(dataHome, "histui"), nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".local", "share", "histui"), nil
}

// migrate runs database migrations.
func (d *DB) migrate() error {
	// Check current schema version
	var version int
	err := d.conn.QueryRow("SELECT version FROM schema_info ORDER BY version DESC LIMIT 1").Scan(&version)
	if err != nil && err != sql.ErrNoRows {
		// Table might not exist yet
		version = 0
	}

	if version >= SchemaVersion {
		return nil // Already up to date
	}

	// Run migrations in a transaction
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if version < 1 {
		if err := d.migrateV1(tx); err != nil {
			return fmt.Errorf("migrate v1: %w", err)
		}
	}

	return tx.Commit()
}

// migrateV1 creates the initial schema.
func (d *DB) migrateV1(tx *sql.Tx) error {
	schema := `
-- Schema version tracking
CREATE TABLE IF NOT EXISTS schema_info (
    version INTEGER PRIMARY KEY,
    migrated_at INTEGER NOT NULL
);

-- Main notifications table
CREATE TABLE IF NOT EXISTS notifications (
    histui_id TEXT PRIMARY KEY,

    -- D-Bus fields
    dbus_id INTEGER DEFAULT 0,
    app_name TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    body TEXT DEFAULT '',
    icon_path TEXT DEFAULT '',
    urgency INTEGER DEFAULT 1,
    category TEXT DEFAULT '',

    -- Timestamps (Unix seconds)
    timestamp INTEGER NOT NULL,
    expire_timeout INTEGER DEFAULT 0,

    -- histui metadata
    source TEXT DEFAULT 'histuid',
    imported_at INTEGER NOT NULL,
    seen_at INTEGER DEFAULT 0,
    acted_at INTEGER DEFAULT 0,
    dismissed_at INTEGER DEFAULT 0,
    replayed INTEGER DEFAULT 0,
    replayed_at INTEGER DEFAULT 0,

    -- Deduplication
    content_hash TEXT UNIQUE,

    -- Complex fields as JSON
    extensions TEXT DEFAULT '{}',
    original_hints TEXT DEFAULT '{}'
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_notifications_timestamp ON notifications(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_app ON notifications(app_name);
CREATE INDEX IF NOT EXISTS idx_notifications_urgency ON notifications(urgency);
CREATE INDEX IF NOT EXISTS idx_notifications_dismissed ON notifications(dismissed_at);
CREATE INDEX IF NOT EXISTS idx_notifications_seen ON notifications(seen_at);
CREATE INDEX IF NOT EXISTS idx_notifications_active ON notifications(dismissed_at, seen_at, urgency);

-- Images stored as BLOBs
CREATE TABLE IF NOT EXISTS notification_images (
    histui_id TEXT NOT NULL,
    ref_type TEXT NOT NULL,
    data BLOB NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (histui_id, ref_type),
    FOREIGN KEY (histui_id) REFERENCES notifications(histui_id) ON DELETE CASCADE
);

-- Record migration
INSERT INTO schema_info (version, migrated_at) VALUES (1, strftime('%s', 'now'));
`

	_, err := tx.Exec(schema)
	return err
}

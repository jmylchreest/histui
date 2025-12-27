// Package contracts defines the interfaces for histui.
// This file serves as documentation and is not compiled.
// Actual implementations live in internal/ packages.
package contracts

import (
	"context"
	"io"
	"time"
)

// =============================================================================
// Model Types
// =============================================================================

// Notification represents a single notification entry.
// See data-model.md for full field descriptions.
type Notification struct {
	// histui metadata
	HistuiID         string `json:"histui_id"`          // ULID string
	HistuiSource     string `json:"histui_source"`      // "dunst", "stdin", "dbus"
	HistuiImportedAt int64  `json:"histui_imported_at"` // Unix timestamp

	// Freedesktop fields
	ID            int    `json:"id"`
	AppName       string `json:"app_name"`
	Summary       string `json:"summary"`
	Body          string `json:"body"`
	Timestamp     int64  `json:"timestamp"`
	ExpireTimeout int    `json:"expire_timeout,omitempty"`

	// Urgency
	Urgency     int    `json:"urgency"`      // 0=low, 1=normal, 2=critical
	UrgencyName string `json:"urgency_name"` // "low", "normal", "critical"

	// Optional
	Category   string      `json:"category,omitempty"`
	IconPath   string      `json:"icon_path,omitempty"`
	Extensions *Extensions `json:"extensions,omitempty"`
}

// Extensions holds daemon-specific fields.
type Extensions struct {
	StackTag   string `json:"stack_tag,omitempty"`
	Progress   int    `json:"progress,omitempty"` // 0-100, -1=none
	Message    string `json:"message,omitempty"`
	URLs       string `json:"urls,omitempty"`
	Foreground string `json:"foreground,omitempty"`
	Background string `json:"background,omitempty"`
}

// FilterOptions specifies criteria for filtering notifications.
type FilterOptions struct {
	Since     time.Duration // Filter to notifications newer than now-since (0=all)
	AppFilter string        // Exact match on app name
	Urgency   *int          // Filter by urgency level (nil=any)
	Limit     int           // Maximum results (0=unlimited)
	SortField string        // Field to sort by: "timestamp", "app", "urgency"
	SortOrder string        // "asc" or "desc" (default: "desc")
}

// PruneOptions configures the prune operation.
type PruneOptions struct {
	OlderThan time.Duration // Remove notifications older than this (default: 48h)
	Keep      int           // Keep at most N notifications (0=unlimited)
	DryRun    bool          // If true, return count without removing
}

// ChangeEvent signals store content changes.
type ChangeEvent struct {
	Type   ChangeType
	Count  int
	Source string
}

// ChangeType indicates the type of store change.
type ChangeType int

const (
	ChangeTypeAdd ChangeType = iota
	ChangeTypeClear
	ChangeTypePrune
)

// =============================================================================
// Input Adapter Interface
// =============================================================================

// InputAdapter fetches notifications from a source.
type InputAdapter interface {
	// Name returns the adapter identifier (e.g., "dunst", "stdin").
	Name() string

	// Import fetches notifications from the source.
	// Returns the notifications and any error encountered.
	// Context can be used for cancellation and timeouts.
	Import(ctx context.Context) ([]Notification, error)
}

// =============================================================================
// Output Formatter Interface
// =============================================================================

// Formatter formats notifications for output.
type Formatter interface {
	// Format outputs notifications to the writer.
	// For single notification lookup, pass a slice with one element.
	Format(w io.Writer, notifications []Notification) error
}

// GetOptions configures the get command output.
type GetOptions struct {
	// Field flags
	IncludeApp          bool
	IncludeTitle        bool
	IncludeBody         bool
	IncludeTimestamp    bool
	IncludeTimeRelative bool
	IncludeULID         bool
	IncludeAll          bool

	// Format preset or template
	Format string // "dmenu", "json", or Go template string
}

// =============================================================================
// Store Interface
// =============================================================================

// Store manages the notification history.
type Store interface {
	// Add adds a single notification to the store.
	// Persists to disk if persistence is enabled.
	// Notifies subscribers of the change.
	Add(n Notification) error

	// AddBatch adds multiple notifications efficiently.
	AddBatch(ns []Notification) error

	// All returns all notifications, sorted by timestamp (newest first by default).
	All() []Notification

	// Filter returns notifications matching the criteria.
	Filter(opts FilterOptions) []Notification

	// Lookup finds a notification by ULID or content match.
	// Input is typically a line from dmenu/fuzzel selection.
	// Returns nil if no match found.
	Lookup(input string) *Notification

	// Prune removes old notifications based on options.
	// Returns the count of removed notifications.
	// This is a reusable utility for both the prune command
	// and automatic cleanup during D-Bus stream ingest.
	Prune(opts PruneOptions) (int, error)

	// Count returns the total number of notifications.
	Count() int

	// Subscribe returns a channel that receives change events.
	// Caller must call Unsubscribe when done.
	Subscribe() <-chan ChangeEvent

	// Unsubscribe removes a subscription.
	Unsubscribe(ch <-chan ChangeEvent)

	// Close releases resources and closes all subscriber channels.
	Close() error
}

// =============================================================================
// Persistence Interface
// =============================================================================

// Persistence handles durable storage of notifications.
type Persistence interface {
	// Load reads all notifications from storage.
	// Returns empty slice if file doesn't exist.
	Load() ([]Notification, error)

	// Append adds a single notification to storage.
	Append(n Notification) error

	// AppendBatch adds multiple notifications efficiently.
	AppendBatch(ns []Notification) error

	// Rewrite replaces the entire storage file (used after prune).
	Rewrite(ns []Notification) error

	// Clear removes all stored notifications (with backup).
	Clear() error

	// Close releases file handles and resources.
	Close() error
}

// =============================================================================
// Clipboard Interface (TUI mode only)
// =============================================================================

// Clipboard handles copying text to system clipboard.
// Only used in TUI mode - shell pipelines handle clipboard for get command.
type Clipboard interface {
	// Copy copies text to the system clipboard.
	// Returns error if no clipboard tool is available.
	Copy(text string) error

	// IsAvailable checks if clipboard functionality is available.
	IsAvailable() bool
}

// =============================================================================
// TUI Interface
// =============================================================================

// TUI represents the interactive terminal UI.
type TUI interface {
	// Run starts the interactive TUI session.
	// Blocks until user quits.
	Run(store Store) error
}

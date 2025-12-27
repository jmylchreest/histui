// Package model defines the core data structures for histui.
package model

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

// Urgency levels matching freedesktop spec.
const (
	UrgencyLow      = 0
	UrgencyNormal   = 1
	UrgencyCritical = 2
)

// UrgencyNames maps urgency levels to human-readable names.
var UrgencyNames = map[int]string{
	UrgencyLow:      "low",
	UrgencyNormal:   "normal",
	UrgencyCritical: "critical",
}

// Notification represents a single notification entry.
// This is the normalized format stored in the history and used by all adapters.
type Notification struct {
	// histui metadata (added by histui)
	HistuiID          string `json:"histui_id"`
	HistuiSource      string `json:"histui_source"`
	HistuiImportedAt  int64  `json:"histui_imported_at"`
	HistuiSeenAt      int64  `json:"histui_seen_at,omitempty"`      // When viewed in TUI
	HistuiActedAt     int64  `json:"histui_acted_at,omitempty"`     // When user acted (copy)
	HistuiDismissedAt int64  `json:"histui_dismissed_at,omitempty"` // When user dismissed (soft delete)
	ContentHash       string `json:"content_hash,omitempty"`        // SHA256 hash for deduplication

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
	Progress   int    `json:"progress,omitempty"` // 0-100, -1=none
	Message    string `json:"message,omitempty"`
	URLs       string `json:"urls,omitempty"`
	Foreground string `json:"foreground,omitempty"`
	Background string `json:"background,omitempty"`

	// D-Bus notification fields (added by histuid)
	Actions      []Action `json:"actions,omitempty"`       // Available actions
	ImageData    []byte   `json:"image_data,omitempty"`    // Raw image data (if provided)
	SoundFile    string   `json:"sound_file,omitempty"`    // Requested sound file
	SoundName    string   `json:"sound_name,omitempty"`    // Named sound from spec
	DesktopEntry string   `json:"desktop_entry,omitempty"` // .desktop file name
	Resident     bool     `json:"resident,omitempty"`      // Don't auto-remove after action
	Transient    bool     `json:"transient,omitempty"`     // Don't persist
}

// Action represents a notification action with key and label.
type Action struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// Validation errors.
var (
	ErrEmptyHistuiID     = errors.New("histui_id cannot be empty")
	ErrEmptyHistuiSource = errors.New("histui_source cannot be empty")
	ErrEmptyAppName      = errors.New("app_name cannot be empty")
	ErrEmptySummary      = errors.New("summary cannot be empty")
	ErrInvalidUrgency    = errors.New("urgency must be 0, 1, or 2")
	ErrInvalidTimestamp  = errors.New("timestamp must be greater than 0")
)

// NewNotification creates a new Notification with generated ULID and metadata.
func NewNotification(source string) (*Notification, error) {
	id, err := ulid.New(ulid.Timestamp(time.Now()), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ULID: %w", err)
	}

	return &Notification{
		HistuiID:         id.String(),
		HistuiSource:     source,
		HistuiImportedAt: time.Now().Unix(),
		Urgency:          UrgencyNormal,
		UrgencyName:      UrgencyNames[UrgencyNormal],
	}, nil
}

// Validate checks that the notification has all required fields.
func (n *Notification) Validate() error {
	if n.HistuiID == "" {
		return ErrEmptyHistuiID
	}
	if n.HistuiSource == "" {
		return ErrEmptyHistuiSource
	}
	if n.AppName == "" {
		return ErrEmptyAppName
	}
	if n.Summary == "" {
		return ErrEmptySummary
	}
	if n.Urgency < 0 || n.Urgency > 2 {
		return ErrInvalidUrgency
	}
	if n.Timestamp <= 0 {
		return ErrInvalidTimestamp
	}
	return nil
}

// SetUrgency sets the urgency level and its human-readable name.
func (n *Notification) SetUrgency(level int) {
	if level < 0 || level > 2 {
		level = UrgencyNormal
	}
	n.Urgency = level
	n.UrgencyName = UrgencyNames[level]
}

// RelativeTime returns a human-readable relative time string.
// Examples: "just now", "5m ago", "2h ago", "1d ago".
func (n *Notification) RelativeTime() string {
	now := time.Now().Unix()
	diff := now - n.Timestamp

	if diff < 0 {
		return "in the future"
	}
	if diff < 60 {
		return "just now"
	}
	if diff < 3600 {
		minutes := diff / 60
		return fmt.Sprintf("%dm ago", minutes)
	}
	if diff < 86400 {
		hours := diff / 3600
		return fmt.Sprintf("%dh ago", hours)
	}
	days := diff / 86400
	return fmt.Sprintf("%dd ago", days)
}

// BodyTruncated returns the body truncated to maxLen characters.
// If the body is longer, it is truncated and "..." is appended.
func (n *Notification) BodyTruncated(maxLen int) string {
	if maxLen <= 0 {
		return ""
	}

	// Collapse whitespace and newlines to single spaces
	body := strings.Join(strings.Fields(n.Body), " ")

	if len(body) <= maxLen {
		return body
	}
	if maxLen <= 3 {
		return body[:maxLen]
	}
	return body[:maxLen-3] + "..."
}

// DedupeKey returns a string key for deduplication.
// Notifications with the same key (same app, summary, body, and timestamp within 1 second)
// are considered duplicates.
func (n *Notification) DedupeKey() string {
	return fmt.Sprintf("%s:%s:%s:%d",
		n.AppName,
		n.Summary,
		n.Body,
		n.Timestamp, // 1-second granularity
	)
}

// ComputeContentHash generates a SHA256 hash of the notification content.
// This is used for deduplication across imports.
func (n *Notification) ComputeContentHash() string {
	hash := sha256.Sum256([]byte(n.DedupeKey()))
	return hex.EncodeToString(hash[:])
}

// EnsureContentHash computes and sets the ContentHash if not already set.
func (n *Notification) EnsureContentHash() {
	if n.ContentHash == "" {
		n.ContentHash = n.ComputeContentHash()
	}
}

// TimestampTime returns the timestamp as a time.Time.
func (n *Notification) TimestampTime() time.Time {
	return time.Unix(n.Timestamp, 0)
}

// ImportedAtTime returns the import timestamp as a time.Time.
func (n *Notification) ImportedAtTime() time.Time {
	return time.Unix(n.HistuiImportedAt, 0)
}

// Clone creates a deep copy of the notification.
func (n *Notification) Clone() *Notification {
	clone := *n
	if n.Extensions != nil {
		extClone := *n.Extensions
		clone.Extensions = &extClone
	}
	return &clone
}

// IsSeen returns true if the notification has been viewed in histui.
func (n *Notification) IsSeen() bool {
	return n.HistuiSeenAt > 0
}

// IsActed returns true if the user has acted on the notification (copy, dismiss).
func (n *Notification) IsActed() bool {
	return n.HistuiActedAt > 0
}

// MarkSeen marks the notification as seen at the current time.
func (n *Notification) MarkSeen() {
	if n.HistuiSeenAt == 0 {
		n.HistuiSeenAt = time.Now().Unix()
	}
}

// MarkActed marks the notification as acted upon at the current time.
func (n *Notification) MarkActed() {
	n.HistuiActedAt = time.Now().Unix()
	// Acting implies seeing
	if n.HistuiSeenAt == 0 {
		n.HistuiSeenAt = n.HistuiActedAt
	}
}

// IsDismissed returns true if the notification has been dismissed.
func (n *Notification) IsDismissed() bool {
	return n.HistuiDismissedAt > 0
}

// MarkDismissed marks the notification as dismissed at the current time.
func (n *Notification) MarkDismissed() {
	n.HistuiDismissedAt = time.Now().Unix()
	// Dismissing implies seeing
	if n.HistuiSeenAt == 0 {
		n.HistuiSeenAt = n.HistuiDismissedAt
	}
}

// Undismiss clears the dismissed state.
func (n *Notification) Undismiss() {
	n.HistuiDismissedAt = 0
}

package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNotification(t *testing.T) {
	n, err := NewNotification("test")
	require.NoError(t, err)

	assert.NotEmpty(t, n.HistuiID)
	assert.Equal(t, "test", n.HistuiSource)
	assert.Greater(t, n.HistuiImportedAt, int64(0))
	assert.Equal(t, UrgencyNormal, n.Urgency)
	assert.Equal(t, "normal", n.UrgencyName)
}

func TestNotification_Validate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Notification)
		wantErr error
	}{
		{
			name:    "valid notification",
			modify:  func(n *Notification) {},
			wantErr: nil,
		},
		{
			name: "empty histui_id",
			modify: func(n *Notification) {
				n.HistuiID = ""
			},
			wantErr: ErrEmptyHistuiID,
		},
		{
			name: "empty histui_source",
			modify: func(n *Notification) {
				n.HistuiSource = ""
			},
			wantErr: ErrEmptyHistuiSource,
		},
		{
			name: "empty app_name",
			modify: func(n *Notification) {
				n.AppName = ""
			},
			wantErr: ErrEmptyAppName,
		},
		{
			name: "empty summary",
			modify: func(n *Notification) {
				n.Summary = ""
			},
			wantErr: ErrEmptySummary,
		},
		{
			name: "invalid urgency (negative)",
			modify: func(n *Notification) {
				n.Urgency = -1
			},
			wantErr: ErrInvalidUrgency,
		},
		{
			name: "invalid urgency (too high)",
			modify: func(n *Notification) {
				n.Urgency = 3
			},
			wantErr: ErrInvalidUrgency,
		},
		{
			name: "invalid timestamp",
			modify: func(n *Notification) {
				n.Timestamp = 0
			},
			wantErr: ErrInvalidTimestamp,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := validNotification()
			tt.modify(n)
			err := n.Validate()
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNotification_SetUrgency(t *testing.T) {
	tests := []struct {
		level       int
		wantUrgency int
		wantName    string
	}{
		{UrgencyLow, UrgencyLow, "low"},
		{UrgencyNormal, UrgencyNormal, "normal"},
		{UrgencyCritical, UrgencyCritical, "critical"},
		{-1, UrgencyNormal, "normal"}, // Invalid defaults to normal
		{5, UrgencyNormal, "normal"},  // Invalid defaults to normal
	}

	for _, tt := range tests {
		t.Run(tt.wantName, func(t *testing.T) {
			n := &Notification{}
			n.SetUrgency(tt.level)
			assert.Equal(t, tt.wantUrgency, n.Urgency)
			assert.Equal(t, tt.wantName, n.UrgencyName)
		})
	}
}

func TestNotification_RelativeTime(t *testing.T) {
	now := time.Now().Unix()

	tests := []struct {
		name      string
		timestamp int64
		want      string
	}{
		{"just now", now - 30, "just now"},
		{"5 minutes ago", now - 300, "5m ago"},
		{"1 hour ago", now - 3600, "1h ago"},
		{"2 hours ago", now - 7200, "2h ago"},
		{"1 day ago", now - 86400, "1d ago"},
		{"3 days ago", now - 259200, "3d ago"},
		{"future timestamp", now + 100, "in the future"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &Notification{Timestamp: tt.timestamp}
			assert.Equal(t, tt.want, n.RelativeTime())
		})
	}
}

func TestNotification_BodyTruncated(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		maxLen int
		want   string
	}{
		{"short body", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"truncated", "hello world", 8, "hello..."},
		{"very short max", "hello", 3, "hel"},
		{"zero max", "hello", 0, ""},
		{"negative max", "hello", -1, ""},
		{"multiline body", "hello\nworld\ntest", 20, "hello world test"},
		{"tabs and spaces", "hello\t\t  world", 20, "hello world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &Notification{Body: tt.body}
			assert.Equal(t, tt.want, n.BodyTruncated(tt.maxLen))
		})
	}
}

func TestNotification_DedupeKey(t *testing.T) {
	n1 := &Notification{
		AppName:   "firefox",
		Summary:   "Download Complete",
		Body:      "file.zip",
		Timestamp: 1703577600,
	}

	n2 := &Notification{
		AppName:   "firefox",
		Summary:   "Download Complete",
		Body:      "file.zip",
		Timestamp: 1703577600,
	}

	n3 := &Notification{
		AppName:   "firefox",
		Summary:   "Download Complete",
		Body:      "file.zip",
		Timestamp: 1703577601, // Different timestamp
	}

	assert.Equal(t, n1.DedupeKey(), n2.DedupeKey())
	assert.NotEqual(t, n1.DedupeKey(), n3.DedupeKey())
}

func TestNotification_TimestampTime(t *testing.T) {
	ts := int64(1703577600)
	n := &Notification{Timestamp: ts}
	expected := time.Unix(ts, 0)
	assert.Equal(t, expected, n.TimestampTime())
}

func TestNotification_Clone(t *testing.T) {
	n := validNotification()
	n.Extensions = &Extensions{
		StackTag: "test-tag",
		Progress: 50,
	}

	clone := n.Clone()

	// Verify values are copied
	assert.Equal(t, n.HistuiID, clone.HistuiID)
	assert.Equal(t, n.AppName, clone.AppName)
	assert.Equal(t, n.Extensions.StackTag, clone.Extensions.StackTag)

	// Verify independence
	clone.AppName = "modified"
	clone.Extensions.StackTag = "modified-tag"

	assert.NotEqual(t, n.AppName, clone.AppName)
	assert.NotEqual(t, n.Extensions.StackTag, clone.Extensions.StackTag)
}

func TestNotification_Clone_NilExtensions(t *testing.T) {
	n := validNotification()
	n.Extensions = nil

	clone := n.Clone()
	assert.Nil(t, clone.Extensions)
}

func TestULIDFormat(t *testing.T) {
	// Verify ULIDs are valid 26-character strings
	n, err := NewNotification("test")
	require.NoError(t, err)

	assert.Len(t, n.HistuiID, 26, "ULID should be 26 characters")

	// Verify it's a valid ULID by parsing
	for _, c := range n.HistuiID {
		// ULID uses Crockford's base32: 0-9, A-Z except I, L, O, U
		valid := (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z' && c != 'I' && c != 'L' && c != 'O' && c != 'U')
		assert.True(t, valid, "ULID character %c should be valid Crockford base32", c)
	}
}

// Helper function to create a valid notification for testing.
func validNotification() *Notification {
	return &Notification{
		HistuiID:         "01HQGXK5P0000000000000000",
		HistuiSource:     "dunst",
		HistuiImportedAt: time.Now().Unix(),
		ID:               123,
		AppName:          "firefox",
		Summary:          "Download Complete",
		Body:             "myfile.zip has finished downloading",
		Timestamp:        time.Now().Unix(),
		Urgency:          UrgencyNormal,
		UrgencyName:      "normal",
	}
}

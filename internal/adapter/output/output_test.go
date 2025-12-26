package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jmylchreest/histui/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testNotifications() []model.Notification {
	now := time.Now()
	return []model.Notification{
		{
			HistuiID:  "abc123",
			AppName:   "Firefox",
			Summary:   "Download Complete",
			Body:      "myfile.zip has finished downloading",
			Timestamp: now.Add(-5 * time.Minute).Unix(),
			Urgency:   model.UrgencyNormal,
		},
		{
			HistuiID:  "def456",
			AppName:   "Slack",
			Summary:   "New Message",
			Body:      "Hello from John",
			Timestamp: now.Add(-2 * time.Hour).Unix(),
			Urgency:   model.UrgencyCritical,
		},
	}
}

func TestDmenuFormatter_Format(t *testing.T) {
	notifications := testNotifications()
	var buf bytes.Buffer

	formatter := NewDmenuFormatter(DefaultFormatterOptions())
	err := formatter.Format(&buf, notifications)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	assert.Len(t, lines, 2)

	// First line should contain index 1
	assert.Contains(t, lines[0], "1")
	assert.Contains(t, lines[0], "Firefox")
	assert.Contains(t, lines[0], "Download Complete")

	// Second line should contain index 2
	assert.Contains(t, lines[1], "2")
	assert.Contains(t, lines[1], "Slack")
	assert.Contains(t, lines[1], "New Message")
}

func TestDmenuFormatter_NoIndex(t *testing.T) {
	notifications := testNotifications()
	var buf bytes.Buffer

	opts := DefaultFormatterOptions()
	opts.ShowIndex = false
	formatter := NewDmenuFormatter(opts)
	err := formatter.Format(&buf, notifications)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	// Should not start with a number when index is disabled
	assert.False(t, strings.HasPrefix(lines[0], "1"))
}

func TestDmenuFormatter_CustomTemplate(t *testing.T) {
	notifications := testNotifications()
	var buf bytes.Buffer

	opts := DefaultFormatterOptions()
	opts.Template = "{{.Index}}: {{.Notification.AppName}} - {{.Notification.Summary}}"
	formatter := NewDmenuFormatter(opts)
	err := formatter.Format(&buf, notifications)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	assert.Equal(t, "1: Firefox - Download Complete", lines[0])
	assert.Equal(t, "2: Slack - New Message", lines[1])
}

func TestDmenuFormatter_TruncateBody(t *testing.T) {
	notifications := []model.Notification{
		{
			HistuiID:  "test",
			AppName:   "Test",
			Summary:   "Test",
			Body:      "This is a very long body that should be truncated when the max length is set",
			Timestamp: time.Now().Unix(),
		},
	}
	var buf bytes.Buffer

	opts := DefaultFormatterOptions()
	opts.BodyMaxLen = 20
	formatter := NewDmenuFormatter(opts)
	err := formatter.Format(&buf, notifications)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "...")
	assert.NotContains(t, output, "truncated when the max length is set")
}

func TestJSONFormatter_Format(t *testing.T) {
	notifications := testNotifications()
	var buf bytes.Buffer

	formatter := NewJSONFormatter(DefaultFormatterOptions())
	err := formatter.Format(&buf, notifications)
	require.NoError(t, err)

	// Should be valid JSON
	var result []model.Notification
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "Firefox", result[0].AppName)
	assert.Equal(t, "Slack", result[1].AppName)
}

func TestJSONFormatter_FormatSingle(t *testing.T) {
	n := &model.Notification{
		HistuiID:  "test123",
		AppName:   "TestApp",
		Summary:   "Test Summary",
		Timestamp: time.Now().Unix(),
	}
	var buf bytes.Buffer

	formatter := NewJSONFormatter(DefaultFormatterOptions())
	err := formatter.FormatSingle(&buf, n)
	require.NoError(t, err)

	var result model.Notification
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "TestApp", result.AppName)
}

func TestPlainFormatter_Format(t *testing.T) {
	notifications := testNotifications()
	var buf bytes.Buffer

	formatter := NewPlainFormatter(DefaultFormatterOptions())
	err := formatter.Format(&buf, notifications)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "[1]")
	assert.Contains(t, output, "<Firefox>")
	assert.Contains(t, output, "Download Complete")
	assert.Contains(t, output, "[2]")
	assert.Contains(t, output, "<Slack>")
}

func TestFormatField(t *testing.T) {
	n := &model.Notification{
		HistuiID:    "test123",
		AppName:     "Firefox",
		Summary:     "Download Complete",
		Body:        "file.zip finished",
		Category:    "transfer.complete",
		IconPath:    "/usr/share/icons/firefox.png",
		UrgencyName: "normal",
	}

	tests := []struct {
		field    string
		expected string
	}{
		{"id", "test123"},
		{"histui_id", "test123"},
		{"app", "Firefox"},
		{"app_name", "Firefox"},
		{"summary", "Download Complete"},
		{"body", "file.zip finished"},
		{"category", "transfer.complete"},
		{"icon", "/usr/share/icons/firefox.png"},
		{"urgency", "normal"},
		{"all", "Download Complete\nfile.zip finished"},
		{"unknown", "Download Complete"}, // defaults to summary
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			result := FormatField(n, tt.field)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewFormatter(t *testing.T) {
	opts := DefaultFormatterOptions()

	t.Run("dmenu", func(t *testing.T) {
		f := NewFormatter(FormatDmenu, opts)
		_, ok := f.(*DmenuFormatter)
		assert.True(t, ok)
	})

	t.Run("json", func(t *testing.T) {
		f := NewFormatter(FormatJSON, opts)
		_, ok := f.(*JSONFormatter)
		assert.True(t, ok)
	})

	t.Run("plain", func(t *testing.T) {
		f := NewFormatter(FormatPlain, opts)
		_, ok := f.(*PlainFormatter)
		assert.True(t, ok)
	})

	t.Run("default", func(t *testing.T) {
		f := NewFormatter("unknown", opts)
		_, ok := f.(*DmenuFormatter)
		assert.True(t, ok) // defaults to dmenu
	})
}

func TestSanitizeBody(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		maxLen         int
		includeNewline bool
		expected       string
	}{
		{"simple", "hello world", 0, false, "hello world"},
		{"with newlines", "hello\nworld", 0, false, "hello world"},
		{"preserve newlines", "hello\nworld", 0, true, "hello\nworld"},
		{"truncate", "hello world", 8, false, "hello..."},
		{"multiple spaces", "hello   world", 0, false, "hello world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeBody(tt.body, tt.maxLen, tt.includeNewline)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRelativeTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		ts       int64
		expected string
	}{
		{"zero", 0, "unknown"},
		{"now", now.Unix(), "now"},
		{"30 seconds", now.Add(-30 * time.Second).Unix(), "now"},
		{"5 minutes", now.Add(-5 * time.Minute).Unix(), "5m"},
		{"2 hours", now.Add(-2 * time.Hour).Unix(), "2h"},
		{"3 days", now.Add(-72 * time.Hour).Unix(), "3d"},
		{"2 weeks", now.Add(-14 * 24 * time.Hour).Unix(), "2w"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := relativeTime(tt.ts)
			assert.Equal(t, tt.expected, result)
		})
	}
}

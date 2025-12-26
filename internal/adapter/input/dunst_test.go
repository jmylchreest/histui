package input

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jmylchreest/histui/internal/model"
)

func TestDunstAdapter_Name(t *testing.T) {
	adapter := NewDunstAdapter()
	assert.Equal(t, "dunst", adapter.Name())
}

func TestParseDunstHistory(t *testing.T) {
	// Sample dunstctl history output
	jsonData := []byte(`{
		"type": "array",
		"data": [[
			{
				"id": {"type": "INT", "data": 123},
				"appname": {"type": "STRING", "data": "firefox"},
				"summary": {"type": "STRING", "data": "Download Complete"},
				"body": {"type": "STRING", "data": "myfile.zip has finished downloading"},
				"timestamp": {"type": "INT", "data": 1703577600000000},
				"timeout": {"type": "INT", "data": 5000},
				"urgency": {"type": "INT", "data": 1},
				"category": {"type": "STRING", "data": "transfer.complete"},
				"icon_path": {"type": "STRING", "data": "/usr/share/icons/firefox.png"},
				"default_action_name": {"type": "STRING", "data": ""},
				"progress": {"type": "INT", "data": -1},
				"message": {"type": "STRING", "data": ""},
				"urls": {"type": "STRING", "data": ""},
				"fg": {"type": "STRING", "data": ""},
				"bg": {"type": "STRING", "data": ""},
				"stack_tag": {"type": "STRING", "data": ""}
			},
			{
				"id": {"type": "INT", "data": 124},
				"appname": {"type": "STRING", "data": "slack"},
				"summary": {"type": "STRING", "data": "New Message"},
				"body": {"type": "STRING", "data": "Hello from John"},
				"timestamp": {"type": "INT", "data": 1703577700000000},
				"timeout": {"type": "INT", "data": 0},
				"urgency": {"type": "INT", "data": 2},
				"category": {"type": "STRING", "data": ""},
				"icon_path": {"type": "STRING", "data": ""},
				"default_action_name": {"type": "STRING", "data": ""},
				"progress": {"type": "INT", "data": -1},
				"message": {"type": "STRING", "data": ""},
				"urls": {"type": "STRING", "data": ""},
				"fg": {"type": "STRING", "data": ""},
				"bg": {"type": "STRING", "data": ""},
				"stack_tag": {"type": "STRING", "data": ""}
			}
		]]
	}`)

	notifications, err := ParseDunstHistory(jsonData)
	require.NoError(t, err)
	require.Len(t, notifications, 2)

	// Check first notification
	n1 := notifications[0]
	assert.Equal(t, "firefox", n1.AppName)
	assert.Equal(t, "Download Complete", n1.Summary)
	assert.Equal(t, "myfile.zip has finished downloading", n1.Body)
	assert.Equal(t, model.UrgencyNormal, n1.Urgency)
	assert.Equal(t, "normal", n1.UrgencyName)
	assert.Equal(t, "dunst", n1.HistuiSource)
	assert.NotEmpty(t, n1.HistuiID)
	assert.Equal(t, 123, n1.ID)

	// Check second notification
	n2 := notifications[1]
	assert.Equal(t, "slack", n2.AppName)
	assert.Equal(t, "New Message", n2.Summary)
	assert.Equal(t, model.UrgencyCritical, n2.Urgency)
	assert.Equal(t, "critical", n2.UrgencyName)
}

func TestParseDunstHistory_Empty(t *testing.T) {
	jsonData := []byte(`{"type": "array", "data": [[]]}`)

	notifications, err := ParseDunstHistory(jsonData)
	require.NoError(t, err)
	assert.Len(t, notifications, 0)
}

func TestParseDunstHistory_InvalidJSON(t *testing.T) {
	jsonData := []byte(`{invalid json`)

	_, err := ParseDunstHistory(jsonData)
	assert.Error(t, err)
}

func TestParseDunstHistory_WithExtensions(t *testing.T) {
	jsonData := []byte(`{
		"type": "array",
		"data": [[
			{
				"id": {"type": "INT", "data": 100},
				"appname": {"type": "STRING", "data": "test-app"},
				"summary": {"type": "STRING", "data": "Progress Test"},
				"body": {"type": "STRING", "data": "Downloading..."},
				"timestamp": {"type": "INT", "data": 1703577600000000},
				"timeout": {"type": "INT", "data": 0},
				"urgency": {"type": "INT", "data": 1},
				"category": {"type": "STRING", "data": ""},
				"icon_path": {"type": "STRING", "data": ""},
				"default_action_name": {"type": "STRING", "data": ""},
				"progress": {"type": "INT", "data": 75},
				"message": {"type": "STRING", "data": ""},
				"urls": {"type": "STRING", "data": "https://example.com"},
				"fg": {"type": "STRING", "data": "#ffffff"},
				"bg": {"type": "STRING", "data": "#000000"},
				"stack_tag": {"type": "STRING", "data": "download-progress"}
			}
		]]
	}`)

	notifications, err := ParseDunstHistory(jsonData)
	require.NoError(t, err)
	require.Len(t, notifications, 1)

	n := notifications[0]
	require.NotNil(t, n.Extensions)
	assert.Equal(t, 75, n.Extensions.Progress)
	assert.Equal(t, "https://example.com", n.Extensions.URLs)
	assert.Equal(t, "#ffffff", n.Extensions.Foreground)
	assert.Equal(t, "#000000", n.Extensions.Background)
	assert.Equal(t, "download-progress", n.Extensions.StackTag)
}

func TestSanitizeString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal string", "normal string"},
		{"with\nnewline", "with\nnewline"},
		{"with\ttab", "with\ttab"},
		{"  trimmed  ", "trimmed"},
		{"control\x00char", "control char"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDunstValue_String(t *testing.T) {
	tests := []struct {
		name     string
		value    dunstValue
		expected string
	}{
		{"string value", dunstValue{Type: "STRING", Data: "hello"}, "hello"},
		{"int value", dunstValue{Type: "INT", Data: float64(123)}, "123"},
		{"nil value", dunstValue{Type: "STRING", Data: nil}, ""},
		{"empty string", dunstValue{Type: "STRING", Data: ""}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.value.String())
		})
	}
}

func TestDunstValue_Int(t *testing.T) {
	tests := []struct {
		name     string
		value    dunstValue
		expected int
	}{
		{"float64 value", dunstValue{Type: "INT", Data: float64(123)}, 123},
		{"int64 value", dunstValue{Type: "INT", Data: int64(456)}, 456},
		{"string value", dunstValue{Type: "STRING", Data: "789"}, 789},
		{"nil value", dunstValue{Type: "INT", Data: nil}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.value.Int())
		})
	}
}

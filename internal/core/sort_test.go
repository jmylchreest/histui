package core

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jmylchreest/histui/internal/model"
)

func TestSort_Empty(t *testing.T) {
	var notifications []model.Notification
	Sort(notifications, DefaultSortOptions())
	assert.Len(t, notifications, 0)
}

func TestSort_ByTimestampDesc(t *testing.T) {
	notifications := []model.Notification{
		{HistuiID: "1", Timestamp: 100},
		{HistuiID: "2", Timestamp: 300},
		{HistuiID: "3", Timestamp: 200},
	}

	Sort(notifications, SortOptions{Field: SortByTimestamp, Order: SortDesc})

	assert.Equal(t, "2", notifications[0].HistuiID) // 300
	assert.Equal(t, "3", notifications[1].HistuiID) // 200
	assert.Equal(t, "1", notifications[2].HistuiID) // 100
}

func TestSort_ByTimestampAsc(t *testing.T) {
	notifications := []model.Notification{
		{HistuiID: "1", Timestamp: 100},
		{HistuiID: "2", Timestamp: 300},
		{HistuiID: "3", Timestamp: 200},
	}

	Sort(notifications, SortOptions{Field: SortByTimestamp, Order: SortAsc})

	assert.Equal(t, "1", notifications[0].HistuiID) // 100
	assert.Equal(t, "3", notifications[1].HistuiID) // 200
	assert.Equal(t, "2", notifications[2].HistuiID) // 300
}

func TestSort_ByAppDesc(t *testing.T) {
	notifications := []model.Notification{
		{HistuiID: "1", AppName: "Firefox"},
		{HistuiID: "2", AppName: "Slack"},
		{HistuiID: "3", AppName: "Discord"},
	}

	Sort(notifications, SortOptions{Field: SortByApp, Order: SortDesc})

	assert.Equal(t, "2", notifications[0].HistuiID) // Slack
	assert.Equal(t, "1", notifications[1].HistuiID) // Firefox
	assert.Equal(t, "3", notifications[2].HistuiID) // Discord
}

func TestSort_ByAppAsc(t *testing.T) {
	notifications := []model.Notification{
		{HistuiID: "1", AppName: "Firefox"},
		{HistuiID: "2", AppName: "Slack"},
		{HistuiID: "3", AppName: "Discord"},
	}

	Sort(notifications, SortOptions{Field: SortByApp, Order: SortAsc})

	assert.Equal(t, "3", notifications[0].HistuiID) // Discord
	assert.Equal(t, "1", notifications[1].HistuiID) // Firefox
	assert.Equal(t, "2", notifications[2].HistuiID) // Slack
}

func TestSort_ByUrgencyDesc(t *testing.T) {
	notifications := []model.Notification{
		{HistuiID: "1", Urgency: model.UrgencyNormal},
		{HistuiID: "2", Urgency: model.UrgencyLow},
		{HistuiID: "3", Urgency: model.UrgencyCritical},
	}

	Sort(notifications, SortOptions{Field: SortByUrgency, Order: SortDesc})

	assert.Equal(t, "3", notifications[0].HistuiID) // Critical (2)
	assert.Equal(t, "1", notifications[1].HistuiID) // Normal (1)
	assert.Equal(t, "2", notifications[2].HistuiID) // Low (0)
}

func TestSort_ByUrgencyAsc(t *testing.T) {
	notifications := []model.Notification{
		{HistuiID: "1", Urgency: model.UrgencyNormal},
		{HistuiID: "2", Urgency: model.UrgencyLow},
		{HistuiID: "3", Urgency: model.UrgencyCritical},
	}

	Sort(notifications, SortOptions{Field: SortByUrgency, Order: SortAsc})

	assert.Equal(t, "2", notifications[0].HistuiID) // Low (0)
	assert.Equal(t, "1", notifications[1].HistuiID) // Normal (1)
	assert.Equal(t, "3", notifications[2].HistuiID) // Critical (2)
}

func TestSort_CaseInsensitiveApp(t *testing.T) {
	notifications := []model.Notification{
		{HistuiID: "1", AppName: "firefox"},
		{HistuiID: "2", AppName: "FIREFOX"},
		{HistuiID: "3", AppName: "Firefox"},
	}

	Sort(notifications, SortOptions{Field: SortByApp, Order: SortAsc})

	// All should be considered equal, stable sort preserves order
	assert.Equal(t, "1", notifications[0].HistuiID)
	assert.Equal(t, "2", notifications[1].HistuiID)
	assert.Equal(t, "3", notifications[2].HistuiID)
}

func TestDefaultSortOptions(t *testing.T) {
	opts := DefaultSortOptions()
	assert.Equal(t, SortByTimestamp, opts.Field)
	assert.Equal(t, SortDesc, opts.Order)
}

func TestParseSortField(t *testing.T) {
	tests := []struct {
		input    string
		expected SortField
	}{
		{"timestamp", SortByTimestamp},
		{"time", SortByTimestamp},
		{"t", SortByTimestamp},
		{"app", SortByApp},
		{"appname", SortByApp},
		{"a", SortByApp},
		{"urgency", SortByUrgency},
		{"u", SortByUrgency},
		{"unknown", SortByTimestamp}, // defaults to timestamp
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, _ := ParseSortField(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseSortOrder(t *testing.T) {
	tests := []struct {
		input    string
		expected SortOrder
	}{
		{"asc", SortAsc},
		{"ascending", SortAsc},
		{"a", SortAsc},
		{"desc", SortDesc},
		{"descending", SortDesc},
		{"d", SortDesc},
		{"unknown", SortDesc}, // defaults to desc
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, _ := ParseSortOrder(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

package core

import (
	"testing"
	"time"

	"github.com/jmylchreest/histui/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilter_Empty(t *testing.T) {
	result := Filter(nil, FilterOptions{})
	assert.Len(t, result, 0)
}

func TestFilter_NoFilters(t *testing.T) {
	notifications := []model.Notification{
		{HistuiID: "1", AppName: "firefox"},
		{HistuiID: "2", AppName: "slack"},
	}

	result := Filter(notifications, FilterOptions{})
	assert.Len(t, result, 2)
}

func TestFilter_ByApp(t *testing.T) {
	notifications := []model.Notification{
		{HistuiID: "1", AppName: "firefox"},
		{HistuiID: "2", AppName: "slack"},
		{HistuiID: "3", AppName: "firefox"},
	}

	result := Filter(notifications, FilterOptions{AppFilter: "firefox"})
	assert.Len(t, result, 2)
	for _, n := range result {
		assert.Equal(t, "firefox", n.AppName)
	}
}

func TestFilter_ByUrgency(t *testing.T) {
	critical := model.UrgencyCritical
	notifications := []model.Notification{
		{HistuiID: "1", AppName: "firefox", Urgency: model.UrgencyLow},
		{HistuiID: "2", AppName: "slack", Urgency: model.UrgencyCritical},
		{HistuiID: "3", AppName: "discord", Urgency: model.UrgencyCritical},
	}

	result := Filter(notifications, FilterOptions{Urgency: &critical})
	assert.Len(t, result, 2)
	for _, n := range result {
		assert.Equal(t, model.UrgencyCritical, n.Urgency)
	}
}

func TestFilter_BySince(t *testing.T) {
	now := time.Now()
	notifications := []model.Notification{
		{HistuiID: "1", Timestamp: now.Add(-30 * time.Minute).Unix()},
		{HistuiID: "2", Timestamp: now.Add(-2 * time.Hour).Unix()},
		{HistuiID: "3", Timestamp: now.Add(-5 * time.Hour).Unix()},
	}

	result := Filter(notifications, FilterOptions{Since: time.Hour})
	assert.Len(t, result, 1)
	assert.Equal(t, "1", result[0].HistuiID)
}

func TestFilter_WithLimit(t *testing.T) {
	notifications := []model.Notification{
		{HistuiID: "1"},
		{HistuiID: "2"},
		{HistuiID: "3"},
		{HistuiID: "4"},
		{HistuiID: "5"},
	}

	result := Filter(notifications, FilterOptions{Limit: 3})
	assert.Len(t, result, 3)
}

func TestFilter_Combined(t *testing.T) {
	now := time.Now()
	critical := model.UrgencyCritical
	notifications := []model.Notification{
		{HistuiID: "1", AppName: "firefox", Urgency: model.UrgencyCritical, Timestamp: now.Add(-30 * time.Minute).Unix()},
		{HistuiID: "2", AppName: "firefox", Urgency: model.UrgencyNormal, Timestamp: now.Add(-30 * time.Minute).Unix()},
		{HistuiID: "3", AppName: "slack", Urgency: model.UrgencyCritical, Timestamp: now.Add(-30 * time.Minute).Unix()},
		{HistuiID: "4", AppName: "firefox", Urgency: model.UrgencyCritical, Timestamp: now.Add(-5 * time.Hour).Unix()},
	}

	result := Filter(notifications, FilterOptions{
		AppFilter: "firefox",
		Urgency:   &critical,
		Since:     time.Hour,
	})
	assert.Len(t, result, 1)
	assert.Equal(t, "1", result[0].HistuiID)
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		hasError bool
	}{
		{"0", 0, false},
		{"", 0, false},
		{"1h", time.Hour, false},
		{"30m", 30 * time.Minute, false},
		{"48h", 48 * time.Hour, false},
		{"7d", 7 * 24 * time.Hour, false},
		{"1w", 7 * 24 * time.Hour, false},
		{"2w", 14 * 24 * time.Hour, false},
		{"invalid", 0, true},
		{"xd", 0, true},
		{"xw", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ParseDuration(tt.input)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestParseUrgency(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		hasError bool
	}{
		{"low", model.UrgencyLow, false},
		{"LOW", model.UrgencyLow, false},
		{"0", model.UrgencyLow, false},
		{"normal", model.UrgencyNormal, false},
		{"NORMAL", model.UrgencyNormal, false},
		{"1", model.UrgencyNormal, false},
		{"critical", model.UrgencyCritical, false},
		{"CRITICAL", model.UrgencyCritical, false},
		{"2", model.UrgencyCritical, false},
		{"invalid", 0, true},
		{"3", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ParseUrgency(tt.input)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

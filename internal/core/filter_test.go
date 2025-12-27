package core

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jmylchreest/histui/internal/model"
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

func TestParseFilter_Empty(t *testing.T) {
	expr, err := ParseFilter("")
	require.NoError(t, err)
	assert.Len(t, expr.Conditions, 0)
}

func TestParseFilter_SingleCondition(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		field    string
		operator FilterOp
		value    string
		hasError bool
	}{
		{"equal", "app=discord", "app", FilterOpEqual, "discord", false},
		{"not_equal", "app!=slack", "app", FilterOpNotEqual, "slack", false},
		{"contains", "body~error", "body", FilterOpContains, "error", false},
		{"regex", "summary~=(?i)test", "summary", FilterOpRegex, "(?i)test", false},
		{"greater", "urgency>low", "urgency", FilterOpGreater, "low", false},
		{"less", "urgency<critical", "urgency", FilterOpLess, "critical", false},
		{"greater_eq", "urgency>=normal", "urgency", FilterOpGreaterEq, "normal", false},
		{"less_eq", "urgency<=normal", "urgency", FilterOpLessEq, "normal", false},
		{"dismissed", "dismissed=true", "dismissed", FilterOpEqual, "true", false},
		{"seen", "seen=false", "seen", FilterOpEqual, "false", false},
		{"unknown_field", "unknown=value", "", "", "", true},
		{"no_operator", "appname", "", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := ParseFilter(tt.input)
			if tt.hasError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Len(t, expr.Conditions, 1)
			assert.Equal(t, tt.field, expr.Conditions[0].Field)
			assert.Equal(t, tt.operator, expr.Conditions[0].Operator)
			assert.Equal(t, tt.value, expr.Conditions[0].Value)
		})
	}
}

func TestParseFilter_MultipleConditions(t *testing.T) {
	expr, err := ParseFilter("app=slack,urgency=critical,dismissed=false")
	require.NoError(t, err)
	require.Len(t, expr.Conditions, 3)

	assert.Equal(t, "app", expr.Conditions[0].Field)
	assert.Equal(t, "urgency", expr.Conditions[1].Field)
	assert.Equal(t, "dismissed", expr.Conditions[2].Field)
}

func TestFilterExpr_Match(t *testing.T) {
	now := time.Now()

	notifications := []model.Notification{
		{HistuiID: "1", AppName: "discord", Summary: "New message", Body: "Hello world", Urgency: model.UrgencyNormal, Timestamp: now.Unix()},
		{HistuiID: "2", AppName: "slack", Summary: "Meeting reminder", Body: "Meeting in 5 minutes", Urgency: model.UrgencyCritical, Timestamp: now.Unix()},
		{HistuiID: "3", AppName: "discord", Summary: "Error occurred", Body: "Something went wrong", Urgency: model.UrgencyCritical, Timestamp: now.Unix()},
		{HistuiID: "4", AppName: "firefox", Summary: "Download complete", Body: "file.zip downloaded", Urgency: model.UrgencyLow, HistuiDismissedAt: now.Unix(), Timestamp: now.Unix()},
	}

	tests := []struct {
		name     string
		filter   string
		expected []string // Expected HistuiIDs
	}{
		{"app_equal", "app=discord", []string{"1", "3"}},
		{"app_not_equal", "app!=discord", []string{"2", "4"}},
		{"summary_contains", "summary~error", []string{"3"}},
		{"body_contains", "body~world", []string{"1"}},
		{"urgency_equal", "urgency=critical", []string{"2", "3"}},
		{"urgency_greater", "urgency>normal", []string{"2", "3"}},
		{"urgency_less_eq", "urgency<=normal", []string{"1", "4"}},
		{"dismissed", "dismissed=true", []string{"4"}},
		{"not_dismissed", "dismissed=false", []string{"1", "2", "3"}},
		{"combined", "app=discord,urgency=critical", []string{"3"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := ParseFilter(tt.filter)
			require.NoError(t, err)

			result := FilterWithExpr(notifications, expr)
			resultIDs := make([]string, len(result))
			for i, n := range result {
				resultIDs[i] = n.HistuiID
			}
			assert.ElementsMatch(t, tt.expected, resultIDs)
		})
	}
}

func TestFilterExpr_MatchRegex(t *testing.T) {
	notifications := []model.Notification{
		{HistuiID: "1", Summary: "Error: connection failed"},
		{HistuiID: "2", Summary: "WARNING: disk space low"},
		{HistuiID: "3", Summary: "Info: backup complete"},
	}

	// Case-insensitive regex match
	expr, err := ParseFilter("summary~=(?i)error|warning")
	require.NoError(t, err)

	result := FilterWithExpr(notifications, expr)
	assert.Len(t, result, 2)
}

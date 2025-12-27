package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsFilterExpression(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected bool
	}{
		// Valid filter expressions
		{"app_equal", "app=discord", true},
		{"app_not_equal", "app!=slack", true},
		{"body_contains", "body~meeting", true},
		{"summary_regex", "summary~=(?i)error", true},
		{"urgency_greater", "urgency>normal", true},
		{"urgency_less", "urgency<critical", true},
		{"urgency_greater_eq", "urgency>=normal", true},
		{"urgency_less_eq", "urgency<=normal", true},
		{"dismissed", "dismissed=true", true},
		{"seen", "seen=false", true},
		{"timestamp", "timestamp<1h", true},
		{"category", "category=email", true},
		{"multiple", "app=slack,urgency=critical", true},

		// Not filter expressions (plain text search)
		{"plain_word", "meeting", false},
		{"plain_phrase", "important message", false},
		{"email_address", "user@example.com", false}, // @ is not a filter operator
		{"url", "https://example.com", false},
		{"unknown_field", "unknown=value", false},
		{"just_equals", "=value", false},
		{"number", "12345", false},
		{"empty", "", false},

		// Edge cases
		{"partial_field", "ap=discord", false},        // "ap" is not a valid field
		{"case_insensitive_field", "APP=discord", true}, // fields are case-insensitive
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isFilterExpression(tt.query)
			assert.Equal(t, tt.expected, result, "query: %q", tt.query)
		})
	}
}

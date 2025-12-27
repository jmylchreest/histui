// Package core provides filtering, sorting, and lookup logic.
package core

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jmylchreest/histui/internal/model"
)

// FilterOp represents a comparison operator.
type FilterOp string

const (
	FilterOpEqual     FilterOp = "="  // Exact match
	FilterOpNotEqual  FilterOp = "!=" // Not equal
	FilterOpContains  FilterOp = "~"  // Contains substring
	FilterOpRegex     FilterOp = "~=" // Regex match
	FilterOpGreater   FilterOp = ">"  // Greater than
	FilterOpLess      FilterOp = "<"  // Less than
	FilterOpGreaterEq FilterOp = ">=" // Greater than or equal
	FilterOpLessEq    FilterOp = "<=" // Less than or equal
)

// FilterCondition represents a single filter condition.
type FilterCondition struct {
	Field    string   // Field name: app, summary, body, urgency, timestamp, category, dismissed, seen
	Operator FilterOp // Comparison operator
	Value    string   // Value to compare against

	// Cached parsed values for efficiency
	regex       *regexp.Regexp // Compiled regex for ~= operator
	urgencyVal  int            // Parsed urgency value
	timestampOp time.Time      // Parsed timestamp for comparison
	boolVal     bool           // Parsed bool value
}

// FilterExpr represents a compound filter expression.
// Multiple conditions are ANDed together.
type FilterExpr struct {
	Conditions []FilterCondition
}

// FilterOptions specifies criteria for filtering notifications.
type FilterOptions struct {
	Since     time.Duration // Filter to notifications newer than now-since (0=all)
	AppFilter string        // Exact match on app name
	Urgency   *int          // Filter by urgency level (nil=any)
	Limit     int           // Maximum results (0=unlimited)
}

// Filter filters notifications based on the provided options.
func Filter(notifications []model.Notification, opts FilterOptions) []model.Notification {
	now := time.Now()
	result := make([]model.Notification, 0, len(notifications))

	for _, n := range notifications {
		// Time filter
		if opts.Since > 0 {
			cutoff := now.Add(-opts.Since)
			if time.Unix(n.Timestamp, 0).Before(cutoff) {
				continue
			}
		}

		// App filter
		if opts.AppFilter != "" && n.AppName != opts.AppFilter {
			continue
		}

		// Urgency filter
		if opts.Urgency != nil && n.Urgency != *opts.Urgency {
			continue
		}

		result = append(result, n)
	}

	// Apply limit
	if opts.Limit > 0 && len(result) > opts.Limit {
		result = result[:opts.Limit]
	}

	return result
}

// ParseDuration parses a duration string with extended formats.
// Supports: 48h, 7d, 1w, 0 (all time)
func ParseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)

	// Special case: 0 means no filter (all time)
	if s == "0" || s == "" {
		return 0, nil
	}

	// Handle day suffix (7d -> 168h)
	if daysStr, found := strings.CutSuffix(s, "d"); found {
		days, err := strconv.Atoi(daysStr)
		if err != nil {
			return 0, fmt.Errorf("invalid duration: %s", s)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}

	// Handle week suffix (1w -> 168h)
	if weeksStr, found := strings.CutSuffix(s, "w"); found {
		weeks, err := strconv.Atoi(weeksStr)
		if err != nil {
			return 0, fmt.Errorf("invalid duration: %s", s)
		}
		return time.Duration(weeks) * 7 * 24 * time.Hour, nil
	}

	// Standard Go duration parsing
	return time.ParseDuration(s)
}

// ParseUrgency parses an urgency string to its integer value.
// Accepts: low, normal, critical, 0, 1, 2
func ParseUrgency(s string) (int, error) {
	s = strings.ToLower(strings.TrimSpace(s))

	switch s {
	case "low", "0":
		return model.UrgencyLow, nil
	case "normal", "1":
		return model.UrgencyNormal, nil
	case "critical", "2":
		return model.UrgencyCritical, nil
	default:
		return 0, fmt.Errorf("invalid urgency: %s (use low, normal, or critical)", s)
	}
}

// ParseFilter parses a filter expression string into a FilterExpr.
// Format: "field=value,field2~value2,field3>value3"
// Multiple conditions are comma-separated and ANDed together.
//
// Supported fields: app, summary, body, urgency, category, dismissed, seen, timestamp
// Supported operators: = (equal), != (not equal), ~ (contains), ~= (regex), >, <, >=, <=
//
// Examples:
//   - "app=discord" - exact app name match
//   - "summary~error" - summary contains "error"
//   - "urgency>=normal" - urgency is normal or higher
//   - "app=slack,urgency=critical" - Slack critical notifications
//   - "body~=(?i)meeting" - body matches regex (case-insensitive "meeting")
//   - "timestamp>1h" - notifications from the last hour
func ParseFilter(expr string) (*FilterExpr, error) {
	if expr == "" {
		return &FilterExpr{}, nil
	}

	filter := &FilterExpr{
		Conditions: make([]FilterCondition, 0),
	}

	// Split by comma
	for part := range strings.SplitSeq(expr, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		cond, err := parseCondition(part)
		if err != nil {
			return nil, err
		}
		filter.Conditions = append(filter.Conditions, cond)
	}

	return filter, nil
}

// parseCondition parses a single condition like "app=discord" or "body~error"
func parseCondition(s string) (FilterCondition, error) {
	// Try operators in order of specificity (longest first)
	operators := []FilterOp{
		FilterOpNotEqual,  // != (must be before =)
		FilterOpGreaterEq, // >= (must be before >)
		FilterOpLessEq,    // <= (must be before <)
		FilterOpRegex,     // ~= (must be before ~)
		FilterOpEqual,
		FilterOpContains,
		FilterOpGreater,
		FilterOpLess,
	}

	for _, op := range operators {
		idx := strings.Index(s, string(op))
		if idx > 0 {
			field := strings.TrimSpace(s[:idx])
			value := strings.TrimSpace(s[idx+len(op):])

			cond := FilterCondition{
				Field:    strings.ToLower(field),
				Operator: op,
				Value:    value,
			}

			// Pre-parse and validate based on field type
			if err := cond.init(); err != nil {
				return FilterCondition{}, err
			}

			return cond, nil
		}
	}

	return FilterCondition{}, fmt.Errorf("invalid filter condition: %s (missing operator)", s)
}

// init pre-parses and validates the condition value.
func (c *FilterCondition) init() error {
	switch c.Field {
	case "app", "app_name", "appname":
		c.Field = "app" // Normalize
	case "summary", "title":
		c.Field = "summary"
	case "body", "message":
		c.Field = "body"
	case "category", "cat":
		c.Field = "category"
	case "urgency", "priority":
		c.Field = "urgency"
		// Parse urgency value
		u, err := ParseUrgency(c.Value)
		if err != nil {
			return err
		}
		c.urgencyVal = u
	case "dismissed", "dismiss":
		c.Field = "dismissed"
		c.boolVal = parseBool(c.Value)
	case "seen":
		c.Field = "seen"
		c.boolVal = parseBool(c.Value)
	case "timestamp", "time", "ts":
		c.Field = "timestamp"
		// Parse duration for relative time comparisons
		dur, err := ParseDuration(c.Value)
		if err != nil {
			return fmt.Errorf("invalid timestamp value: %w", err)
		}
		c.timestampOp = time.Now().Add(-dur)
	default:
		return fmt.Errorf("unknown filter field: %s", c.Field)
	}

	// Compile regex if needed
	if c.Operator == FilterOpRegex {
		re, err := regexp.Compile(c.Value)
		if err != nil {
			return fmt.Errorf("invalid regex: %w", err)
		}
		c.regex = re
	}

	return nil
}

// parseBool parses various boolean representations.
func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "true", "yes", "1", "y", "t":
		return true
	default:
		return false
	}
}

// Match tests if a notification matches the filter expression.
// All conditions must match (AND logic).
func (f *FilterExpr) Match(n model.Notification) bool {
	for _, cond := range f.Conditions {
		if !cond.Match(n) {
			return false
		}
	}
	return true
}

// Match tests if a notification matches this single condition.
func (c *FilterCondition) Match(n model.Notification) bool {
	switch c.Field {
	case "app":
		return c.matchString(n.AppName)
	case "summary":
		return c.matchString(n.Summary)
	case "body":
		return c.matchString(n.Body)
	case "category":
		return c.matchString(n.Category)
	case "urgency":
		return c.matchInt(n.Urgency, c.urgencyVal)
	case "dismissed":
		return c.matchBool(n.IsDismissed())
	case "seen":
		return c.matchBool(n.IsSeen())
	case "timestamp":
		return c.matchTimestamp(time.Unix(n.Timestamp, 0))
	default:
		return false
	}
}

// matchString matches a string field.
func (c *FilterCondition) matchString(fieldValue string) bool {
	switch c.Operator {
	case FilterOpEqual:
		return fieldValue == c.Value
	case FilterOpNotEqual:
		return fieldValue != c.Value
	case FilterOpContains:
		return strings.Contains(strings.ToLower(fieldValue), strings.ToLower(c.Value))
	case FilterOpRegex:
		return c.regex != nil && c.regex.MatchString(fieldValue)
	default:
		return false
	}
}

// matchInt matches an integer field with numeric comparison.
func (c *FilterCondition) matchInt(fieldValue, condValue int) bool {
	switch c.Operator {
	case FilterOpEqual:
		return fieldValue == condValue
	case FilterOpNotEqual:
		return fieldValue != condValue
	case FilterOpGreater:
		return fieldValue > condValue
	case FilterOpLess:
		return fieldValue < condValue
	case FilterOpGreaterEq:
		return fieldValue >= condValue
	case FilterOpLessEq:
		return fieldValue <= condValue
	default:
		return false
	}
}

// matchBool matches a boolean field.
func (c *FilterCondition) matchBool(fieldValue bool) bool {
	switch c.Operator {
	case FilterOpEqual:
		return fieldValue == c.boolVal
	case FilterOpNotEqual:
		return fieldValue != c.boolVal
	default:
		return false
	}
}

// matchTimestamp matches a timestamp field.
func (c *FilterCondition) matchTimestamp(fieldValue time.Time) bool {
	switch c.Operator {
	case FilterOpGreater:
		return fieldValue.After(c.timestampOp)
	case FilterOpLess:
		return fieldValue.Before(c.timestampOp)
	case FilterOpGreaterEq:
		return fieldValue.After(c.timestampOp) || fieldValue.Equal(c.timestampOp)
	case FilterOpLessEq:
		return fieldValue.Before(c.timestampOp) || fieldValue.Equal(c.timestampOp)
	default:
		return false
	}
}

// FilterWithExpr filters notifications using a filter expression.
func FilterWithExpr(notifications []model.Notification, expr *FilterExpr) []model.Notification {
	if expr == nil || len(expr.Conditions) == 0 {
		return notifications
	}

	result := make([]model.Notification, 0, len(notifications))
	for _, n := range notifications {
		if expr.Match(n) {
			result = append(result, n)
		}
	}
	return result
}

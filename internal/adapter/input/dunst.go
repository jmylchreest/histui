package input

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/jmylchreest/histui/internal/model"
	"github.com/oklog/ulid/v2"
)

// DunstAdapter fetches notifications from dunstctl history.
type DunstAdapter struct{}

// NewDunstAdapter creates a new DunstAdapter.
func NewDunstAdapter() *DunstAdapter {
	return &DunstAdapter{}
}

// Name returns the adapter identifier.
func (a *DunstAdapter) Name() string {
	return "dunst"
}

// Import fetches notifications from dunstctl history.
func (a *DunstAdapter) Import(ctx context.Context) ([]model.Notification, error) {
	// Execute dunstctl history
	cmd := exec.CommandContext(ctx, "dunstctl", "history")
	output, err := cmd.Output()
	if err != nil {
		return nil, &AdapterError{
			Source:  "dunst",
			Message: "failed to execute dunstctl history",
			Err:     err,
		}
	}

	return ParseDunstHistory(output)
}

// dunstHistory represents the top-level dunstctl history JSON structure.
type dunstHistory struct {
	Type string          `json:"type"`
	Data [][]dunstEntry  `json:"data"`
}

// dunstEntry represents a single notification in dunstctl history.
type dunstEntry struct {
	ID            dunstValue `json:"id"`
	AppName       dunstValue `json:"appname"`
	Summary       dunstValue `json:"summary"`
	Body          dunstValue `json:"body"`
	Timestamp     dunstValue `json:"timestamp"`
	Timeout       dunstValue `json:"timeout"`
	Urgency       dunstValue `json:"urgency"`
	Category      dunstValue `json:"category"`
	IconPath      dunstValue `json:"icon_path"`
	DefaultAction dunstValue `json:"default_action_name"`
	Progress      dunstValue `json:"progress"`
	Message       dunstValue `json:"message"`
	URLs          dunstValue `json:"urls"`
	Foreground    dunstValue `json:"fg"`
	Background    dunstValue `json:"bg"`
	StackTag      dunstValue `json:"stack_tag"`
}

// dunstValue represents a typed value in dunst JSON.
// dunst uses {"type": "INT", "data": 123} format.
type dunstValue struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// String returns the value as a string.
func (v dunstValue) String() string {
	switch d := v.Data.(type) {
	case string:
		return d
	case float64:
		return strconv.FormatFloat(d, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(d, 10)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", d)
	}
}

// Int returns the value as an int.
func (v dunstValue) Int() int {
	switch d := v.Data.(type) {
	case float64:
		return int(d)
	case int64:
		return int(d)
	case string:
		i, _ := strconv.Atoi(d)
		return i
	default:
		return 0
	}
}

// Int64 returns the value as an int64.
func (v dunstValue) Int64() int64 {
	switch d := v.Data.(type) {
	case float64:
		return int64(d)
	case int64:
		return d
	case string:
		i, _ := strconv.ParseInt(d, 10, 64)
		return i
	default:
		return 0
	}
}

// ParseDunstHistory parses dunstctl history JSON output.
func ParseDunstHistory(data []byte) ([]model.Notification, error) {
	var history dunstHistory
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, &AdapterError{
			Source:  "dunst",
			Message: "failed to parse dunstctl history JSON",
			Err:     err,
		}
	}

	var notifications []model.Notification

	// dunst uses nested arrays: data is [[entry1, entry2, ...]]
	for _, group := range history.Data {
		for _, entry := range group {
			n, err := convertDunstEntry(entry)
			if err != nil {
				// Log and skip malformed entries
				continue
			}
			notifications = append(notifications, *n)
		}
	}

	return notifications, nil
}

// convertDunstEntry converts a dunst entry to a Notification.
func convertDunstEntry(entry dunstEntry) (*model.Notification, error) {
	// Generate ULID
	id, err := ulid.New(ulid.Timestamp(time.Now()), rand.Reader)
	if err != nil {
		return nil, err
	}

	now := time.Now()

	// Convert dunst timestamp (microseconds since boot) to Unix timestamp
	timestamp := convertDunstTimestamp(entry.Timestamp.Int64())

	// Get urgency
	urgency := entry.Urgency.Int()
	if urgency < 0 || urgency > 2 {
		urgency = model.UrgencyNormal
	}

	n := &model.Notification{
		HistuiID:         id.String(),
		HistuiSource:     "dunst",
		HistuiImportedAt: now.Unix(),
		ID:               entry.ID.Int(),
		AppName:          sanitizeString(entry.AppName.String()),
		Summary:          sanitizeString(entry.Summary.String()),
		Body:             sanitizeString(entry.Body.String()),
		Timestamp:        timestamp,
		ExpireTimeout:    entry.Timeout.Int(),
		Urgency:          urgency,
		UrgencyName:      model.UrgencyNames[urgency],
		Category:         entry.Category.String(),
		IconPath:         entry.IconPath.String(),
	}

	// Add extensions if any non-empty values
	ext := &model.Extensions{
		StackTag:   entry.StackTag.String(),
		Progress:   entry.Progress.Int(),
		Message:    entry.Message.String(),
		URLs:       entry.URLs.String(),
		Foreground: entry.Foreground.String(),
		Background: entry.Background.String(),
	}

	if ext.StackTag != "" || ext.Progress != 0 || ext.Message != "" ||
		ext.URLs != "" || ext.Foreground != "" || ext.Background != "" {
		n.Extensions = ext
	}

	return n, nil
}

// convertDunstTimestamp converts dunst timestamp to Unix timestamp.
// Dunst timestamps are microseconds since boot, so we need to convert them.
func convertDunstTimestamp(dunstTimestamp int64) int64 {
	if dunstTimestamp == 0 {
		return time.Now().Unix()
	}

	// Read system uptime
	uptimeData, err := os.ReadFile("/proc/uptime")
	if err != nil {
		// Fallback: assume timestamp is already Unix time if it looks like it
		if dunstTimestamp > 1000000000 {
			return dunstTimestamp
		}
		return time.Now().Unix()
	}

	// Parse uptime (first number in seconds with decimal)
	uptimeStr := strings.Fields(string(uptimeData))[0]
	uptimeFloat, err := strconv.ParseFloat(uptimeStr, 64)
	if err != nil {
		return time.Now().Unix()
	}

	// Convert uptime to microseconds
	uptimeMicros := int64(uptimeFloat * 1000000)

	// Current time in microseconds since epoch
	nowMicros := time.Now().UnixMicro()

	// Boot time in microseconds since epoch
	bootTimeMicros := nowMicros - uptimeMicros

	// Notification time in microseconds since epoch
	notifMicros := bootTimeMicros + dunstTimestamp

	// Convert to seconds
	return notifMicros / 1000000
}

// sanitizeString removes control characters and normalizes whitespace.
func sanitizeString(s string) string {
	// Replace control characters with spaces
	var result strings.Builder
	for _, r := range s {
		if r < 32 && r != '\n' && r != '\t' {
			result.WriteRune(' ')
		} else {
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(result.String())
}

// DunstCounts holds notification counts from dunst.
type DunstCounts struct {
	Displayed int // Currently visible on screen
	History   int // Dismissed, in history
	Waiting   int // Queued, waiting to be displayed
}

// GetCounts returns notification counts from dunstctl.
func (a *DunstAdapter) GetCounts(ctx context.Context) (*DunstCounts, error) {
	counts := &DunstCounts{}

	// Get displayed count
	if n, err := getDunstCount(ctx, "displayed"); err == nil {
		counts.Displayed = n
	}

	// Get history count
	if n, err := getDunstCount(ctx, "history"); err == nil {
		counts.History = n
	}

	// Get waiting count
	if n, err := getDunstCount(ctx, "waiting"); err == nil {
		counts.Waiting = n
	}

	return counts, nil
}

// getDunstCount executes dunstctl count <type> and returns the count.
func getDunstCount(ctx context.Context, countType string) (int, error) {
	cmd := exec.CommandContext(ctx, "dunstctl", "count", countType)
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	count, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return 0, err
	}

	return count, nil
}

// GetActiveCount returns the count of active (displayed + waiting) notifications.
func (a *DunstAdapter) GetActiveCount(ctx context.Context) (int, error) {
	counts, err := a.GetCounts(ctx)
	if err != nil {
		return 0, err
	}
	return counts.Displayed + counts.Waiting, nil
}

package input

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"os"
	"time"

	"github.com/jmylchreest/histui/internal/model"
	"github.com/oklog/ulid/v2"
)

// StdinAdapter reads notifications from standard input.
type StdinAdapter struct {
	reader io.Reader
}

// NewStdinAdapter creates a new StdinAdapter reading from os.Stdin.
func NewStdinAdapter() *StdinAdapter {
	return &StdinAdapter{reader: os.Stdin}
}

// NewStdinAdapterWithReader creates a new StdinAdapter with a custom reader.
func NewStdinAdapterWithReader(r io.Reader) *StdinAdapter {
	return &StdinAdapter{reader: r}
}

// Name returns the adapter identifier.
func (a *StdinAdapter) Name() string {
	return "stdin"
}

// Import reads notifications from standard input.
// Supports two formats:
// 1. JSON array of notifications
// 2. dunstctl history format
func (a *StdinAdapter) Import(ctx context.Context) ([]model.Notification, error) {
	// Read all input
	scanner := bufio.NewScanner(a.reader)
	const maxSize = 10 * 1024 * 1024 // 10MB max
	scanner.Buffer(make([]byte, 64*1024), maxSize)

	var data []byte
	for scanner.Scan() {
		data = append(data, scanner.Bytes()...)
		data = append(data, '\n')
	}

	if err := scanner.Err(); err != nil {
		return nil, &AdapterError{
			Source:  "stdin",
			Message: "failed to read stdin",
			Err:     err,
		}
	}

	if len(data) == 0 {
		return nil, nil
	}

	// Try to parse as dunst history format first
	notifications, err := ParseDunstHistory(data)
	if err == nil && len(notifications) > 0 {
		return notifications, nil
	}

	// Try to parse as JSON array
	return parseJSONArray(data)
}

// parseJSONArray parses a JSON array of notifications.
func parseJSONArray(data []byte) ([]model.Notification, error) {
	// Try array format
	var entries []stdinEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, &AdapterError{
			Source:  "stdin",
			Message: "failed to parse JSON input",
			Err:     err,
		}
	}

	var notifications []model.Notification
	for _, entry := range entries {
		n, err := convertStdinEntry(entry)
		if err != nil {
			continue
		}
		notifications = append(notifications, *n)
	}

	return notifications, nil
}

// stdinEntry represents a notification in the simple JSON format.
type stdinEntry struct {
	AppName   string `json:"app_name"`
	Summary   string `json:"summary"`
	Body      string `json:"body"`
	Timestamp int64  `json:"timestamp"`
	Urgency   int    `json:"urgency"`
	Category  string `json:"category,omitempty"`
	IconPath  string `json:"icon_path,omitempty"`
}

// convertStdinEntry converts a stdin entry to a Notification.
func convertStdinEntry(entry stdinEntry) (*model.Notification, error) {
	id, err := ulid.New(ulid.Timestamp(time.Now()), rand.Reader)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	timestamp := entry.Timestamp
	if timestamp == 0 {
		timestamp = now.Unix()
	}

	urgency := entry.Urgency
	if urgency < 0 || urgency > 2 {
		urgency = model.UrgencyNormal
	}

	return &model.Notification{
		HistuiID:         id.String(),
		HistuiSource:     "stdin",
		HistuiImportedAt: now.Unix(),
		AppName:          sanitizeString(entry.AppName),
		Summary:          sanitizeString(entry.Summary),
		Body:             sanitizeString(entry.Body),
		Timestamp:        timestamp,
		Urgency:          urgency,
		UrgencyName:      model.UrgencyNames[urgency],
		Category:         entry.Category,
		IconPath:         entry.IconPath,
	}, nil
}
